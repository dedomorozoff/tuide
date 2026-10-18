package wire

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/dedomorozoff/tuide/internal/sys"
)

// StateFile is the name of the file holding the running server's address.
const StateFile = "server.json"

// State is where a running server listens and how a client proves it may join.
type State struct {
	Pid   int    `json:"pid"`
	Addr  string `json:"addr"`
	Token string `json:"token"`
}

// WriteState publishes the server state so clients can find it.
func WriteState(state State) error {
	dir, err := sys.ConfigDir()
	if err != nil {
		return err
	}
	if err := sys.EnsureDir(dir); err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("wire: encode state: %w", err)
	}
	path := filepath.Join(dir, StateFile)
	if err := os.WriteFile(path, append(data, '\n'), 0o600); err != nil {
		return fmt.Errorf("wire: write %s: %w", path, err)
	}
	return nil
}

// ReadState loads the server state. No file means no server: (nil, nil).
func ReadState() (*State, error) {
	path, err := sys.StatePath(StateFile)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("wire: read %s: %w", path, err)
	}
	var state State
	if err := json.Unmarshal(data, &state); err != nil {
		return nil, fmt.Errorf("wire: parse %s: %w", path, err)
	}
	return &state, nil
}

// RemoveState deletes the state file, ignoring one that is already gone.
func RemoveState() error {
	path, err := sys.StatePath(StateFile)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("wire: remove %s: %w", path, err)
	}
	return nil
}
