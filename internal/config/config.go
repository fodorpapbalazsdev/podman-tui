// Package config handles loading the podman-tui configuration file.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Preset is a named podman command the user can launch from the presets dialog.
// Command holds everything after "podman", e.g. "run -d -p 80:80 --name nginx nginx:latest".
type Preset struct {
	Name    string `json:"name"`
	Command string `json:"command"`
}

// Config is the top-level configuration struct.
type Config struct {
	Presets []Preset `json:"presets"`
}

// DefaultPath returns the default config file location:
// ~/.config/podman-tui/presets.json
func DefaultPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "podman-tui", "presets.json")
}

// Load reads and parses the config file at path.
// Returns an empty Config (no error) when the file does not exist.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{}, nil
		}
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	return &cfg, nil
}
