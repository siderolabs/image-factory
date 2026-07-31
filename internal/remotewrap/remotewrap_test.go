// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package remotewrap_test

import (
	"testing"
	"time"

	"github.com/google/go-containerregistry/pkg/name"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/remotewrap"
)

func TestRegistryNameOptionsAreClientSpecific(t *testing.T) {
	remoteOptions := []remote.Option{}

	insecurePusher, err := remotewrap.NewPusher(time.Minute, []name.Option{name.Insecure}, remoteOptions)
	require.NoError(t, err)
	insecurePuller, err := remotewrap.NewPuller(time.Minute, []name.Option{name.Insecure}, remoteOptions)
	require.NoError(t, err)

	securePusher, err := remotewrap.NewPusher(time.Minute, nil, nil)
	require.NoError(t, err)
	securePuller, err := remotewrap.NewPuller(time.Minute, nil, nil)
	require.NoError(t, err)

	assertScheme := func(t *testing.T, options []name.Option, expected string) {
		t.Helper()

		repository, repositoryErr := name.NewRepository("registry.local:5000/cache", options...)
		require.NoError(t, repositoryErr)
		require.Equal(t, expected, repository.Scheme())
	}

	for _, client := range []interface{ NameOptions() []name.Option }{insecurePusher, insecurePuller} {
		assertScheme(t, client.NameOptions(), "http")
	}

	for _, client := range []interface{ NameOptions() []name.Option }{securePusher, securePuller} {
		assertScheme(t, client.NameOptions(), "https")
	}

	returnedOptions := insecurePusher.NameOptions()
	returnedOptions[0] = name.StrictValidation

	assertScheme(t, insecurePusher.NameOptions(), "http")
}
