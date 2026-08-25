// Package config manages bb's configuration (config.yml) and stored
// credentials (hosts.yml) under $XDG_CONFIG_HOME/bb (or ~/.config/bb).
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"

	"gopkg.in/yaml.v3"
)

// DefaultHost is the only Bitbucket host supported (Bitbucket Cloud).
const DefaultHost = "bitbucket.org"

// Known configuration keys and their defaults.
var defaults = map[string]string{
	"workspace":      "",
	"git_protocol":   "https",
	"editor":         "",
	"pager":          "",
	"prompt":         "enabled",
	"merge_strategy": "merge_commit",
	"browser":        "",
}

// ValidateKey returns an error if key is not a known configuration key.
func ValidateKey(key string) error {
	if _, ok := defaults[key]; !ok {
		return fmt.Errorf("invalid key %q", key)
	}
	return nil
}

// ValidateValue checks well-known enumerated values.
func ValidateValue(key, value string) error {
	switch key {
	case "git_protocol":
		if value != "https" && value != "ssh" {
			return fmt.Errorf("invalid value %q for git_protocol (https|ssh)", value)
		}
	case "prompt":
		if value != "enabled" && value != "disabled" {
			return fmt.Errorf("invalid value %q for prompt (enabled|disabled)", value)
		}
	case "merge_strategy":
		switch value {
		case "merge_commit", "squash", "fast_forward", "squash_fast_forward", "rebase_fast_forward", "rebase_merge":
		default:
			return fmt.Errorf("invalid value %q for merge_strategy", value)
		}
	}
	return nil
}

// Config is the on-disk configuration plus a credential store.
type Config struct {
	mu     sync.Mutex
	dir    string
	values map[string]string
	store  CredentialStore
}

// Dir returns the configuration directory, honoring BB_CONFIG_DIR and
// XDG_CONFIG_HOME.
func Dir() string {
	if d := os.Getenv("BB_CONFIG_DIR"); d != "" {
		return d
	}
	if x := os.Getenv("XDG_CONFIG_HOME"); x != "" {
		return filepath.Join(x, "bb")
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(".", ".bb")
	}
	return filepath.Join(home, ".config", "bb")
}

// Load reads config.yml from Dir(). A missing file yields defaults.
func Load() (*Config, error) {
	return LoadFrom(Dir())
}

// LoadFrom reads configuration from an explicit directory.
func LoadFrom(dir string) (*Config, error) {
	c := &Config{dir: dir, values: map[string]string{}}
	data, err := os.ReadFile(filepath.Join(dir, "config.yml"))
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	if len(data) > 0 {
		var raw map[string]string
		if err := yaml.Unmarshal(data, &raw); err != nil {
			return nil, fmt.Errorf("parsing config.yml: %w", err)
		}
		for k, v := range raw {
			c.values[k] = v
		}
	}
	c.store = NewFileCredentialStore(dir)
	return c, nil
}

// Get returns the value for key, falling back to the default.
func (c *Config) Get(key string) (string, error) {
	if err := ValidateKey(key); err != nil {
		return "", err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if v, ok := c.values[key]; ok && v != "" {
		return v, nil
	}
	return defaults[key], nil
}

// Set stores a value in memory; call Write to persist.
func (c *Config) Set(key, value string) error {
	if err := ValidateKey(key); err != nil {
		return err
	}
	if err := ValidateValue(key, value); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.values[key] = value
	return nil
}

// Keys returns all known keys, sorted.
func (c *Config) Keys() []string {
	keys := make([]string, 0, len(defaults))
	for k := range defaults {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// Write persists config.yml atomically.
func (c *Config) Write() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	out := map[string]string{}
	for k, v := range c.values {
		if v != "" {
			out[k] = v
		}
	}
	data, err := yaml.Marshal(out)
	if err != nil {
		return err
	}
	return writeFileAtomic(filepath.Join(c.dir, "config.yml"), data, 0o600)
}

// Credentials returns the credential store.
func (c *Config) Credentials() CredentialStore { return c.store }
