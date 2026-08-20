// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

// Auth0 Management API client: creates, lists and deletes per-org node M2M applications.
//
// Org identity is carried via client_metadata, since Organizations is plan-gated on this tenant.

package auth0

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"time"

	auth0mgmt "github.com/auth0/go-auth0/v3/management"
	auth0mgmtclient "github.com/auth0/go-auth0/v3/management/client"
	auth0option "github.com/auth0/go-auth0/v3/management/option"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

const (
	managementRequestTimeout = 10 * time.Second
	managementListTimeout    = 30 * time.Second

	managementListPageSize = 100
	// managementMaxListResults guards against an unbounded loop; Auth0 itself caps offset
	// pagination around 1000 results tenant-wide, so this is not expected to bind first.
	managementMaxListResults = 1000

	// managementTokenFetchTimeout bounds the client-credentials token source's HTTP client,
	// captured once at construction, since it isn't covered by a later per-request timeout.
	managementTokenFetchTimeout = 10 * time.Second

	managedByTag = "image-factory"
)

// newManagementSDK builds the Management API client. extraOpts lets export_test.go pass
// options like WithMaxAttempts to skip retry backoff in error-case tests.
func newManagementSDK(ctx context.Context, tenantURL *url.URL, cfg Config, extraOpts ...auth0option.RequestOption) (*auth0mgmtclient.Management, error) {
	tokenCtx := context.WithValue(ctx, oauth2.HTTPClient, &http.Client{Timeout: managementTokenFetchTimeout})

	opts := append([]auth0option.RequestOption{
		auth0option.WithBaseURL(tenantURL.JoinPath("/api/v2").String()),
		auth0option.WithClientCredentials(tokenCtx, cfg.ClientID, cfg.ClientSecret),
		auth0option.WithHTTPClient(&http.Client{}),
	}, extraOpts...)

	return auth0mgmtclient.New(tenantURL.Host, opts...)
}

// NodeClient is the subset of an Auth0 M2M client surfaced for node-token management.
type NodeClient struct {
	CreatedAt time.Time
	ClientID  string
	Name      string
}

func metadataString(md auth0mgmt.ClientMetadata, key string) string {
	if s, ok := md[key].(string); ok {
		return s
	}

	return ""
}

// ErrInvalidNodeClientID and ErrNodeClientNotFound let callers distinguish DeleteNodeClient's
// failure modes via errors.Is, e.g. to map both to an HTTP 404.
var (
	ErrInvalidNodeClientID = errors.New("auth0: invalid node client id")
	ErrNodeClientNotFound  = errors.New("auth0: node client not found")
)

// ErrMachineScopeRequired is returned by CreateNodeClient when MachineScope is unconfigured.
var ErrMachineScopeRequired = errors.New("auth0: machineScope must be configured to create node clients")

var validNodeClientIDPattern = regexp.MustCompile(`^[A-Za-z0-9]+$`)

func validNodeClientID(s string) bool {
	return validNodeClientIDPattern.MatchString(s)
}

