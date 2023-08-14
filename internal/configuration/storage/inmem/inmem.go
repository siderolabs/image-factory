// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package inmem implements a configuration storage in memory (for testing).
package inmem

import (
	"context"
	"sync"

	"github.com/siderolabs/image-service/internal/configuration/storage"
)

// Storage is a config storage.
type Storage struct {
	data map[string][]byte
	mu   sync.Mutex
}

// Check interface.
var _ storage.Storage = (*Storage)(nil)

// Head checks if the configuration exists.
func (s *Storage) Head(_ context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.data[id]; ok {
		return nil
	}

	return storage.ErrNotFound
}

// Get returns the configuration.
func (s *Storage) Get(_ context.Context, id string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if data, ok := s.data[id]; ok {
		return data, nil
	}

	return nil, storage.ErrNotFound
}

// Put stores the configuration.
func (s *Storage) Put(_ context.Context, id string, data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.data == nil {
		s.data = map[string][]byte{}
	}

	s.data[id] = data

	return nil
}
