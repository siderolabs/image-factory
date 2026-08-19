// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package schematic implements schematic factory: storing image schematics.
package schematic

import (
	"context"
	"errors"
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/siderolabs/gen/xerrors"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"

	"github.com/siderolabs/image-factory/internal/ctxlog"
	"github.com/siderolabs/image-factory/internal/schematic/storage"
	"github.com/siderolabs/image-factory/pkg/schematic"
)

// OwnershipChecker resolves the authenticated user from a context.
type OwnershipChecker interface {
	UsernameFromContext(ctx context.Context) (string, bool)
}

// Factory is the schematic factory.
type Factory struct {
	storage storage.Storage
	sf      singleflight.Group

	metricGet       prometheus.Counter
	metricCreate    prometheus.Counter
	metricDuplicate prometheus.Counter
	logger          *zap.Logger
	options         Options
}

// Options for the schematic factory.
type Options struct {
	MetricsNamespace string
}

// NewFactory creates a new schematic factory.
func NewFactory(logger *zap.Logger, storage storage.Storage, options Options) *Factory {
	return &Factory{
		options: options,
		storage: storage,
		logger:  logger.With(zap.String("factory", "schematic")),

		metricGet: prometheus.NewCounter(prometheus.CounterOpts{
			Name:      "image_factory_schematic_get_total",
			Help:      "Number of times schematics were retrieved.",
			Namespace: options.MetricsNamespace,
		}),
		metricCreate: prometheus.NewCounter(prometheus.CounterOpts{
			Name:      "image_factory_schematic_create_total",
			Help:      "Number of new schematics created.",
			Namespace: options.MetricsNamespace,
		}),
		metricDuplicate: prometheus.NewCounter(prometheus.CounterOpts{
			Name:      "image_factory_schematic_duplicate_create_total",
			Help:      "Number of new schematics which were created as duplicate.",
			Namespace: options.MetricsNamespace,
		}),
	}
}

// Put stores the schematic.
//
// If the schematic already exists, Put does nothing.
func (s *Factory) Put(ctx context.Context, cfg *schematic.Schematic) (string, error) {
	id, err := cfg.ID()
	if err != nil {
		return "", err
	}

	return id, s.put(ctx, id, cfg)
}

func (s *Factory) put(ctx context.Context, id string, cfg *schematic.Schematic) error {
	// carry the request ID into the detached call so its logs keep the request_id.
	reqID := ctxlog.RequestID(ctx)

	// put always returns the schematic
	put := func() (*schematic.Schematic, error) { //nolint:contextcheck
		dctx := ctxlog.WithRequestID(context.Background(), reqID)

		logger := ctxlog.Logger(dctx, s.logger)

		if err := s.storage.Head(dctx, id); err == nil {
			logger.Info("schematic already exists", zap.String("id", id))

			s.metricDuplicate.Inc()

			return cfg, nil
		}

		data, err := cfg.Marshal()
		if err != nil {
			return cfg, err
		}

		if err = s.storage.Put(dctx, id, data); err != nil {
			return cfg, err
		}

		s.metricCreate.Inc()

		logger.Info("schematic created", zap.String("id", id), zap.Any("customization", cfg.Customization))

		return cfg, nil
	}

	for {
		result, err, _ := s.sf.Do(id, func() (any, error) { //nolint:contextcheck
			return put()
		})

		if result == nil && err != nil {
			if errors.Is(err, context.Canceled) {
				return err
			}

			// This Put joined a failed Get. Retry so that the write is still
			// performed after the failed read flight completes.
			continue
		}

		if _, ok := result.(*schematic.Schematic); !ok {
			return fmt.Errorf("unexpected singleflight result type: %T", result)
		}

		return err
	}
}

// Get retrieves the stored schematic.
func (s *Factory) get(ctx context.Context, id string) (*schematic.Schematic, error) {
	// carry the request ID into the detached call so its logs keep the request_id.
	reqID := ctxlog.RequestID(ctx)

	result, err, _ := s.sf.Do(id, func() (any, error) { //nolint:contextcheck
		dctx := ctxlog.WithRequestID(context.Background(), reqID)

		data, err := s.storage.Get(dctx, id)
		if err != nil {
			return nil, err
		}

		s.metricGet.Inc()

		return schematic.Unmarshal(data)
	})
	if err != nil {
		return nil, err
	}

	schematic, ok := result.(*schematic.Schematic)
	if !ok {
		return nil, fmt.Errorf("unexpected singleflight result type: %T", result)
	}

	return schematic, nil
}

// Get retrieves the stored schematic and enforces ownership.
//
// If auth is non-nil and the caller is unauthenticated, RequiresAuthenticationTag is returned
// even when the schematic is not found, to avoid leaking schematic existence to anonymous callers.
func (s *Factory) Get(ctx context.Context, id string, auth OwnershipChecker) (*schematic.Schematic, error) {
	sc, err := s.get(ctx, id)
	if err != nil {
		if auth != nil {
			if _, ok := auth.UsernameFromContext(ctx); !ok {
				return nil, xerrors.NewTagged[schematic.RequiresAuthenticationTag](err)
			}
		}

		return nil, err
	}

	if sc.Owner == "" && auth == nil {
		return sc, nil
	}

	if auth == nil {
		return nil, xerrors.NewTagged[schematic.RequiresAuthenticationTag](errors.New("authentication required"))
	}

	username, ok := auth.UsernameFromContext(ctx)
	if !ok {
		return nil, xerrors.NewTagged[schematic.RequiresAuthenticationTag](errors.New("authentication required"))
	}

	if username != sc.Owner {
		return nil, xerrors.NewTagged[schematic.ForbiddenTag](errors.New("access denied"))
	}

	return sc, nil
}

// Describe implements prom.Collector interface.
func (s *Factory) Describe(ch chan<- *prometheus.Desc) {
	prometheus.DescribeByCollect(s, ch)
}

// Collect implements prom.Collector interface.
func (s *Factory) Collect(ch chan<- prometheus.Metric) {
	s.metricCreate.Collect(ch)
	s.metricGet.Collect(ch)
	s.metricDuplicate.Collect(ch)

	s.storage.Collect(ch)
}

var _ prometheus.Collector = &Factory{}
