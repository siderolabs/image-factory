// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package schematic implements schematic factory: storing image schematics.
package schematic

import (
	"context"

	"github.com/prometheus/client_golang/prometheus"
	"go.uber.org/zap"

	"github.com/skyssolutions/siderolabs-image-factory/internal/schematic/storage"
	"github.com/skyssolutions/siderolabs-image-factory/pkg/schematic"
)

// Factory is the schematic factory.
type Factory struct {
	options Options
	logger  *zap.Logger
	storage storage.Storage

	metricGet, metricCreate, metricDuplicate prometheus.Counter
}

// Options for the schematic factory.
type Options struct{}

// NewFactory creates a new schematic factory.
func NewFactory(logger *zap.Logger, storage storage.Storage, options Options) *Factory {
	return &Factory{
		options: options,
		storage: storage,
		logger:  logger.With(zap.String("factory", "schematic")),

		metricGet: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "image_factory_schematic_get_total",
			Help: "Number of times schematics were retrieved.",
		}),
		metricCreate: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "image_factory_schematic_create_total",
			Help: "Number of new schematics created.",
		}),
		metricDuplicate: prometheus.NewCounter(prometheus.CounterOpts{
			Name: "image_factory_schematic_duplicate_create_total",
			Help: "Number of new schematics which were created as duplicate.",
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

	if err = s.storage.Head(ctx, id); err == nil {
		s.logger.Info("schematic already exists", zap.String("id", id))

		s.metricDuplicate.Inc()

		return id, nil
	}

	data, err := cfg.Marshal()
	if err != nil {
		return "", err
	}

	err = s.storage.Put(ctx, id, data)

	if err == nil {
		s.metricCreate.Inc()

		s.logger.Info("schematic created", zap.String("id", id), zap.Any("customization", cfg.Customization))
	}

	return id, err
}

// Get retrieves the stored schematic.
func (s *Factory) Get(ctx context.Context, id string) (*schematic.Schematic, error) {
	data, err := s.storage.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	s.metricGet.Inc()

	return schematic.Unmarshal(data)
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
