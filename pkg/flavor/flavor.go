// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package flavor provides a data model for requested image flavor.
package flavor

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"

	"github.com/siderolabs/gen/xerrors"
	"gopkg.in/yaml.v3"
)

// Flavor represents the requested image customization.
type Flavor struct {
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

// InvalidErrorTag is a tag for invalid flavor errors.
type InvalidErrorTag struct{}

// Unmarshal the flavor from text representation.
func Unmarshal(data []byte) (*Flavor, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)

	var cfg Flavor

	if err := dec.Decode(&cfg); err != nil {
		return nil, xerrors.NewTagged[InvalidErrorTag](err)
	}

	return &cfg, nil
}

// Marshal the flavor to text representation.
//
// Marshal result should be stable if new flavor fields are added.
func (cfg *Flavor) Marshal() ([]byte, error) {
	return yaml.Marshal(cfg)
}

// ID returns the identifier of the flavor.
//
// ID is stable (does not change if the flavor is same).
// ID matches sha256 hash of the canonical representation of the flavor.
func (cfg *Flavor) ID() (string, error) {
	data, err := cfg.Marshal()
	if err != nil {
		return "", err
	}

	binaryID := sha256.Sum256(data)

	return hex.EncodeToString(binaryID[:]), nil
}
