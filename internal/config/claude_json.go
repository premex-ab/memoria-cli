// claude_json.go provides merge-safe editing of ~/.claude.json so that
// `memoria init` can add (or update) the MCP server entry without touching
// any other keys Claude Code stores in that file.
package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// McpEntry is the value written under mcpServers["memoria"] in ~/.claude.json.
type McpEntry struct {
	Type          string `json:"type"`
	URL           string `json:"url"`
	HeadersHelper string `json:"headersHelper"`
}

// ClaudeJSONPath returns the canonical path to ~/.claude.json.
// The HOME directory is read from the environment at call time so tests can
// override it with t.Setenv("HOME", tempDir).
func ClaudeJSONPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".claude.json"), nil
}

// WriteMemoriaEntry merges the supplied entry into ~/.claude.json under
// mcpServers["memoria"].
//
// It reads the existing file (if any), parses the top-level object into
// map[string]any, ensures mcpServers is a map, and sets
// mcpServers["memoria"] = entry — preserving all other top-level keys and all
// other servers under mcpServers.
//
// The write is atomic (tempfile + rename) and uses 0600 permissions.
// If the parent directory does not exist it is created with 0755 permissions.
// If the file exists but is not valid JSON, an error is returned rather than
// silently overwriting.
func WriteMemoriaEntry(path string, entry McpEntry) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("claude.json: create parent dir %s: %w", dir, err)
	}

	// Read the existing file, if present.
	var doc map[string]any
	raw, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		// Fresh machine — start with an empty document.
		doc = make(map[string]any)
	case err != nil:
		return fmt.Errorf("claude.json: read %s: %w", path, err)
	default:
		// File exists — parse it. Fail loudly if malformed so we don't
		// silently clobber the user's existing config.
		if err := json.Unmarshal(raw, &doc); err != nil {
			return fmt.Errorf("claude.json: malformed JSON in %s: %w", path, err)
		}
	}

	// Ensure mcpServers is a map[string]any, creating it if absent or null.
	servers, ok := doc["mcpServers"].(map[string]any)
	if !ok {
		// Handles both absence and an explicit null value.
		servers = make(map[string]any)
	}

	servers["memoria"] = entry
	doc["mcpServers"] = servers

	newRaw, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("claude.json: marshal: %w", err)
	}
	newRaw = append(newRaw, '\n')

	// Write atomically via tempfile + rename.
	tmp, err := os.CreateTemp(dir, "claude.json.*.tmp")
	if err != nil {
		return fmt.Errorf("claude.json: create temp file: %w", err)
	}
	tmpName := tmp.Name()

	if _, err := tmp.Write(newRaw); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("claude.json: write temp file: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		os.Remove(tmpName)
		return fmt.Errorf("claude.json: chmod temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("claude.json: close temp file: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		os.Remove(tmpName)
		return fmt.Errorf("claude.json: rename to %s: %w", path, err)
	}
	return nil
}
