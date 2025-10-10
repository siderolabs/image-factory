// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

type aliasConfig struct {
	ExtensionNameAlias []struct {
		Name  string `yaml:"name"`
		Alias string `yaml:"alias"`
	} `yaml:"extensionNameAlias"`
}

var aliasMap map[string]string

// Loads aliases from aliases.yaml.
func LoadAliases(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("failed to read alias config: %w", err)
	}

	var cfg aliasConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to unmarshal alias config: %w", err)
	}

	aliasMap = make(map[string]string)
	for _, item := range cfg.ExtensionNameAlias {
		aliasMap[item.Name] = item.Alias
	}

	return nil
}

// GetAlias returns alias if exists.
func GetAlias(name string) (string, bool) {
	if aliasMap == nil {
		return "", false
	}
	alias, ok := aliasMap[name]
	return alias, ok
}

