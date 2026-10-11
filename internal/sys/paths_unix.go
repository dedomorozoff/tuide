//go:build !windows

package sys

import (
	"os"
	"path/filepath"
)

func homeDir() string {
	return os.Getenv("HOME")
}

func configBase() string {
	return firstNonEmpty(os.Getenv("XDG_CONFIG_HOME"), filepath.Join(homeDir(), ".config"))
}

func dataBase() string {
	return firstNonEmpty(os.Getenv("XDG_DATA_HOME"), filepath.Join(homeDir(), ".local", "share"))
}

func cacheBase() string {
	return firstNonEmpty(os.Getenv("XDG_CACHE_HOME"), filepath.Join(homeDir(), ".cache"))
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
