package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/dedomorozoff/tuide/internal/sys"
	"github.com/dedomorozoff/tuide/internal/theme"
)

// FileName is the config file inside the config directory.
const FileName = "config.json"

// Config is everything the user can change without recompiling.
type Config struct {
	Theme   string   `json:"theme"`
	Startup []string `json:"startup"`
	Taskbar bool     `json:"taskbar"`
	Mouse   bool     `json:"mouse"`
}

// Default returns the configuration used when no file exists.
func Default() Config {
	return Config{
		Theme:   theme.DefaultName,
		Startup: nil,
		Taskbar: true,
		Mouse:   true,
	}
}

// Load reads the config file, falling back to defaults when it is missing.
// A malformed file is an error: silently ignoring it hides typos.
func Load() (Config, error) {
	cfg := Default()

	path, err := sys.StatePath(FileName)
	if err != nil {
		return cfg, err
	}

	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return cfg, fmt.Errorf("config: read %s: %w", path, err)
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return Default(), fmt.Errorf("config: parse %s: %w", path, err)
	}
	if cfg.Theme == "" {
		cfg.Theme = theme.DefaultName
	}
	return cfg, nil
}

// Save writes the config file, creating the directory if needed.
func (c Config) Save() error {
	dir, err := sys.ConfigDir()
	if err != nil {
		return err
	}
	if err := sys.EnsureDir(dir); err != nil {
		return err
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("config: encode: %w", err)
	}

	path, err := sys.StatePath(FileName)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("config: write %s: %w", path, err)
	}
	return nil
}
