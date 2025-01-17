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
	"net/http"
	"net/url"

	"github.com/skyssolutions/siderolabs-image-factory/pkg/schematic"
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
	baseURL *url.URL
	client  http.Client
}

// New creates a new Image Factory API client.
func New(baseURL string, options ...Option) (*Client, error) {
	opts := withDefaults(options)

	bURL, err := url.Parse(baseURL)
	if err != nil {
		return nil, err
	}

	c := &Client{
		baseURL: bURL,
		client:  opts.Client,
	}

	return c, nil
}

// BaseURL returns the base URL of the client.
func (c *Client) BaseURL() string {
	return c.baseURL.String()
}

// SchematicCreate generates new schematic from the configuration.
func (c *Client) SchematicCreate(ctx context.Context, schematic schematic.Schematic) (string, error) {
	data, err := schematic.Marshal()
	if err != nil {
		return "", err
	}

	var response struct {
		ID string `json:"id"`
	}

	if err = c.do(ctx, http.MethodPost, "/schematics", data, &response, map[string]string{
		"Content-Type": "application/yaml",
	}); err != nil {
		return "", err
	}

	return response.ID, nil
}

// Versions gets the list of Talos versions available.
func (c *Client) Versions(ctx context.Context) ([]string, error) {
	var versions []string

	if err := c.do(ctx, http.MethodGet, "/versions", nil, &versions, nil); err != nil {
		return nil, err
	}

	return versions, nil
}

// ExtensionsVersions gets the version of the extension for a Talos version.
func (c *Client) ExtensionsVersions(ctx context.Context, talosVersion string) ([]ExtensionInfo, error) {
	var versions []ExtensionInfo

	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/version/%s/extensions/official", talosVersion), nil, &versions, nil); err != nil {
		return nil, err
	}

	return versions, nil
}

// OverlaysVersions gets the version of the extension for a Talos version.
func (c *Client) OverlaysVersions(ctx context.Context, talosVersion string) ([]OverlayInfo, error) {
	var versions []OverlayInfo

	if err := c.do(ctx, http.MethodGet, fmt.Sprintf("/version/%s/overlays/official", talosVersion), nil, &versions, nil); err != nil {
		return nil, err
	}

	return versions, nil
}

func (c *Client) do(ctx context.Context, method, uri string, requestData []byte, responseData any, headers map[string]string) error {
	var reader io.Reader

	if requestData != nil {
		reader = bytes.NewReader(requestData)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL.JoinPath(uri).String(), reader)
	if err != nil {
		return err
	}

	for k, v := range headers {
		req.Header.Add(k, v)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return err
	}

	defer resp.Body.Close() //nolint:errcheck

	if err = c.checkError(resp); err != nil {
		return err
	}

	if responseData != nil {
		decoder := json.NewDecoder(resp.Body)

		return decoder.Decode(responseData)
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
