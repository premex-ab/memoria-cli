package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/premex-ab/memoria-cli/internal/config"
	"github.com/premex-ab/memoria-cli/internal/skill"
)

// runStatus executes the status command with the supplied arguments and returns
// stdout, stderr, and any error.
func runStatus(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	var stdoutBuf, stderrBuf bytes.Buffer
	root := newRootCmd()
	root.SetOut(&stdoutBuf)
	root.SetErr(&stderrBuf)

	root.SetArgs(append([]string{"status"}, args...))
	err = root.Execute()
	return stdoutBuf.String(), stderrBuf.String(), err
}

// TestStatus_NoToken verifies that when no token is configured, status prints
// the unconfigured message plus the CLI line and returns exit 0 (informational).
func TestStatus_NoToken(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// Ensure no env var token is present.
	t.Setenv("MEMORIA_API_KEY", "")

	stdout, _, err := runStatus(t)
	if err != nil {
		t.Fatalf("expected exit 0 for no-token (informational), got: %v", err)
	}

	if !strings.Contains(stdout, "not configured") {
		t.Errorf("expected 'not configured' in stdout, got: %q", stdout)
	}
	if !strings.Contains(stdout, "memoria init") {
		t.Errorf("expected 'memoria init' suggestion in stdout, got: %q", stdout)
	}
	// CLI line must still appear.
	if !strings.Contains(stdout, "CLI:") {
		t.Errorf("expected CLI version line in stdout, got: %q", stdout)
	}
}

// TestStatus_HappyPath verifies the full output when a token is set, /v1/whoami
// returns 200, ~/.claude.json has the memoria entry, and the skill exists.
// The Skill line must show the version from the installed SKILL.md, and the MCP
// line must show the URL stored in ~/.claude.json, not the --api-url flag.
func TestStatus_HappyPath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("MEMORIA_API_KEY", "mem_live_statustest")

	// Spin up a fake whoami server.
	srv := buildFakeWhoamiServer(t, "mem_live_statustest", "tenant1", "brain1",
		[]string{"memory:read", "memory:write"})
	defer srv.Close()

	// Write ~/.claude.json with a memoria entry pointing at the fake server.
	mcpURL := srv.URL + "/mcp"
	claudeJSON := map[string]any{
		"mcpServers": map[string]any{
			"memoria": map[string]any{
				"type":          "http",
				"url":           mcpURL,
				"headersHelper": "memoria headers",
			},
		},
	}
	claudeBytes, _ := json.MarshalIndent(claudeJSON, "", "  ")
	claudePath := filepath.Join(home, ".claude.json")
	if err := os.WriteFile(claudePath, claudeBytes, 0o600); err != nil {
		t.Fatalf("write ~/.claude.json: %v", err)
	}

	// Install the skill (embed the real embedded content so the version is
	// readable from frontmatter).
	skillDir := filepath.Join(home, ".claude", "skills", "memoria")
	if err := os.MkdirAll(skillDir, 0o700); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), skill.EmbeddedSKILL, 0o600); err != nil {
		t.Fatalf("write SKILL.md: %v", err)
	}

	// Determine expected installed version from the embedded content.
	embVer, err := skill.EmbeddedVersion()
	if err != nil {
		t.Fatalf("EmbeddedVersion: %v", err)
	}

	stdout, _, err := runStatus(t, "--api-url", srv.URL)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	// Token line.
	if !strings.Contains(stdout, "Token:") {
		t.Errorf("expected 'Token:' line, got: %q", stdout)
	}
	// Brain line.
	if !strings.Contains(stdout, "tenant1/brain1") {
		t.Errorf("expected 'tenant1/brain1' in brain line, got: %q", stdout)
	}
	if !strings.Contains(stdout, "memory:read") {
		t.Errorf("expected scopes in output, got: %q", stdout)
	}
	// MCP line must show the URL from ~/.claude.json, not the --api-url flag.
	if !strings.Contains(stdout, "MCP:") {
		t.Errorf("expected MCP line, got: %q", stdout)
	}
	if !strings.Contains(stdout, mcpURL) {
		t.Errorf("expected MCP line to contain %q (from ~/.claude.json), got: %q", mcpURL, stdout)
	}
	// Skill line must show the version read from the installed file.
	if !strings.Contains(stdout, "Skill:") {
		t.Errorf("expected Skill line, got: %q", stdout)
	}
	if !strings.Contains(stdout, embVer) {
		t.Errorf("expected Skill line to contain version %q, got: %q", embVer, stdout)
	}
	// CLI line.
	if !strings.Contains(stdout, "CLI:") {
		t.Errorf("expected CLI line, got: %q", stdout)
	}
}

