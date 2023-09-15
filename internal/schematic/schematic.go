// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package schematic implements schematic factory: storing image schematics.
package schematic

import (
	"context"

	"go.uber.org/zap"

	"github.com/siderolabs/image-factory/internal/schematic/storage"
	"github.com/siderolabs/image-factory/pkg/schematic"
)

// Factory is the schematic factory.
type Factory struct {
	options Options
	logger  *zap.Logger
	storage storage.Storage
}

// Options for the schematic factory.
type Options struct{}

// NewFactory creates a new schematic factory.
func NewFactory(logger *zap.Logger, storage storage.Storage, options Options) *Factory {
	return &Factory{
		options: options,
		storage: storage,
		logger:  logger.With(zap.String("factory", "schematic")),
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

		return id, nil
	}

	data, err := cfg.Marshal()
	if err != nil {
		return "", err
	}

	err = s.storage.Put(ctx, id, data)

	if err == nil {
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

	return schematic.Unmarshal(data)
}
