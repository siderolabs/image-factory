// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

// Refresh deduplication is tested in-package because it needs sessionToken and
// the provider's unexported authentication client.

package auth0

import (
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"github.com/siderolabs/image-factory/internal/testoidc"
)

// refreshProvider builds a browser-login provider whose token endpoint counts the
// exchanges it serves.
func refreshProvider(t *testing.T) (*Provider, *atomic.Int64) {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	var exchanges atomic.Int64

	issuerURL := testoidc.StartServerWithRoutes(t, privateKey, "test-key", map[string]http.HandlerFunc{
		"/oauth/token": func(w http.ResponseWriter, _ *http.Request) {
			exchanges.Add(1)

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

	parsed, err := url.Parse(issuerURL)
	require.NoError(t, err)

	p, err := NewProvider(t.Context(), zaptest.NewLogger(t), Config{
		Domain:            "tenant.example.com",
		Audience:          "https://factory.example.com",
		ClientID:          "client-id",
		ClientSecret:      "client-secret",
		RedirectURL:       "https://factory.example.com/callback",
		ExternalURL:       "https://factory.example.com",
		SessionKey:        sessionKey,
		IssuerURLOverride: issuerURL,
		HTTPClient: &http.Client{
			Transport: rewriteHost{host: parsed.Host, scheme: parsed.Scheme},
		},
	})
	require.NoError(t, err)
	require.True(t, p.BrowserLoginEnabled())
	require.NoError(t, p.warmup(t.Context()))

	return p, &exchanges
}

// rewriteHost points the go-auth0 client at the test server while leaving the
// configured tenant domain in place.
type rewriteHost struct {
	host   string
	scheme string
}

func (rt rewriteHost) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	r.URL.Scheme = rt.scheme
	r.URL.Host = rt.host
	r.Host = rt.host

	return http.DefaultTransport.RoundTrip(r)
}

// TestConcurrentRefreshExchangesOnce is the regression test for parallel htmx
// requests all refreshing the same near-expiry session. With refresh token rotation
// on, a second exchange would be a replay of an already-rotated token, and Auth0
// answers a replay by revoking the entire grant family.
func TestConcurrentRefreshExchangesOnce(t *testing.T) {
	t.Parallel()

	p, exchanges := refreshProvider(t)

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
		start = make(chan struct{})
		done  sync.WaitGroup
	)

	done.Add(concurrency)

	tokens := make([]string, concurrency)
	errs := make([]error, concurrency)

	for i := range concurrency {
		go func() {
			defer done.Done()

			r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)
			r.AddCookie(cookie)

			<-start

			tokens[i], errs[i] = p.sessionToken(t.Context(), httptest.NewRecorder(), r)
		}()
	}

	close(start)
	done.Wait()

	for i := range concurrency {
		require.NoError(t, errs[i])
		require.Equal(t, "access-token", tokens[i], "every caller must get the refreshed token")
	}

	require.Equal(t, int64(1), exchanges.Load(),
		"the refresh token may only be exchanged once, the rest must share that result")
}

// TestSequentialRefreshExchangesEachTime pins that the deduplication is scoped to
// concurrent callers. A later request has to perform its own exchange, otherwise
// singleflight would be caching results and sessions would never actually renew.
func TestSequentialRefreshExchangesEachTime(t *testing.T) {
	t.Parallel()

	p, exchanges := refreshProvider(t)

	for range 3 {
		rec := httptest.NewRecorder()
		r := httptest.NewRequestWithContext(t.Context(), http.MethodGet, "/", nil)

		_, err := p.doRefresh(t.Context(), rec, r, sessionPayload{RefreshToken: "shared-refresh-token"})
		require.NoError(t, err)
	}

	require.Equal(t, int64(3), exchanges.Load())
}
