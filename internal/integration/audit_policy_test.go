// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

//go:build integration

package integration_test

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/siderolabs/image-factory/cmd/image-factory/cmd"
	"github.com/siderolabs/image-factory/internal/audit"
	"github.com/siderolabs/image-factory/pkg/enterprise"
)

func testAuditPolicy(options cmd.Options) func(t *testing.T) {
	return func(t *testing.T) {
		auditPath := filepath.Join(t.TempDir(), "audit.log")

		options.Audit.Mode = cmd.AuditModeFile
		options.Audit.File.Path = auditPath
		options.Audit.File.MaxSizeMB = 1
		options.Metrics.Namespace = "test_audit_policy"

		if enterprise.Enabled() {
			options.Enterprise.Scanner.DatabaseURL = "http://127.0.0.1:1/databases"
			options.Enterprise.Scanner.DatabaseUpdateAt = ""
			options.Enterprise.Scanner.DatabaseRootDir = t.TempDir()
		}

		ctx, listenAddr, _, metricsAddr := setupFactoryWithMetrics(t, options)
		baseURL := "http://" + listenAddr

		for _, method := range []string{http.MethodGet, http.MethodHead} {
			req, err := http.NewRequestWithContext(ctx, method, baseURL+"/readyz", nil)
			require.NoError(t, err)

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)

			body, err := io.ReadAll(resp.Body)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())

			assert.NotEmpty(t, resp.Header.Get("Server"))
			assert.NotEmpty(t, resp.Header.Get("X-Request-ID"))

			if enterprise.Enabled() {
				assert.Equal(t, http.StatusServiceUnavailable, resp.StatusCode)
				if method == http.MethodGet {
					assert.Equal(t, "not ready\n", string(body))
				} else {
					assert.Empty(t, body)
				}
			} else {
				assert.Equal(t, http.StatusOK, resp.StatusCode)
				assert.Empty(t, body)
			}
		}

		doRequest := func(method, path, body string, configure func(*http.Request), expectedStatus int) {
			t.Helper()

			req, err := http.NewRequestWithContext(ctx, method, baseURL+path, strings.NewReader(body))
			require.NoError(t, err)

			if configure == nil {
				addTestAuth(req)
			} else {
				configure(req)
			}

			resp, err := http.DefaultClient.Do(req)
			require.NoError(t, err)
			require.NoError(t, resp.Body.Close())
			require.Equal(t, expectedStatus, resp.StatusCode)
		}

		// Public routes are observed but not audited.
		doRequest(http.MethodGet, "/versions", "", nil, http.StatusOK)

		// Static routes and router-level misses bypass both observation and audit.
		doRequest(http.MethodGet, "/css/output.css", "", nil, http.StatusOK)
		doRequest(http.MethodGet, "/not-a-route", "", nil, http.StatusNotFound)

		// Protected successes and handler failures are audited even when
		// authentication is disabled in the Community build.
		doRequest(http.MethodHead, "/", "", nil, http.StatusOK)
		doRequest(http.MethodPost, "/schematics", "{", func(req *http.Request) {
			req.Header.Set("Content-Type", "application/json")
			addTestAuth(req)
		}, http.StatusBadRequest)

		expectedRecords := 2

		if enterprise.Enabled() {
			// Provider denials are audited, but the current capture layer does not
			// retain the rejected identity because the provider never calls next.
			doRequest(http.MethodHead, "/", "", func(req *http.Request) {
				req.SetBasicAuth("alice", "incorrect")
			}, http.StatusUnauthorized)

			status, created := createToken(ctx, t, baseURL, `{"name":"audit-token","scopes":["image:read"]}`)
			require.Equal(t, http.StatusOK, status)

			token, _ := created["token"].(string)
			require.NotEmpty(t, token)

			doRequest(http.MethodGet, "/v2/", "", func(req *http.Request) {
				req.Header.Set("Authorization", "Bearer "+token)
			}, http.StatusOK)

			expectedRecords += 3 // provider denial, token creation, and token-authenticated request
		}

		var records []audit.Record

		require.Eventually(t, func() bool {
			contents, err := os.ReadFile(auditPath)
			if err != nil {
				return false
			}

			lines := strings.Split(strings.TrimSpace(string(contents)), "\n")
			if len(lines) != expectedRecords {
				return false
			}

			records = make([]audit.Record, 0, len(lines))

			for _, line := range lines {
				var record audit.Record
				if err = json.Unmarshal([]byte(line), &record); err != nil {
					return false
				}

				records = append(records, record)
			}

			return true
		}, time.Second, 10*time.Millisecond)

		require.Len(t, records, expectedRecords)

		for _, record := range records {
			assert.NotZero(t, record.Time)
			assert.NotEmpty(t, record.RequestID)
			assert.NotEmpty(t, record.ClientIP)
			assert.Positive(t, record.Duration)
		}

		assertAuditRecord(t, records, http.MethodHead, "/", http.StatusOK, false)
		assertAuditRecord(t, records, http.MethodPost, "/schematics", http.StatusBadRequest, true)

		if enterprise.Enabled() {
			assertAuditRecord(t, records, http.MethodHead, "/", http.StatusUnauthorized, true)
			assertAuditRecord(t, records, http.MethodPost, "/tokens", http.StatusOK, false)
			assertAuditRecord(t, records, http.MethodGet, "/v2/", http.StatusOK, false)

			for _, record := range records {
				if record.Status == http.StatusUnauthorized {
					assert.Empty(t, record.Username, "%s %s status %d", record.Method, record.Path, record.Status)

					continue
				}

				assert.Equal(t, "alice", record.Username, "%s %s status %d", record.Method, record.Path, record.Status)
			}
		}

		resp, err := http.Get("http://" + metricsAddr + "/metrics")
		require.NoError(t, err)
		defer resp.Body.Close() //nolint:errcheck

		require.Equal(t, http.StatusOK, resp.StatusCode)

		metricsBody, err := io.ReadAll(resp.Body)
		require.NoError(t, err)

		metricsText := string(metricsBody)
		assert.Contains(t, metricsText, "test_audit_policy_http_request_duration_seconds")
		assert.Contains(t, metricsText, `handler="/versions"`)
		assert.Contains(t, metricsText, `code="200",handler="/versions",method="GET",service=""} 1`)
		assert.Contains(t, metricsText, `handler="/"`)
		assert.Contains(t, metricsText, `handler="/schematics"`)
		assert.Contains(t, metricsText, `code="400",handler="/schematics",method="POST",service=""} 1`)
		assert.NotContains(t, metricsText, `handler="/css/*filepath"`)
		assert.NotContains(t, metricsText, `handler="/not-a-route"`)

		if enterprise.Enabled() {
			assert.Contains(t, metricsText, `code="503",handler="/readyz",method="GET",service=""} 1`)
			assert.Contains(t, metricsText, `code="503",handler="/readyz",method="HEAD",service=""} 1`)
			assert.Contains(t, metricsText, `code="401",handler="/",method="HEAD",service=""} 1`)
			assert.Contains(t, metricsText, `code="200",handler="/v2/*path",method="GET",service=""} 1`)
		} else {
			assert.Contains(t, metricsText, `code="200",handler="/readyz",method="GET",service=""} 1`)
			assert.Contains(t, metricsText, `code="200",handler="/readyz",method="HEAD",service=""} 1`)
		}
	}
}

