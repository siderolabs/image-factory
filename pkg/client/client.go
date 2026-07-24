// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package client implements image factory HTTP API client.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"net/url"

	"github.com/siderolabs/image-factory/pkg/schematic"
)

// ExtensionInfo defines extensions versions list response item.
type ExtensionInfo struct {
	Name        string `json:"name"`
	Ref         string `json:"ref"`
	Digest      string `json:"digest"`
	Author      string `json:"author"`
	Description string `json:"description"`
}

// OverlayInfo defines overlay versions list response item.
type OverlayInfo struct {
	Name   string `json:"name"`
	Image  string `json:"image"`
	Ref    string `json:"ref"`
	Digest string `json:"digest"`
}

// Client is the Image Factory HTTP API client.
type Client struct {
	baseURL      *url.URL
	extraHeaders http.Header
	client       http.Client
}

// New creates a new Image Factory API client.
func New(baseURL string, options ...Option) (*Client, error) {
	opts := withDefaults(options)

	bURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	c := &Client{
		baseURL:      bURL,
		client:       opts.Client,
		extraHeaders: opts.ExtraHeaders,
	}

	return c, nil
}

// BaseURL returns the base URL of the client.
func (c *Client) BaseURL() string {
	return c.baseURL.String()
}

// SchematicCreate generates new schematic from the configuration.
//
// It returns the schematic ID and the normalized schematic as returned by the factory.
// The normalized schematic should be considered authoritative as it may differ from the
// input (for example, the factory may set the owner field for authenticated requests).
func (c *Client) SchematicCreate(ctx context.Context, sc schematic.Schematic) (string, *schematic.Schematic, error) {
	data, err := sc.Marshal()
	if err != nil {
		return "", nil, err
	}

	var response struct {
		ID        string `json:"id"`
		Schematic string `json:"schematic"`
	}

	if err = c.do(
		ctx, http.MethodPost, "/schematics", &response,
		WithRequestData(data),
		WithHeaders(map[string]string{"Content-Type": "application/yaml"}),
	); err != nil {
		return "", nil, err
	}

	if response.Schematic == "" {
		// Older factories don't return the schematic in the create response, fetch it separately.
		normalized, getErr := c.SchematicGet(ctx, response.ID)
		if getErr != nil {
			return "", nil, getErr
		}

		return response.ID, normalized, nil
	}

	normalized, err := schematic.Unmarshal([]byte(response.Schematic))
	if err != nil {
		return "", nil, err
	}

	return response.ID, normalized, nil
}

func (c *Client) SchematicGet(ctx context.Context, schematicID string) (*schematic.Schematic, error) {
	var response schematic.Schematic

	if err := c.do(ctx, http.MethodGet, "/schematics/"+schematicID, &response); err != nil {
		return nil, err
	}

	return &response, nil
}

// Versions gets the list of Talos versions available.
func (c *Client) Versions(ctx context.Context) ([]string, error) {
	var versions []string

	if err := c.do(ctx, http.MethodGet, "/versions", &versions); err != nil {
		return nil, err
	}

	return versions, nil
}

// BrokenVersions gets the list of Talos versions marked as broken by the factory.
func (c *Client) BrokenVersions(ctx context.Context) ([]string, error) {
	var versions []string

	if err := c.do(
		ctx, http.MethodGet, "/versions", &versions,
		WithQueryParams(url.Values{"broken": {"true"}}),
	); err != nil {
		return nil, err
	}

	return versions, nil
}

// ExtensionsVersions gets the version of the extension for a Talos version.
func (c *Client) ExtensionsVersions(ctx context.Context, talosVersion string) ([]ExtensionInfo, error) {
	var versions []ExtensionInfo

	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/version/%s/extensions/official", talosVersion), &versions); err != nil {
		return nil, err
	}

	return versions, nil
}

