package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// memoriaEntry is the test fixture entry written in each test.
var memoriaEntry = McpEntry{
	Type:          "http",
	URL:           "https://api.memoria.premex.se/mcp",
	HeadersHelper: "memoria headers",
}

// readJSON reads path and unmarshals into a map.
func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var m map[string]any
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("unmarshal %s: %v", path, err)
	}
	return m
}

func TestWriteMemoriaEntry_MissingFile_CreatesFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude.json")

	if err := WriteMemoriaEntry(path, memoriaEntry); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	doc := readJSON(t, path)
	servers, ok := doc["mcpServers"].(map[string]any)
	if !ok {
		t.Fatalf("expected mcpServers to be a map, got %T", doc["mcpServers"])
	}
	mem, ok := servers["memoria"]
	if !ok {
		t.Fatal("expected mcpServers.memoria to be set")
	}
	memMap, ok := mem.(map[string]any)
	if !ok {
		t.Fatalf("expected memoria entry to be a map, got %T", mem)
	}
	if memMap["type"] != "http" {
		t.Errorf("expected type=http, got %v", memMap["type"])
	}
	if memMap["url"] != "https://api.memoria.premex.se/mcp" {
		t.Errorf("unexpected url: %v", memMap["url"])
	}
	if memMap["headersHelper"] != "memoria headers" {
		t.Errorf("unexpected headersHelper: %v", memMap["headersHelper"])
	}
}

func TestWriteMemoriaEntry_ExistingFileOtherKeys_Preserved(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude.json")

	// Write an existing file with some other top-level keys.
	existing := map[string]any{
		"numStartups": 42,
		"projects":    []any{"projA", "projB"},
	}
	raw, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := WriteMemoriaEntry(path, memoriaEntry); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	doc := readJSON(t, path)

	// Other keys must be preserved.
	if v, _ := doc["numStartups"].(float64); int(v) != 42 {
		t.Errorf("expected numStartups=42, got %v", doc["numStartups"])
	}
	projects, ok := doc["projects"].([]any)
	if !ok || len(projects) != 2 {
		t.Errorf("expected projects to be preserved, got %v", doc["projects"])
	}

	// memoria entry must exist.
	servers, ok := doc["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal("expected mcpServers to be a map")
	}
	if _, ok := servers["memoria"]; !ok {
		t.Fatal("expected mcpServers.memoria to be present")
	}
}

func TestWriteMemoriaEntry_ExistingMcpServers_BothServersPresent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude.json")

	// Write an existing file that already has another MCP server.
	existing := map[string]any{
		"mcpServers": map[string]any{
			"other-tool": map[string]any{
				"type": "http",
				"url":  "https://other.example.com/mcp",
			},
		},
	}
	raw, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	if err := WriteMemoriaEntry(path, memoriaEntry); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	doc := readJSON(t, path)
	servers, ok := doc["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal("expected mcpServers to be a map")
	}

	// Both servers must be present.
	if _, ok := servers["other-tool"]; !ok {
		t.Error("expected mcpServers.other-tool to be preserved")
	}
	if _, ok := servers["memoria"]; !ok {
		t.Error("expected mcpServers.memoria to be present")
	}
}

func TestWriteMemoriaEntry_ExistingEntry_Overwritten(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude.json")

	// Write an existing memoria entry with old values.
	existing := map[string]any{
		"mcpServers": map[string]any{
			"memoria": map[string]any{
				"type":          "http",
				"url":           "https://old.example.com/mcp",
				"headersHelper": "old headers",
			},
		},
	}
	raw, _ := json.MarshalIndent(existing, "", "  ")
	if err := os.WriteFile(path, append(raw, '\n'), 0o600); err != nil {
		t.Fatal(err)
	}

	newEntry := McpEntry{
		Type:          "http",
		URL:           "https://new.example.com/mcp",
		HeadersHelper: "nueva headers",
	}
	if err := WriteMemoriaEntry(path, newEntry); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	doc := readJSON(t, path)
	servers := doc["mcpServers"].(map[string]any)
	mem := servers["memoria"].(map[string]any)

	if mem["url"] != "https://new.example.com/mcp" {
		t.Errorf("expected new url, got %v", mem["url"])
	}
	if mem["headersHelper"] != "nueva headers" {
		t.Errorf("expected new headersHelper, got %v", mem["headersHelper"])
	}
}

func TestWriteMemoriaEntry_MalformedJSON_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude.json")

	// Write malformed JSON.
	if err := os.WriteFile(path, []byte("not json at all {{{"), 0o600); err != nil {
		t.Fatal(err)
	}

	err := WriteMemoriaEntry(path, memoriaEntry)
	if err == nil {
		t.Fatal("expected error for malformed JSON, got nil")
	}
}

func TestWriteMemoriaEntry_FilePerms(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, ".claude.json")

	if err := WriteMemoriaEntry(path, memoriaEntry); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if mode := info.Mode().Perm(); mode != 0o600 {
		t.Errorf("expected 0600 permissions, got %04o", mode)
	}
}
