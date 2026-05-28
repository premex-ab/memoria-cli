package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fakeWhoamiServer returns a test server that always replies 200 with the
// given identity JSON.
func fakeWhoamiServer(t *testing.T, tenantID, brainID string, scopes []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/whoami" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"tenantId": tenantID,
			"brainId":  brainID,
			"scopes":   scopes,
			"kind":     "live",
		})
	}))
}

// fake401Server returns a test server that always replies 401.
func fake401Server(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"code":"invalid_key","message":"unknown or revoked api key"}}`))
	}))
}

// runInit executes the init command with the supplied arguments and returns
// stdout, stderr, and any error.
func runInit(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	var stdoutBuf, stderrBuf bytes.Buffer
	root := newRootCmd()
	root.SetOut(&stdoutBuf)
	root.SetErr(&stderrBuf)

	root.SetArgs(append([]string{"init"}, args...))
	err = root.Execute()
	return stdoutBuf.String(), stderrBuf.String(), err
}

func TestInit_Success(t *testing.T) {
	// Point HOME at a temp dir so all file writes are isolated.
	home := t.TempDir()
	t.Setenv("HOME", home)
	// Clear any token env var so the file backend is used.
	t.Setenv("MEMORIA_API_KEY", "")

	srv := fakeWhoamiServer(t, "t", "b", []string{"memory:read"})
	defer srv.Close()

	stdout, _, err := runInit(t, "mem_live_testtoken", "--api-url", srv.URL)
	if err != nil {
		t.Fatalf("expected success, got error: %v", err)
	}

	// Stdout must confirm the bound identity.
	if !strings.Contains(stdout, "Bound to tenant=t") {
		t.Errorf("expected stdout to contain 'Bound to tenant=t', got: %q", stdout)
	}
	if !strings.Contains(stdout, "brain=b") {
		t.Errorf("expected stdout to contain 'brain=b', got: %q", stdout)
	}

	// ~/.claude.json must have the memoria MCP entry.
	claudePath := filepath.Join(home, ".claude.json")
	raw, err := os.ReadFile(claudePath)
	if err != nil {
		t.Fatalf("expected ~/.claude.json to exist, got: %v", err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("expected valid JSON in ~/.claude.json, got: %v", err)
	}
	servers, ok := doc["mcpServers"].(map[string]any)
	if !ok {
		t.Fatal("expected mcpServers to be a map")
	}
	mem, ok := servers["memoria"]
	if !ok {
		t.Fatal("expected mcpServers.memoria to be present")
	}
	memMap, ok := mem.(map[string]any)
	if !ok {
		t.Fatalf("expected memoria entry to be a map, got %T", mem)
	}
	expectedURL := srv.URL + "/mcp"
	if memMap["url"] != expectedURL {
		t.Errorf("expected url=%s, got %v", expectedURL, memMap["url"])
	}

	// ~/.config/memoria/state.json must have BoundTenant and BoundBrain.
	statePath := filepath.Join(home, ".config", "memoria", "state.json")
	stateRaw, err := os.ReadFile(statePath)
	if err != nil {
		t.Fatalf("expected state.json to exist, got: %v", err)
	}
	var state map[string]any
	if err := json.Unmarshal(stateRaw, &state); err != nil {
		t.Fatalf("expected valid JSON in state.json, got: %v", err)
	}
	if state["bound_tenant"] != "t" {
		t.Errorf("expected bound_tenant=t, got %v", state["bound_tenant"])
	}
	if state["bound_brain"] != "b" {
		t.Errorf("expected bound_brain=b, got %v", state["bound_brain"])
	}
}

func TestInit_InvalidToken_NonZeroExitAndStderr(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("MEMORIA_API_KEY", "")

	srv := fake401Server(t)
	defer srv.Close()

	_, stderr, err := runInit(t, "mem_live_badtoken", "--api-url", srv.URL)
	if err == nil {
		t.Fatal("expected non-zero exit for 401, got nil error")
	}

	// Stderr must contain a useful message.
	if !strings.Contains(stderr, "token rejected") {
		t.Errorf("expected stderr to mention 'token rejected', got: %q", stderr)
	}

	// ~/.claude.json must NOT be written.
	claudePath := filepath.Join(home, ".claude.json")
	if _, err := os.Stat(claudePath); !os.IsNotExist(err) {
		t.Errorf("expected ~/.claude.json to NOT exist after 401, but it does")
	}
}