// TalosctlList gets the list of talosctl download URLs for a Talos version.
func (c *Client) TalosctlList(ctx context.Context, talosVersion string) ([]string, error) {
	var urls []string

	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/talosctl/%s", talosVersion), &urls); err != nil {
		return nil, err
	}

	return urls, nil
}

// OverlaysVersions gets the version of the extension for a Talos version.
func (c *Client) OverlaysVersions(ctx context.Context, talosVersion string) ([]OverlayInfo, error) {
	var versions []OverlayInfo

	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/version/%s/overlays/official", talosVersion), &versions); err != nil {
		return nil, err
	}

	return versions, nil
}

// DownloadToken requests a short-lived JWT download token scoped to the
// authenticated caller's identity. The token can be appended as ?token= to
// any image download URL; one token covers all schematics owned by the caller.
func (c *Client) DownloadToken(ctx context.Context) (string, error) {
	var response struct {
		Token string `json:"token"`
	}

	if err := c.do(ctx, http.MethodPost, "/download-token", &response); err != nil {
		return "", err
	}

	return response.Token, nil
}

// ScanReport downloads a vulnerability scan report for the given schematic, Talos version,
// architecture, and report filename. The filename extension selects the report format:
// ".sarif" → SARIF, ".cdx" → CycloneDX, ".json" → JSON, ".table" → plain-text table.
func (c *Client) ScanReport(ctx context.Context, schematicID, talosVersion, arch, filename string) ([]byte, error) {
	var data []byte

	if err := c.do(ctx, http.MethodGet,
		fmt.Sprintf("/scans/%s/%s/%s/%s", schematicID, talosVersion, arch, filename),
		&data); err != nil {
		return nil, err
	}

	return data, nil
}

// requestOptions are options that can be applied to a request via [requestOption] functions.
type requestOptions struct {
	headers     map[string]string
	query       url.Values
	requestData []byte
}

// requestOption configures a request issued via [Client.do].
type requestOption func(*requestOptions)

// WithRequestData attaches a request body.
func WithRequestData(data []byte) requestOption {
	return func(o *requestOptions) {
		o.requestData = data
	}
}

// WithHeaders attaches request headers.
func WithHeaders(headers map[string]string) requestOption {
	return func(o *requestOptions) {
		o.headers = headers
	}
}

// WithQueryParams attaches URL query parameters.
func WithQueryParams(query url.Values) requestOption {
	return func(o *requestOptions) {
		o.query = query
	}
}

func (c *Client) do(ctx context.Context, method, uri string, responseData any, opts ...requestOption) error {
	var ro requestOptions

	for _, opt := range opts {
		opt(&ro)
	}

	var reader io.Reader

	if ro.requestData != nil {
		reader = bytes.NewReader(ro.requestData)
	}

	endpoint := c.baseURL.JoinPath(uri)
	if len(ro.query) > 0 {
		endpoint.RawQuery = ro.query.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), reader)
	if err != nil {
		return err
	}

	for k, v := range ro.headers {
		req.Header.Add(k, v)
	}

	maps.Copy(req.Header, c.extraHeaders)

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close() //nolint:errcheck

	if err = c.checkError(resp); err != nil {
		return err
	}

	if responseData != nil {
		switch v := responseData.(type) {
		case *schematic.Schematic:
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				return err
			}

			retrieved, err := schematic.Unmarshal(data)
			if err != nil {
				return err
			}

			*v = *retrieved

			return nil
		case *[]byte:
			var err error

			*v, err = io.ReadAll(resp.Body)

			return err
		default:
			decoder := json.NewDecoder(resp.Body)

			return decoder.Decode(responseData)
		}
	}

	return nil
}

func (c *Client) checkError(resp *http.Response) error {
	const maxErrorBody = 8192

	if resp.StatusCode < http.StatusBadRequest {
		return nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	if err != nil {
		return err
	}

	err = &HTTPError{
		Code:    resp.StatusCode,
		Message: string(body),
	}

	if resp.StatusCode == http.StatusBadRequest {
		return &InvalidSchematicError{
			e: err,
		}
	}

	return err
}
