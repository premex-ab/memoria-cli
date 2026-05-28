// Package config manages persistent install state for the memoria CLI.
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// State holds metadata about the local Memoria installation.
type State struct {
	InstalledAt time.Time `json:"installed_at"`
	LastUpdated time.Time `json:"last_updated"`
	LastChecked time.Time `json:"last_checked"` // last `update --check-only`
	TokenSource string    `json:"token_source"` // "env", "keychain", "file"
	BoundTenant string    `json:"bound_tenant"` // populated by `memoria init` (Slice 2)
	BoundBrain  string    `json:"bound_brain"`  // same
}

// Path returns the state file location: ~/.config/memoria/state.json.
func Path() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "memoria", "state.json"), nil
}

// Read returns the persisted state, or a zero State if the file doesn't exist.
func Read() (State, error) {
	path, err := Path()
	if err != nil {
		return State{}, err
	}

	raw, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return State{}, nil
	}
	if err != nil {
		return State{}, fmt.Errorf("read state: %w", err)
	}

	var s State
	if err := json.Unmarshal(raw, &s); err != nil {
		return State{}, fmt.Errorf("parse state: %w", err)
	}
	return s, nil
}

// Write atomically writes the state via tempfile + rename. Creates parent
// directories as needed. Uses 0600 permissions.
func Write(s State) error {
	path, err := Path()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	raw, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal state: %w", err)
	}
	raw = append(raw, '\n')

	// Write to a temp file in the same directory so the rename is atomic.
	tmp, err := os.CreateTemp(dir, "state-*.json.tmp")
	if err != nil {
		return fmt.Errorf("create temp state file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(raw); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("write temp state: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("chmod temp state: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("close temp state: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("rename state file: %w", err)
	}
	return nil
}
