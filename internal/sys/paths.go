// Package sys is the only place allowed to know about the host platform.
package sys

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var errNoBase = errors.New("sys: no usable base directory")

// ConfigDir returns the directory holding user configuration.
func ConfigDir() (string, error) {
	return resolve(os.Getenv("TUIDE_CONFIG_DIR"), configBase(), "tuide")
}

// DataDir returns the directory holding user data.
func DataDir() (string, error) {
	return resolve(os.Getenv("TUIDE_DATA_DIR"), dataBase(), "tuide")
}

// CacheDir returns the directory holding regenerable caches.
func CacheDir() (string, error) {
	return resolve(os.Getenv("TUIDE_CACHE_DIR"), cacheBase(), "tuide")
}

// StatePath returns the full path of a file inside the config directory.
func StatePath(name string) (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

// EnsureDir creates dir and every missing parent.
func EnsureDir(dir string) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("sys: create dir %s: %w", dir, err)
	}
	return nil
}

func resolve(override, base, name string) (string, error) {
	if override != "" {
		return filepath.Clean(override), nil
	}
	if base == "" {
		return "", fmt.Errorf("sys: %s: %w", name, errNoBase)
	}
	return filepath.Join(base, name), nil
}