// CreateNodeClient creates an Auth0 M2M application scoped to orgID and returns its
// credentials. The secret is only ever available here — Auth0 never returns it again.
func (p *Provider) CreateNodeClient(ctx context.Context, orgID, name string) (clientID, clientSecret string, err error) {
	// An empty MachineScope means "unrestricted" for bearer-token verification elsewhere, but
	// minting a new M2M credential with that same emptiness would grant it every scope on the
	// audience (Auth0 requires allow_all_scopes when scope is empty) -- too large a blast
	// radius to fall into by omission, so node clients require an explicit scope to exist.
	if p.machineScope == "" {
		return "", "", ErrMachineScopeRequired
	}

	ctx, cancel := context.WithTimeout(ctx, managementRequestTimeout)
	defer cancel()

	created, err := p.sdk.Clients.Create(ctx, &auth0mgmt.CreateClientRequestContent{
		Name:       name,
		AppType:    auth0mgmt.ClientAppTypeEnumNonInteractive.Ptr(),
		GrantTypes: []string{"client_credentials"},
		ClientMetadata: &auth0mgmt.ClientMetadata{
			"managed_by": managedByTag,
			"org_id":     orgID,
			"created_at": time.Now().UTC().Format(time.RFC3339),
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("auth0: create node client: %w", err)
	}

	grant := &auth0mgmt.CreateClientGrantRequestContent{
		ClientID: created.ClientID,
		Audience: p.audience,
		Scope:    []string{p.machineScope},
	}

	if _, err = p.sdk.ClientGrants.Create(ctx, grant); err != nil {
		// A fresh timeout, not ctx's: if the grant call failed because ctx's deadline
		// expired, reusing it here would make this cleanup fail immediately too.
		cleanupCtx, cleanupCancel := context.WithTimeout(context.WithoutCancel(ctx), managementRequestTimeout)
		defer cleanupCancel()

		if delErr := p.sdk.Clients.Delete(cleanupCtx, created.GetClientID()); delErr != nil {
			p.logger.Warn("auth0: failed to clean up node client after authorization failure",
				zap.String("client_id", created.GetClientID()), zap.Error(delErr))
		}

		return "", "", fmt.Errorf("auth0: authorize node client: %w", err)
	}

	return created.GetClientID(), created.GetClientSecret(), nil
}

// ListNodeClients returns orgID's node clients. It scans every client in the tenant, since
// client_metadata cannot be filtered server-side — see the package doc comment. Checkpoint
// (from/take) pagination can't help here either: Auth0 only permits it in combination with two
// Organizations-specific q queries, so this falls back to offset pagination, capped by Auth0
// around 1000 results tenant-wide.
func (p *Provider) ListNodeClients(ctx context.Context, orgID string) ([]NodeClient, error) {
	ctx, cancel := context.WithTimeout(ctx, managementListTimeout)
	defer cancel()

	page, perPage, includeTotals := 0, managementListPageSize, true

	resp, err := p.sdk.Clients.List(ctx, &auth0mgmt.ListClientsRequestParameters{
		Page:          &page,
		PerPage:       &perPage,
		IncludeTotals: &includeTotals,
	})
	if err != nil {
		return nil, fmt.Errorf("auth0: list node clients: %w", err)
	}

	var result []NodeClient

	it := resp.Iterator()
	for n := 0; it.Next(ctx); n++ {
		if n >= managementMaxListResults {
			return nil, fmt.Errorf("auth0: list node clients: exceeded %d results without exhausting the tenant's client list", managementMaxListResults)
		}

		c := it.Current()
		metadata := c.GetClientMetadata()

		if metadataString(metadata, "managed_by") != managedByTag || metadataString(metadata, "org_id") != orgID {
			continue
		}

		result = append(result, NodeClient{
			ClientID:  c.GetClientID(),
			Name:      c.GetName(),
			CreatedAt: parseNodeClientCreatedAt(metadataString(metadata, "created_at")),
		})
	}

	if err := it.Err(); err != nil {
		return nil, fmt.Errorf("auth0: list node clients: %w", err)
	}

	return result, nil
}

// parseNodeClientCreatedAt parses the created_at stamp CreateNodeClient writes into
// client_metadata, returning the zero time on failure rather than failing the whole list.
func parseNodeClientCreatedAt(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}

	return t
}

// DeleteNodeClient deletes clientID after verifying its client_metadata.org_id matches orgID.
func (p *Provider) DeleteNodeClient(ctx context.Context, orgID, clientID string) error {
	if !validNodeClientID(clientID) {
		return ErrInvalidNodeClientID
	}

	ctx, cancel := context.WithTimeout(ctx, managementRequestTimeout)
	defer cancel()

	existing, err := p.sdk.Clients.Get(ctx, clientID, &auth0mgmt.GetClientRequestParameters{})
	if err != nil {
		var notFound *auth0mgmt.NotFoundError
		if errors.As(err, &notFound) {
			return ErrNodeClientNotFound
		}

		return fmt.Errorf("auth0: look up node client: %w", err)
	}

	metadata := existing.GetClientMetadata()
	if metadataString(metadata, "managed_by") != managedByTag || metadataString(metadata, "org_id") != orgID {
		return ErrNodeClientNotFound
	}

	if err := p.sdk.Clients.Delete(ctx, clientID); err != nil {
		return fmt.Errorf("auth0: delete node client: %w", err)
	}

	return nil
}
