// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

// Package schematic provides a data model for requested image schematic.
package schematic

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"

	"github.com/siderolabs/gen/xerrors"
	"gopkg.in/yaml.v3"
)

// Schematic represents the requested image customization.
type Schematic struct {
	// Overlay represents the overlay options for image generation.
	Overlay Overlay `yaml:"overlay,omitempty"`
	// Customization represents the Talos image customization.
	Customization Customization `yaml:"customization"`
}

// Customization represents the Talos image customization.
type Customization struct {
	// Extra kernel arguments to be passed to the kernel.
	ExtraKernelArgs []string `yaml:"extraKernelArgs,omitempty"`
	// Meta provides initial META contents for the image.
	Meta []MetaValue `yaml:"meta,omitempty"`
	// SystemExtensions represents the Talos system extensions to be installed.
	SystemExtensions SystemExtensions `yaml:"systemExtensions,omitempty"`
}

// MetaValue provides initial META contents for the image.
type MetaValue struct { //nolint:govet
	// Key is the META key.
	Key uint8 `yaml:"key"`
	// Value is the META value.
	Value string `yaml:"value"`
}

// SystemExtensions represents the Talos system extensions to be installed.
type SystemExtensions struct {
	// OfficialExtensions represents the Talos official system extensions to be installed.
	//
	// The image factory will pick up automatically the version compatible with Talos version.
	OfficialExtensions []string `yaml:"officialExtensions,omitempty"`
}

// Overlay represents the overlay options for image generation.
type Overlay struct { //nolint:govet
	Image   string         `yaml:"image"`
	Name    string         `yaml:"name"`
	Options map[string]any `yaml:"options,omitempty"`
}

// InvalidErrorTag is a tag for invalid schematic errors.
type InvalidErrorTag struct{}

// Unmarshal the schematic from text representation.
func Unmarshal(data []byte) (*Schematic, error) {
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)

	var cfg Schematic

	if err := dec.Decode(&cfg); err != nil {
		return nil, xerrors.NewTagged[InvalidErrorTag](err)
	}

	return &cfg, nil
}

// Marshal the schematic to text representation.
//
// Marshal result should be stable if new schematic fields are added.
func (cfg *Schematic) Marshal() ([]byte, error) {
	return yaml.Marshal(cfg)
}

// ID returns the identifier of the schematic.
//
// ID is stable (does not change if the schematic is same).
// ID matches sha256 hash of the canonical representation of the schematic.
func (cfg *Schematic) ID() (string, error) {
	data, err := cfg.Marshal()
	if err != nil {
		return "", err
	}

	binaryID := sha256.Sum256(data)

	return hex.EncodeToString(binaryID[:]), nil
}
