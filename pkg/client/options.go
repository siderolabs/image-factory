// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package client

import "net/http"

// Options defines client options.
type Options struct {
	// Client is the http client.
	Client http.Client
}

// Option defines a single client option setter.
type Option func(*Options)

// WithClient overrides default client instance.
func WithClient(client http.Client) Option {
	return func(o *Options) {
		o.Client = client
	}
}

func withDefaults(options []Option) *Options {
	opts := &Options{}

	for _, o := range options {
		o(opts)
	}

	return opts
}
