// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"context"
	"encoding/base64"
	"flag"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
	"golang.org/x/sync/errgroup"

	"github.com/siderolabs/image-service/cmd/image-service/cmd"
)

func setupService(t *testing.T) (context.Context, string) {
	t.Helper()

	ctx, cancel := context.WithCancel(context.Background())

	logger := zaptest.NewLogger(t)

	options := cmd.DefaultOptions
	options.ConfigKeyBase64 = base64.StdEncoding.EncodeToString([]byte("secret")) // use a fixed key for reproducibility
	options.HTTPListenAddr = findListenAddr(t)
	options.ImagePrefix = imagePrefixFlag
	options.ExternalURL = "http://" + options.HTTPListenAddr + "/"

	eg, ctx := errgroup.WithContext(ctx)

	eg.Go(func() error {
		return cmd.RunService(ctx, logger, options)
	})

	t.Cleanup(func() {
		require.NoError(t, eg.Wait())
	})
	t.Cleanup(cancel)
	t.Cleanup(http.DefaultClient.CloseIdleConnections)

	// wait for the endpoint to be ready
	require.Eventually(t, func() bool {
		d, err := net.Dial("tcp", options.HTTPListenAddr)
		if d != nil {
			require.NoError(t, d.Close())
		}

		return err == nil
	}, 10*time.Second, 10*time.Millisecond)

	return ctx, "http://" + options.HTTPListenAddr
}

func findListenAddr(t *testing.T) string {
	t.Helper()

	l, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)

	addr := l.Addr().String()

	require.NoError(t, l.Close())

	return addr
}

func TestIntegration(t *testing.T) {
	ctx, baseURL := setupService(t)

	t.Run("TestConfiguration", func(t *testing.T) {
		// configuration should be created first, thus no t.Parallel
		testConfiguration(ctx, t, baseURL)
	})

	t.Run("TestDownloadFrontend", func(t *testing.T) {
		t.Parallel()

		testDownloadFrontend(ctx, t, baseURL)
	})

	t.Run("TestPXEFrontend", func(t *testing.T) {
		t.Parallel()

		testPXEFrontend(ctx, t, baseURL)
	})
}

var imagePrefixFlag string

func init() {
	flag.StringVar(&imagePrefixFlag, "test.image-prefix", cmd.DefaultOptions.ImagePrefix, "image prefix")
}
