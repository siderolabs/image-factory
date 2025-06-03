// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package refresher implements a simple refresh on interval mechanism.
package refresher

import (
	"sync"
	"time"
)

// Refresher is a generic type that holds an instance of type T and refreshes it
// at a specified interval using the provided refresh function.
//
// It acts a bit like sync.Once, but allows for periodic refreshes of the instance.
type Refresher[T any] struct { //nolint:govet
	mu       sync.Mutex
	instance T
	err      error

	refreshFunc     func() (T, error)
	lastRefresh     time.Time
	refreshInterval time.Duration
}

// New creates a new Refresher instance with the given refresh function and interval.
func New[T any](refreshFunc func() (T, error), refreshInterval time.Duration) *Refresher[T] {
	if refreshInterval <= 0 {
		panic("refresh interval must be greater than zero")
	}

	return &Refresher[T]{
		refreshFunc:     refreshFunc,
		refreshInterval: refreshInterval,
	}
}

// Get returns the current instance of type T, refreshing it if the
// refresh interval has passed since the last refresh.
//
// If the refresh function returns an error, it will be stored and returned
// on subsequent calls until the next successful refresh.
func (r *Refresher[T]) Get() (T, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	if time.Since(r.lastRefresh) < r.refreshInterval {
		return r.instance, r.err
	}

	r.instance, r.err = r.refreshFunc()
	r.lastRefresh = time.Now()

	return r.instance, r.err
}
