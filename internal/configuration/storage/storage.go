// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package storage implements a storage for configuration data.
package storage

import (
	"context"
	"errors"
)

// Storage is the configuration storage.
type Storage interface {
	Head(ctx context.Context, id string) error
	Get(ctx context.Context, id string) ([]byte, error)
	Put(ctx context.Context, id string, data []byte) error
}

// ErrNotFound is returned when the configuration is not found.
var ErrNotFound = errors.New("configuration not found")
