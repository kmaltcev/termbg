// Package config loads termbg's configuration file, a TOML document
// describing which background source(s) and terminal adapter(s) are
// active and how they're configured.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config is the root of ~/.config/termbg/config.toml.
type Config struct {
	// Source is the name of the active background source, e.g.
	// "local" or "wallhaven". Must match a key in SourceConfig.
	Source string `toml:"source"`

	// Terminal is the name of the active terminal adapter, e.g.
	// "ghostty". Must match a key in TerminalConfig.
	Terminal string `toml:"terminal"`

	// Schedule is an optional cron expression (e.g. "@every 30m" or
	// "0 9,21 * * *") controlling automatic rotation. Empty disables
	// scheduled rotation; "termbg next" still works on request.
	Schedule string `toml:"schedule"`

	// SourceConfig holds one raw config section per source name,
	// e.g. SourceConfig["local"]["dir"].
	SourceConfig map[string]map[string]any `toml:"source_config"`

	// TerminalConfig holds one raw config section per terminal
	// adapter name, e.g. TerminalConfig["ghostty"]["config_path"].
	TerminalConfig map[string]map[string]any `toml:"terminal_config"`
}

// DefaultPath returns ~/.config/termbg/config.toml, honoring
// $XDG_CONFIG_HOME.
func DefaultPath() (string, error) {
	dir := os.Getenv("XDG_CONFIG_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("config: determining home dir: %w", err)
		}
		dir = filepath.Join(home, ".config")
	}
	return filepath.Join(dir, "termbg", "config.toml"), nil
}

// Load reads and parses the config file at path.
func Load(path string) (*Config, error) {
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, fmt.Errorf("config: loading %s: %w", path, err)
	}
	if cfg.SourceConfig == nil {
		cfg.SourceConfig = map[string]map[string]any{}
	}
	if cfg.TerminalConfig == nil {
		cfg.TerminalConfig = map[string]map[string]any{}
	}
	return &cfg, nil
}

// Save writes cfg to path as TOML, creating parent directories as
// needed.
func Save(path string, cfg *Config) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("config: creating config dir: %w", err)
	}
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("config: creating %s: %w", path, err)
	}
	defer f.Close()
	if err := toml.NewEncoder(f).Encode(cfg); err != nil {
		return fmt.Errorf("config: encoding %s: %w", path, err)
	}
	return nil
}
