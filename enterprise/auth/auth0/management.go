// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

// Auth0 Management API client: creates, lists and deletes per-org node M2M applications.
//
// Auth0's Organizations feature that would otherwise bind a client to an org (the "organization"
// token-request parameter, org-client-grant association) is gated to plans this tenant doesn't
// have. Org identity is carried instead via client_metadata, read back into the org_id claim by
// a tenant-level Auth0 Action bound to the Client Credentials Exchange flow (dashboard setup,
// not code in this repository).

package auth0

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"

	"golang.org/x/oauth2/clientcredentials"
)

const (
	// managementRequestTimeout bounds a single-resource Management API call.
	managementRequestTimeout = 10 * time.Second

	// managementListTimeout bounds ListNodeClients, which pages through every client in the
	// tenant (client_metadata cannot be filtered server-side) rather than a single resource.
	managementListTimeout = 30 * time.Second

	// managementMaxResponseBytes bounds a single Management API response body.
	managementMaxResponseBytes = 1 << 20

	// managementListPageSize is the checkpoint-pagination page size; 100 is Auth0's max.
	managementListPageSize = 100

	// managementMaxListPages guards against an unbounded loop if the Management API ever
	// returns a non-terminating next cursor; not expected to bind in normal operation.
	managementMaxListPages = 1000

	// managedByTag marks a client as owned by image-factory in client_metadata, distinguishing
	// it from any other application in the tenant when ListNodeClients walks the full list.
	managedByTag = "image-factory"
)

// management holds the Management API client credentials grant.
type management struct {
	httpClient *http.Client // injects the Authorization header and refreshes the token
	baseURL    *url.URL     // tenantURL + /api/v2/
}

// newManagement builds a Management API client from the same clientID/clientSecret used for
// browser login. ctx scopes the token source's HTTP client, matching how NewProvider's ctx
// scopes the JWKS client — both live for the process lifetime, not a single request.
func newManagement(ctx context.Context, tenantURL *url.URL, cfg Config) *management {
	baseURL := tenantURL.JoinPath("/api/v2/")

	cc := &clientcredentials.Config{
		ClientID:     cfg.ClientID,
		ClientSecret: cfg.ClientSecret,
		TokenURL:     tenantURL.JoinPath("/oauth/token").String(),
		EndpointParams: url.Values{
			"audience": {baseURL.String()},
		},
	}

	return &management{
		httpClient: cc.Client(ctx),
		baseURL:    baseURL,
	}
}

// NodeClient is the subset of an Auth0 M2M client surfaced for node-token management.
type NodeClient struct {
	// CreatedAt is read back from client_metadata.created_at, stamped by CreateNodeClient.
	// The Auth0 client resource carries no creation timestamp of its own.
	CreatedAt time.Time

	ClientID string
	Name     string
}

// nodeClientMetadata is stamped onto every client this package creates. It is the only place
// org ownership is recorded: Auth0 has no server-side way to filter or search
// client_metadata, so ListNodeClients pages through every client in the tenant and filters on
// this in Go.
type nodeClientMetadata struct {
	ManagedBy string `json:"managed_by"`
	OrgID     string `json:"org_id"`
	CreatedAt string `json:"created_at"` // RFC3339
}

// clientResource is the subset of the Auth0 client resource this package reads or writes.
type clientResource struct {
	Metadata     nodeClientMetadata `json:"client_metadata"`
	ClientID     string             `json:"client_id,omitempty"`
	ClientSecret string             `json:"client_secret,omitempty"`
	Name         string             `json:"name,omitempty"`
	AppType      string             `json:"app_type,omitempty"`
	GrantTypes   []string           `json:"grant_types,omitempty"`
}

// clientListPage is one page of a checkpoint-paginated GET /api/v2/clients response.
type clientListPage struct {
	Next    string           `json:"next"`
	Clients []clientResource `json:"clients"`
}

// clientGrantRequest authorizes a client for an API and scope via POST /api/v2/client-grants.
type clientGrantRequest struct {
	ClientID string   `json:"client_id"`
	Audience string   `json:"audience"`
	Scope    []string `json:"scope,omitempty"`
}

// validNodeClientIDPattern matches an Auth0 client_id: an opaque alphanumeric string. This is
// deliberately conservative, since clientID reaches this package from a caller-supplied URL
// parameter (enterprise/nodetoken) and is used to build a Management API request path — an
// unvalidated "/" or ".." would let a caller reshape that path.
var validNodeClientIDPattern = regexp.MustCompile(`^[A-Za-z0-9]+$`)

func validNodeClientID(s string) bool {
	return validNodeClientIDPattern.MatchString(s)
}

