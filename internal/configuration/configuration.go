// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package configuration implements configuration service: storing image configurations.
package configuration

import (
	"context"

	"go.uber.org/zap"

	"github.com/siderolabs/image-service/internal/configuration/storage"
	"github.com/siderolabs/image-service/pkg/configuration"
)

// Service is the configuration service.
type Service struct {
	logger  *zap.Logger
	storage storage.Storage
	options Options
}

// Options for the configuration service.
type Options struct {
	// A key used to derive the configuration ID.
	//
	// This key should not change.
	Key []byte
}

// NewService creates a new configuration service.
func NewService(logger *zap.Logger, storage storage.Storage, options Options) *Service {
	return &Service{
		options: options,
		storage: storage,
		logger:  logger.With(zap.String("service", "configuration")),
	}
}

// Put stores the configuration.
//
// If the configuration already exists, Put does nothing.
func (s *Service) Put(ctx context.Context, cfg *configuration.Configuration) (string, error) {
	id, err := cfg.ID(s.options.Key)
	if err != nil {
		return "", err
	}

	if err = s.storage.Head(ctx, id); err == nil {
		s.logger.Info("configuration already exists", zap.String("id", id))

		return id, nil
	}

	data, err := cfg.Marshal()
	if err != nil {
		return "", err
	}

	err = s.storage.Put(ctx, id, data)

	if err == nil {
		s.logger.Info("configuration created", zap.String("id", id), zap.Any("customization", cfg.Customization))
	}

	return id, err
}

// Get retrieves the stored configuration.
func (s *Service) Get(ctx context.Context, id string) (*configuration.Configuration, error) {
	data, err := s.storage.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	return configuration.Unmarshal(data)
}
