// Package session saves the desktop layout to disk so a restarted server
// finds its windows back: geometry, workspaces, tiling and focus.
package session

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/dedomorozoff/tuide/internal/geom"
	"github.com/dedomorozoff/tuide/internal/sys"
)

// FileName is the session file inside the config directory.
const FileName = "session.json"

// Window is one saved window.
type Window struct {
	ID  string `json:"id"`
	App string `json:"app"`
	// Rect is where the window was; tiling recomputes slots on restore.
	Rect      geom.Rect `json:"rect"`
	Workspace int       `json:"workspace"`
	// State is wm.State as a number: normal, maximized or minimized.
	State int `json:"state"`
}

// Snapshot is the whole desktop at one moment.
type Snapshot struct {
	Layout  int      `json:"layout"`
	Active  int      `json:"active"`
	Focused string   `json:"focused,omitempty"`
	Windows []Window `json:"windows"`
}

// Empty reports whether the snapshot holds no windows.
func (s Snapshot) Empty() bool { return len(s.Windows) == 0 }

// Load reads the session file. A missing file is an empty session.
func Load() (Snapshot, error) {
	var snap Snapshot
	path, err := sys.StatePath(FileName)
	if err != nil {
		return snap, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return snap, nil
	}
	if err != nil {
		return snap, fmt.Errorf("session: read %s: %w", path, err)
	}
	if err := json.Unmarshal(data, &snap); err != nil {
		return Snapshot{}, fmt.Errorf("session: parse %s: %w", path, err)
	}
	return snap, nil
}

// Save writes the session file, creating the directory if needed.
func Save(snap Snapshot) error {
	dir, err := sys.ConfigDir()
	if err != nil {
		return err
	}
	if err := sys.EnsureDir(dir); err != nil {
		return err
	}
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return fmt.Errorf("session: encode: %w", err)
	}
	path, err := sys.StatePath(FileName)
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("session: write %s: %w", path, err)
	}
	return nil
}
