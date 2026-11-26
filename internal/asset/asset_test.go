// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package asset_test

import (
	"context"
	"errors"
	"testing"

	talosprofile "github.com/siderolabs/talos/pkg/imager/profile"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/image-factory/internal/asset"
	"github.com/siderolabs/image-factory/internal/asset/cache"
)

// cancelingCache cancels the request during Get and fails the lookup with an error that is not
// a cache miss, the way a canceled fetch of the entry's blob surfaces. Put records that a build
// ran to completion.
type cancelingCache struct {
	cancel context.CancelFunc
	puts   int
}

func (c *cancelingCache) Get(context.Context, string) (cache.BootAsset, error) {
	c.cancel()

	return nil, errors.New(`error creating object reference for profile: Get "https://cdn.example/blob": context canceled`)
}

func (c *cancelingCache) Put(context.Context, string, cache.BootAsset, string) error {
	c.puts++

	return nil
}

// TestBuildDoesNotRebuildForCanceledRequest asserts that a request canceled during the cache
// lookup does not start a build. The build runs detached, so it would complete for a caller that
// is gone and replace a valid cache entry, missing every concurrent request while it does.
func TestBuildDoesNotRebuildForCanceledRequest(t *testing.T) {
	t.Parallel()

	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()

	c := &cancelingCache{cancel: cancel}

	// no artifacts manager: reaching the build at all would panic
	builder, err := asset.NewBuilder(zaptest.NewLogger(t), nil, c, asset.Options{
		AllowedConcurrency: 1,
		GetAfterPut:        true,
	})
	require.NoError(t, err)

	prof := talosprofile.Profile{
		Arch:     "arm64",
		Platform: "metal",
		Output:   talosprofile.Output{Kind: talosprofile.OutKindISO},
	}

	_, err = builder.Build(ctx, prof, "v1.14.0", "metal-arm64.iso", "")
	require.ErrorIs(t, err, context.Canceled)
	require.Zero(t, c.puts)
}
