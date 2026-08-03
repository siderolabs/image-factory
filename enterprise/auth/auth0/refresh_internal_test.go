// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

// Refresh deduplication is tested in-package because it needs the unexported
// sessionToken, doRefresh and session cookie helpers.

package auth0

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/image-factory/internal/testoidc"
)

// refreshProvider builds a browser-login provider whose token endpoint counts the
// exchanges it serves. A non-nil gate holds each exchange open until it is closed,
// which is how the concurrency test keeps the singleflight leader in flight.
func refreshProvider(t *testing.T, gate <-chan struct{}) (*Provider, *atomic.Int64) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	var exchanges atomic.Int64

	issuerURL := testoidc.StartServerWithRoutes(t, privateKey, "test-key", map[string]http.HandlerFunc{
		"/oauth/token": func(w http.ResponseWriter, _ *http.Request) {
			exchanges.Add(1)

			if gate != nil {
				<-gate
			}

			// A rotating tenant returns a fresh refresh token, invalidating the old one.
			json.NewEncoder(w).Encode(map[string]any{ //nolint:errcheck
				"access_token":  "access-token",
				"refresh_token": "rotated-refresh-token",
				"expires_in":    3600,
				"token_type":    "Bearer",
			})
		},
	})

	sessionKey := make([]byte, 32)
	_, err = io.ReadFull(rand.Reader, sessionKey)
	require.NoError(t, err)

	// IssuerURLOverride points the token endpoint at the test server above.
	p, err := NewProvider(t.Context(), zaptest.NewLogger(t), Config{
		Domain:            "tenant.example.com",
		Audience:          "https://factory.example.com",
		ClientID:          "client-id",
		ClientSecret:      "client-secret",
		RedirectURL:       "https://factory.example.com/callback",
		ExternalURL:       "https://factory.example.com",
		SessionKey:        sessionKey,
		IssuerURLOverride: issuerURL,
	})
	require.NoError(t, err)
	require.True(t, p.BrowserLoginEnabled())

	return p, &exchanges
}

// TestConcurrentRefreshExchangesOnce is the regression test for parallel htmx
// requests all refreshing the same near-expiry session. With refresh token rotation
// on, a second exchange would be a replay of an already-rotated token, and Auth0
// answers a replay by revoking the entire grant family.
func TestConcurrentRefreshExchangesOnce(t *testing.T) {
	t.Parallel()

	// The exchange blocks until every caller is inside sessionToken. Without that the
	// leader can finish and leave the singleflight before the others arrive, and they
	// then start exchanges of their own — which is the very thing under test.
	gate := make(chan struct{})

	p, exchanges := refreshProvider(t, gate)

	// Expired, so every caller takes the refresh path rather than the best-effort one.
	expired := sessionPayload{
		AccessToken:  "old-access-token",
		RefreshToken: "shared-refresh-token",
		Expiry:       time.Now().Add(-time.Minute),
	}

	rec := httptest.NewRecorder()
	require.NoError(t, setSessionCookie(rec, expired, p.sessionCipher, false))

	cookie := rec.Result().Cookies()[0] //nolint:bodyclose // no body on a recorder result

	const concurrency = 16

	var (
		start   = make(chan struct{})
		arrived sync.WaitGroup
		done    sync.WaitGroup
	)

	arrived.Add(concurrency)
	done.Add(concurrency)

	tokens := make([]string, concurrency)
	errs := make([]error, concurrency)

	for i := range concurrency {
		go func() {
			defer done.Done()

			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			r.AddCookie(cookie)

			<-start
			arrived.Done()

			tokens[i], errs[i] = p.sessionToken(t.Context(), httptest.NewRecorder(), r)
		}()
	}

	close(start)
	arrived.Wait()
	close(gate)
	done.Wait()

	for i := range concurrency {
		require.NoError(t, errs[i])
		require.Equal(t, "access-token", tokens[i], "every caller must get the refreshed token")
	}

	require.Equal(t, int64(1), exchanges.Load(),
		"the refresh token may only be exchanged once, the rest must share that result")
}

// TestSequentialRefreshExchangesEachTime pins that the deduplication is keyed on the
// refresh token and nothing else. A session that has moved on to a new one has to
// perform its own exchange, otherwise sessions would never actually renew.
func TestSequentialRefreshExchangesEachTime(t *testing.T) {
	t.Parallel()

	p, exchanges := refreshProvider(t, nil)

	for i := range 3 {
		rec := httptest.NewRecorder()
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)

		_, err := p.doRefresh(t.Context(), rec, r, sessionPayload{RefreshToken: fmt.Sprintf("refresh-token-%d", i)})
		require.NoError(t, err)
	}

	require.Equal(t, int64(3), exchanges.Load())
}

// TestRepeatRefreshIsServedFromCache covers the requests that arrive after the leader
// has left the singleflight still holding the old cookie. Exchanging again would
// replay a rotated token, which Auth0 answers by revoking the whole grant family.
func TestRepeatRefreshIsServedFromCache(t *testing.T) {
	t.Parallel()

	p, exchanges := refreshProvider(t, nil)

	for range 3 {
		rec := httptest.NewRecorder()
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)

		token, err := p.doRefresh(t.Context(), rec, r, sessionPayload{RefreshToken: "shared-refresh-token"})
		require.NoError(t, err)
		require.Equal(t, "access-token", token)
		require.NotEmpty(t, rec.Result().Cookies(), "each caller still needs its own Set-Cookie") //nolint:bodyclose // no body on a recorder result
	}

	require.Equal(t, int64(1), exchanges.Load())
}
