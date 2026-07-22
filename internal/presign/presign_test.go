// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package presign_test

import (
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/internal/presign"
)

func TestRoundTrip(t *testing.T) {
	t.Parallel()

	s := presign.NewSigner(make([]byte, 32), 5*time.Minute)
	path := "/image/abc123/v1.9.0/metal-amd64.iso"

	expires, sig := s.Sign(path)

	r := &http.Request{URL: &url.URL{
		Path:     path,
		RawQuery: "expires=" + expires + "&signature=" + sig,
	}}

	require.NoError(t, s.Verify(r))
}

func TestExpiredURL(t *testing.T) {
	t.Parallel()

	// TTL of 0 means the URL expires immediately.
	s := presign.NewSigner(make([]byte, 32), 0)
	path := "/image/abc123/v1.9.0/metal-amd64.iso"

	expires, sig := s.Sign(path)

	// Sleep briefly to ensure expiry.
	time.Sleep(time.Second)

	r := &http.Request{URL: &url.URL{
		Path:     path,
		RawQuery: "expires=" + expires + "&signature=" + sig,
	}}

	err := s.Verify(r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "expired")
}

func TestTamperedSignature(t *testing.T) {
	t.Parallel()

	s := presign.NewSigner(make([]byte, 32), 5*time.Minute)
	path := "/image/abc123/v1.9.0/metal-amd64.iso"

	expires, _ := s.Sign(path)

	r := &http.Request{URL: &url.URL{
		Path:     path,
		RawQuery: "expires=" + expires + "&signature=deadbeef",
	}}

	err := s.Verify(r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid signature")
}

func TestTamperedPath(t *testing.T) {
	t.Parallel()

	s := presign.NewSigner(make([]byte, 32), 5*time.Minute)
	origPath := "/image/abc123/v1.9.0/metal-amd64.iso"

	expires, sig := s.Sign(origPath)

	r := &http.Request{URL: &url.URL{
		Path:     "/image/EVIL/v1.9.0/metal-amd64.iso",
		RawQuery: "expires=" + expires + "&signature=" + sig,
	}}

	err := s.Verify(r)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid signature")
}

func TestMissingParams(t *testing.T) {
	t.Parallel()

	s := presign.NewSigner(make([]byte, 32), 5*time.Minute)

	t.Run("MissingSignature", func(t *testing.T) {
		t.Parallel()

		r := &http.Request{URL: &url.URL{
			Path:     "/image/abc/v1/foo",
			RawQuery: "expires=9999999999",
		}}

		err := s.Verify(r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing signature")
	})

	t.Run("MissingExpires", func(t *testing.T) {
		t.Parallel()

		r := &http.Request{URL: &url.URL{
			Path:     "/image/abc/v1/foo",
			RawQuery: "signature=deadbeef",
		}}

		err := s.Verify(r)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "missing expires")
	})
}

func TestDifferentKeys(t *testing.T) {
	t.Parallel()

	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	key2[0] = 1

	s1 := presign.NewSigner(key1, 5*time.Minute)
	s2 := presign.NewSigner(key2, 5*time.Minute)

	path := "/image/abc123/v1.9.0/metal-amd64.iso"

	expires, sig := s1.Sign(path)

	r := &http.Request{URL: &url.URL{
		Path:     path,
		RawQuery: "expires=" + expires + "&signature=" + sig,
	}}

	require.Error(t, s2.Verify(r))
}

func TestGenerateSigner(t *testing.T) {
	t.Parallel()

	s, err := presign.GenerateSigner(5 * time.Minute)
	require.NoError(t, err)

	path := "/image/abc/v1/foo"

	expires, sig := s.Sign(path)

	r := &http.Request{URL: &url.URL{
		Path:     path,
		RawQuery: "expires=" + expires + "&signature=" + sig,
	}}

	require.NoError(t, s.Verify(r))
}