func testAuditSinkFailure(options cmd.Options) func(t *testing.T) {
	return func(t *testing.T) {
		// lumberjack cannot open a directory as its log file, which makes every
		// audit write fail after the frontend has started.
		options.Audit.Mode = cmd.AuditModeFile
		options.Audit.File.Path = t.TempDir()
		options.Audit.File.MaxSizeMB = 1
		options.Metrics.Namespace = "test_audit_sink_failure"

		if enterprise.Enabled() {
			options.Enterprise.Scanner.DatabaseURL = "http://127.0.0.1:1/databases"
			options.Enterprise.Scanner.DatabaseUpdateAt = ""
			options.Enterprise.Scanner.DatabaseRootDir = t.TempDir()
		}

		ctx, listenAddr, _ := setupFactory(t, options)

		req, err := http.NewRequestWithContext(ctx, http.MethodHead, "http://"+listenAddr+"/", nil)
		require.NoError(t, err)

		addTestAuth(req)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		defer resp.Body.Close() //nolint:errcheck

		assert.Equal(t, http.StatusOK, resp.StatusCode)
		assert.NotEmpty(t, resp.Header.Get("X-Request-ID"))
	}
}

func assertAuditRecord(t *testing.T, records []audit.Record, method, path string, status int, expectsError bool) {
	t.Helper()

	for _, record := range records {
		if record.Method != method || record.Path != path || record.Status != status {
			continue
		}

		if expectsError {
			assert.NotEmpty(t, record.Error)
		} else {
			assert.Empty(t, record.Error)
		}

		return
	}

	assert.Failf(t, "audit record not found", "%s %s with status %d", method, path, status)
}
