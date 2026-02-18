// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package flags

import (
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/knadh/koanf/parsers/dotenv"
	"github.com/knadh/koanf/parsers/json"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env/v2"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// Configs is a pflag.Value implementation for multiple koanf configurations.
type Configs struct {
	raw     []string
	configs []Config
}

// MustNewConfigs creates a new Configs instance with default value, or panics.
func MustNewConfigs(envPrefix string) *Configs {
	c := new(Configs)

	err := c.Set("env=" + envPrefix)
	if err != nil {
		panic(err)
	}

	return c
}

// Config represents a koanf configuration source.
type Config struct {
	Parser   koanf.Parser
	Provider koanf.Provider
}

// String implements pflag.Value interface.
func (c *Configs) String() string {
	return strings.Join(c.raw, ",")
}

// Set implements pflag.Value interface.
func (c *Configs) Set(value string) error {
	if strings.TrimSpace(value) == "" {
		return errors.New("configs value must not be empty")
	}

	raw := strings.Split(value, ",")

	cfgs, err := parseConfigsFlag(raw)
	if err != nil {
		return err
	}

	c.raw = append(c.raw, raw...)
	c.configs = append(c.configs, cfgs...)

	return nil
}

// Type implements pflag.Value interface.
func (c *Configs) Type() string {
	return "configs"
}

// Value returns the underlying configuration list.
func (c *Configs) Value() []Config {
	return c.configs
}

func parseConfigsFlag(values []string) ([]Config, error) {
	var configs []Config

	for _, v := range values {
		providerType, providerConfig := splitProvider(v)

		// TODO: add support for special providers that need extra params,
		// e.g. vault, consul, etcd
		switch providerType {
		case "file":
			cfg, err := newFileProvider(providerConfig)
			if err != nil {
				return nil, err
			}

			configs = append(configs, cfg)

		case "env":
			configs = append(configs, newEnvConfig(providerConfig))

		default:
			return nil, fmt.Errorf("unsupported config provider: %s", providerType)
		}
	}

	return configs, nil
}

func splitProvider(s string) (string, string) {
	if strings.Contains(s, "=") {
		parts := strings.SplitN(s, "=", 2)

		return parts[0], parts[1]
	}

	return "file", s
}

func newEnvConfig(prefix string) Config {
	return Config{
		Provider: env.Provider(".", env.Opt{Prefix: prefix}),
	}
}

func newFileProvider(filename string) (Config, error) {
	ext := strings.ToLower(filepath.Ext(filename))

	var parser koanf.Parser

	switch ext {
	case ".json":
		parser = json.Parser()
	case ".yaml", ".yml":
		parser = yaml.Parser()
	case ".env":
		parser = dotenv.Parser()
	default:
		return Config{}, fmt.Errorf("unsupported config format: %s", ext)
	}

	return Config{
		Parser:   parser,
		Provider: file.Provider(filename),
	}, nil
}
