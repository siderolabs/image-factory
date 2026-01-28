// Copyright (c) 2026 Sidero Labs, Inc.
//
// Use of this software is governed by the Business Source License
// included in the LICENSE file.

//go:build enterprise

package auth

import (
	"os"

	"go.yaml.in/yaml/v4"
)

// ConfigFile represents the authentication configuration file structure.
type ConfigFile struct {
	APITokens []APIToken `yaml:"apiTokens"`
}

// APIToken represents an API token with associated username and passwords.
type APIToken struct {
	Token     string   `yaml:"token"`
	Passwords []string `yaml:"passwords"`
}

// LoadConfig loads the authentication configuration from the specified file path.
func LoadConfig(filePath string) (*ConfigFile, error) {
	in, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}

	defer in.Close() //nolint:errcheck

	var config ConfigFile

	dec := yaml.NewDecoder(in)
	dec.KnownFields(true)

	if err = dec.Decode(&config); err != nil {
		return nil, err
	}

	return &config, nil
}