// CreateNodeClient creates a new Auth0 M2M application scoped to the Image Factory API,
// tagged with org ownership metadata, and returns its credentials. The secret is only ever
// available here, at creation time — Auth0 never returns it again afterward.
func (p *Provider) CreateNodeClient(ctx context.Context, orgID, name string) (clientID, clientSecret string, err error) {
	ctx, cancel := context.WithTimeout(ctx, managementRequestTimeout)
	defer cancel()

	reqBody := clientResource{
		Name:       name,
		AppType:    "non_interactive",
		GrantTypes: []string{"client_credentials"},
		Metadata: nodeClientMetadata{
			ManagedBy: managedByTag,
			OrgID:     orgID,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
		},
	}

	var created clientResource

	if err = p.management.do(ctx, http.MethodPost, "clients", nil, reqBody, &created); err != nil {
		return "", "", fmt.Errorf("auth0: create node client: %w", err)
	}

	grant := clientGrantRequest{ClientID: created.ClientID, Audience: p.audience}
	if p.machineScope != "" {
		grant.Scope = []string{p.machineScope}
	}

	if err = p.management.do(ctx, http.MethodPost, "client-grants", nil, grant, nil); err != nil {
		return "", "", fmt.Errorf("auth0: authorize node client: %w", err)
	}

	return created.ClientID, created.ClientSecret, nil
}

// ListNodeClients returns orgID's node clients. It pages through every client in the tenant,
// since client_metadata cannot be filtered server-side — see the package doc comment.
func (p *Provider) ListNodeClients(ctx context.Context, orgID string) ([]NodeClient, error) {
	ctx, cancel := context.WithTimeout(ctx, managementListTimeout)
	defer cancel()

	var (
		result []NodeClient
		cursor string
	)

	for range managementMaxListPages {
		query := url.Values{"take": {strconv.Itoa(managementListPageSize)}}
		if cursor != "" {
			query.Set("from", cursor)
		}

		var page clientListPage

		if err := p.management.do(ctx, http.MethodGet, "clients", query, nil, &page); err != nil {
			return nil, fmt.Errorf("auth0: list node clients: %w", err)
		}

		for _, c := range page.Clients {
			if c.Metadata.ManagedBy != managedByTag || c.Metadata.OrgID != orgID {
				continue
			}

			result = append(result, NodeClient{
				ClientID:  c.ClientID,
				Name:      c.Name,
				CreatedAt: parseNodeClientCreatedAt(c.Metadata.CreatedAt),
			})
		}

		if page.Next == "" {
			return result, nil
		}

		cursor = page.Next
	}

	return nil, fmt.Errorf("auth0: list node clients: exceeded %d pages without exhausting the tenant's client list", managementMaxListPages)
}

// parseNodeClientCreatedAt parses the created_at stamp CreateNodeClient writes into
// client_metadata. A parse failure returns the zero time rather than failing the whole list,
// since Auth0 itself never validates this field's format.
func parseNodeClientCreatedAt(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}

	return t
}

// DeleteNodeClient deletes clientID after verifying its client_metadata.org_id matches orgID.
// clientID is only ever reached via the frontend's URL param and must never be trusted alone —
// this ownership check is what stops one org from revoking another org's node token.
func (p *Provider) DeleteNodeClient(ctx context.Context, orgID, clientID string) error {
	if !validNodeClientID(clientID) {
		return errors.New("auth0: invalid node client id")
	}

	ctx, cancel := context.WithTimeout(ctx, managementRequestTimeout)
	defer cancel()

	var existing clientResource

	if err := p.management.do(ctx, http.MethodGet, "clients/"+clientID, nil, nil, &existing); err != nil {
		return fmt.Errorf("auth0: look up node client: %w", err)
	}

	if existing.Metadata.ManagedBy != managedByTag || existing.Metadata.OrgID != orgID {
		return errors.New("auth0: node client not found")
	}

	if err := p.management.do(ctx, http.MethodDelete, "clients/"+clientID, nil, nil, nil); err != nil {
		return fmt.Errorf("auth0: delete node client: %w", err)
	}

	return nil
}

// do executes a Management API request and decodes a JSON response into out, unless out is
// nil. query and body may be nil.
func (m *management) do(ctx context.Context, method, resourcePath string, query url.Values, body, out any) error {
	reqURL := *m.baseURL.JoinPath(resourcePath)
	if len(query) > 0 {
		reqURL.RawQuery = query.Encode()
	}

	var reqBody io.Reader

	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode request: %w", err)
		}

		reqBody = bytes.NewReader(encoded)
	}

	req, err := http.NewRequestWithContext(ctx, method, reqURL.String(), reqBody)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, managementMaxResponseBytes))
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%s %s: %s: %s", method, resourcePath, resp.Status, truncate(string(respBody), 500))
	}

	if out == nil || len(respBody) == 0 {
		return nil
	}

	if err := json.Unmarshal(respBody, out); err != nil {
		return fmt.Errorf("decode response: %w", err)
	}

	return nil
}
