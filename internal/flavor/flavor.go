// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package flavor implements flavor service: storing image flavors.
package flavor

import (
	"context"

	"go.uber.org/zap"

	"github.com/siderolabs/image-service/internal/flavor/storage"
	"github.com/siderolabs/image-service/pkg/flavor"
)

// Service is the flavor service.
type Service struct {
	options Options
	logger  *zap.Logger
	storage storage.Storage
}

// Options for the flavor service.
type Options struct{}

// NewService creates a new flavor service.
func NewService(logger *zap.Logger, storage storage.Storage, options Options) *Service {
	return &Service{
		options: options,
		storage: storage,
		logger:  logger.With(zap.String("service", "flavor")),
	}
}

// Put stores the flavor.
//
// If the flavor already exists, Put does nothing.
func (s *Service) Put(ctx context.Context, cfg *flavor.Flavor) (string, error) {
	id, err := cfg.ID()
	if err != nil {
		return "", err
	}

	if err = s.storage.Head(ctx, id); err == nil {
		s.logger.Info("flavor already exists", zap.String("id", id))

		return id, nil
	}

	data, err := cfg.Marshal()
	if err != nil {
		return "", err
	}

	err = s.storage.Put(ctx, id, data)

	if err == nil {
		s.logger.Info("flavor created", zap.String("id", id), zap.Any("customization", cfg.Customization))
	}

	return id, err
}

// Get retrieves the stored flavor.
func (s *Service) Get(ctx context.Context, id string) (*flavor.Flavor, error) {
	data, err := s.storage.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	return flavor.Unmarshal(data)
}