// TestStatus_StaleSkill verifies that when the installed SKILL.md carries an
// older version than the binary embeds, the status line hints to re-run init.
func TestStatus_StaleSkill(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("MEMORIA_API_KEY", "mem_live_staleskill")

	srv := buildFakeWhoamiServer(t, "mem_live_staleskill", "t", "b",
		[]string{"memory:read"})
	defer srv.Close()

	// Write ~/.claude.json.
	claudeJSON := map[string]any{
		"mcpServers": map[string]any{
			"memoria": map[string]any{
				"type": "http",
				"url":  srv.URL + "/mcp",
			},
		},
	}
	claudeBytes, _ := json.MarshalIndent(claudeJSON, "", "  ")
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), claudeBytes, 0o600); err != nil {
		t.Fatalf("write ~/.claude.json: %v", err)
	}

	// Write a SKILL.md with an older version than what the binary embeds (0.1.0).
	skillDir := filepath.Join(home, ".claude", "skills", "memoria")
	if err := os.MkdirAll(skillDir, 0o700); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	staleContent := strings.Replace(string(skill.EmbeddedSKILL), "version: 0.1.0", "version: 0.0.1", 1)
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(staleContent), 0o600); err != nil {
		t.Fatalf("write stale SKILL.md: %v", err)
	}

	stdout, _, err := runStatus(t, "--api-url", srv.URL)
	if err != nil {
		t.Fatalf("expected exit 0, got: %v", err)
	}

	// Must mention both the installed version and the binary-embedded version.
	if !strings.Contains(stdout, "installed 0.0.1") {
		t.Errorf("expected 'installed 0.0.1' in Skill line, got: %q", stdout)
	}
	if !strings.Contains(stdout, "binary embeds 0.1.0") {
		t.Errorf("expected 'binary embeds 0.1.0' in Skill line, got: %q", stdout)
	}
	if !strings.Contains(stdout, "memoria init") {
		t.Errorf("expected 'memoria init' refresh hint in Skill line, got: %q", stdout)
	}
}

// TestStatus_TokenRevoked verifies that a 401 from /v1/whoami prints the
// "unable to resolve" message but the command still exits 0.
func TestStatus_TokenRevoked(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("MEMORIA_API_KEY", "mem_live_revoked")

	// Fake server always returns 401.
	srv := fake401Server(t)
	defer srv.Close()

	// Write minimal ~/.claude.json so MCP check passes.
	claudeJSON := map[string]any{
		"mcpServers": map[string]any{
			"memoria": map[string]any{
				"type": "http",
				"url":  srv.URL + "/mcp",
			},
		},
	}
	claudeBytes, _ := json.MarshalIndent(claudeJSON, "", "  ")
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), claudeBytes, 0o600); err != nil {
		t.Fatalf("write ~/.claude.json: %v", err)
	}

	stdout, _, err := runStatus(t, "--api-url", srv.URL)
	if err != nil {
		t.Fatalf("expected exit 0 even with revoked token, got: %v", err)
	}

	if !strings.Contains(stdout, "unable to resolve") {
		t.Errorf("expected 'unable to resolve' in output, got: %q", stdout)
	}
	if !strings.Contains(stdout, "revoked") {
		t.Errorf("expected 'revoked' in output, got: %q", stdout)
	}
}

// TestStatus_MissingMcpEntry verifies that when ~/.claude.json has no
// memoria server the "NOT configured" line appears.
func TestStatus_MissingMcpEntry(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("MEMORIA_API_KEY", "mem_live_statustest2")

	// Spin up a whoami server.
	srv := buildFakeWhoamiServer(t, "mem_live_statustest2", "t", "b",
		[]string{"memory:read"})
	defer srv.Close()

	// ~/.claude.json exists but has no memoria server.
	claudeJSON := map[string]any{
		"mcpServers": map[string]any{
			"other-server": map[string]any{"type": "http", "url": "https://example.com"},
		},
	}
	claudeBytes, _ := json.MarshalIndent(claudeJSON, "", "  ")
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), claudeBytes, 0o600); err != nil {
		t.Fatalf("write ~/.claude.json: %v", err)
	}

	stdout, _, err := runStatus(t, "--api-url", srv.URL)
	if err != nil {
		t.Fatalf("expected exit 0, got: %v", err)
	}

	if !strings.Contains(stdout, "NOT configured") {
		t.Errorf("expected 'NOT configured' in output, got: %q", stdout)
	}
}

// TestStatus_MissingSkill verifies that when the skill file is absent the
// "NOT installed" line appears.
func TestStatus_MissingSkill(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("MEMORIA_API_KEY", "mem_live_statustest3")

	// Spin up a whoami server.
	srv := buildFakeWhoamiServer(t, "mem_live_statustest3", "t", "b",
		[]string{"memory:read"})
	defer srv.Close()

	// Write ~/.claude.json with memoria entry.
	claudeJSON := map[string]any{
		"mcpServers": map[string]any{
			"memoria": map[string]any{
				"type": "http",
				"url":  srv.URL + "/mcp",
			},
		},
	}
	claudeBytes, _ := json.MarshalIndent(claudeJSON, "", "  ")
	if err := os.WriteFile(filepath.Join(home, ".claude.json"), claudeBytes, 0o600); err != nil {
		t.Fatalf("write ~/.claude.json: %v", err)
	}
	// Skill file deliberately NOT created.

	stdout, _, err := runStatus(t, "--api-url", srv.URL)
	if err != nil {
		t.Fatalf("expected exit 0, got: %v", err)
	}

	if !strings.Contains(stdout, "NOT installed") {
		t.Errorf("expected 'NOT installed' in output, got: %q", stdout)
	}
}

// TestStatus_LastCheckedRelativeTime verifies that when LastChecked is set in
// state.json, the CLI line shows a relative time rather than "never".
func TestStatus_LastCheckedRelativeTime(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	// No MEMORIA_API_KEY — we want the no-token path which still prints CLI line.
	t.Setenv("MEMORIA_API_KEY", "")

	// Write a state file with LastChecked set 5 minutes ago.
	configDir := filepath.Join(home, ".config", "memoria")
	if err := os.MkdirAll(configDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	state := config.State{
		LastChecked: time.Now().UTC().Add(-5 * time.Minute),
	}
	if err := config.Write(state); err != nil {
		t.Fatalf("write state: %v", err)
	}

	stdout, _, _ := runStatus(t)
	if !strings.Contains(stdout, "minutes ago") {
		t.Errorf("expected 'minutes ago' in CLI line, got: %q", stdout)
	}
}
