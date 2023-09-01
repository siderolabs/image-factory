// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package configuration provides a data model for requested image configuration.
package configuration

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"

	"github.com/siderolabs/gen/xerrors"
	"gopkg.in/yaml.v3"
)

// Configuration represents the requested image configuration.
type Configuration struct {
	// Customization represents the Talos image customization.
	Customization Customization `yaml:"customization"`
}

// Customization represents the Talos image customization.
type Customization struct {
	// Extra kernel arguments to be passed to the kernel.
	ExtraKernelArgs []string `yaml:"extraKernelArgs,omitempty"`
	// SystemExtensions represents the Talos system extensions to be installed.
	SystemExtensions SystemExtensions `yaml:"systemExtensions,omitempty"`
}

// SystemExtensions represents the Talos system extensions to be installed.
type SystemExtensions struct {
	// OfficialExtensions represents the Talos official system extensions to be installed.
	//
	// The image service will pick up automatically the version compatible with Talos version.
	OfficialExtensions []string `yaml:"officialExtensions,omitempty"`
}

// InvalidErrorTag is a tag for invalid configuration errors.
type InvalidErrorTag struct{}

// Unmarshal the configuration from text representation.
func Unmarshal(data []byte) (*Configuration, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)

	var cfg Configuration

	if err := dec.Decode(&cfg); err != nil {
		return nil, xerrors.NewTagged[InvalidErrorTag](err)
	}

	return &cfg, nil
}

// Marshal the configuration to text representation.
//
// Marshal result should be stable if new configuration fields are added.
func (cfg *Configuration) Marshal() ([]byte, error) {
	return yaml.Marshal(cfg)
}

// ID returns the identifier of the configuration.
//
// ID is stable (does not change if the configuration is same), but it should not be possible
// to derive the configuration from the ID.
//
// Key is a secret used to generate the ID.
func (cfg *Configuration) ID(key []byte) (string, error) {
	data, err := cfg.Marshal()
	if err != nil {
		return "", err
	}

	hasher := hmac.New(sha256.New, key)

	if _, err := hasher.Write(data); err != nil {
		return "", err
	}

	binaryID := hasher.Sum(nil)

	return hex.EncodeToString(binaryID), nil
}
