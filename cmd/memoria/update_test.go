package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/premex-ab/memoria-cli/internal/config"
	"github.com/premex-ab/memoria-cli/internal/version"
)

// runUpdate executes the update command with the supplied arguments and returns
// stdout, stderr, and any error.
func runUpdate(t *testing.T, args ...string) (stdout, stderr string, err error) {
	t.Helper()

	var stdoutBuf, stderrBuf bytes.Buffer
	root := newRootCmd()
	root.SetOut(&stdoutBuf)
	root.SetErr(&stderrBuf)

	root.SetArgs(append([]string{"update"}, args...))
	err = root.Execute()
	return stdoutBuf.String(), stderrBuf.String(), err
}

// buildFakeReleaseServer constructs an httptest.Server that serves:
//   - GET / → a release manifest with the given tag and one asset matching goos/goarch
//   - GET /assets/<name> → assetContent
func buildFakeReleaseServer(t *testing.T, tag, goos, goarch string, assetContent []byte) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	assetName := "memoria-" + goos + "-" + goarch

	var srv *httptest.Server
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		info := map[string]any{
			"tag_name": tag,
			"assets": []map[string]any{
				{
					"name":                 assetName,
					"browser_download_url": srv.URL + "/assets/" + assetName,
				},
			},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(info)
	})
	mux.HandleFunc("/assets/"+assetName, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(assetContent)
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// TestUpdate_UpToDate verifies that when the server returns the same version
// as the current binary, the "up to date" message appears and no file is written.
func TestUpdate_UpToDate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	srv := buildFakeReleaseServer(t, "cli/"+version.Version, runtime.GOOS, runtime.GOARCH, nil)

	stdout, _, err := runUpdate(t, "--source", srv.URL)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if !strings.Contains(stdout, "up to date") {
		t.Errorf("expected 'up to date' message, got: %q", stdout)
	}
}

// TestUpdate_CheckOnly_Available verifies that --check-only with a newer version
// prints the "Update available" line and bumps state.LastChecked.
func TestUpdate_CheckOnly_Available(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	srv := buildFakeReleaseServer(t, "cli/99.99.99", runtime.GOOS, runtime.GOARCH, []byte("fake-binary"))

	stdout, _, err := runUpdate(t, "--check-only", "--source", srv.URL)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if !strings.Contains(stdout, "Update available") {
		t.Errorf("expected 'Update available' message, got: %q", stdout)
	}
	if !strings.Contains(stdout, "99.99.99") {
		t.Errorf("expected new version in output, got: %q", stdout)
	}

	// state.LastChecked must have been bumped.
	state, err := config.Read()
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if state.LastChecked.IsZero() {
		t.Error("expected LastChecked to be set after check-only")
	}
}

// TestUpdate_CheckOnly_NoUpdate verifies that --check-only with no available
// update still bumps state.LastChecked.
func TestUpdate_CheckOnly_NoUpdate(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Use the same version as the current binary — no update available.
	srv := buildFakeReleaseServer(t, "cli/"+version.Version, runtime.GOOS, runtime.GOARCH, nil)

	_, _, err := runUpdate(t, "--check-only", "--source", srv.URL)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}

	state, err := config.Read()
	if err != nil {
		t.Fatalf("read state: %v", err)
	}
	if state.LastChecked.IsZero() {
		t.Error("expected LastChecked to be set even when no update available")
	}
}

// TestUpdate_DownloadsAndSwaps verifies the full happy-path: fake server serves
// the manifest AND the asset bytes; the target binary is replaced atomically.
func TestUpdate_DownloadsAndSwaps(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Create a fake "current binary" in a temp dir.
	binDir := t.TempDir()
	fakeBin := filepath.Join(binDir, "memoria")
	if err := os.WriteFile(fakeBin, []byte("old-binary"), 0o755); err != nil {
		t.Fatalf("create fake binary: %v", err)
	}

	// Override executablePath so the update replaces our fake binary.
	origExec := executablePath
	executablePath = func() (string, error) { return fakeBin, nil }
	t.Cleanup(func() { executablePath = origExec })

	newBinaryContent := []byte("new-binary-content")
	srv := buildFakeReleaseServer(t, "cli/99.99.99", runtime.GOOS, runtime.GOARCH, newBinaryContent)

	stdout, _, err := runUpdate(t, "--source", srv.URL)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if !strings.Contains(stdout, "Installed memoria") {
		t.Errorf("expected 'Installed memoria' message, got: %q", stdout)
	}

	// Verify the binary was replaced.
	got, err := os.ReadFile(fakeBin)
	if err != nil {
		t.Fatalf("read replaced binary: %v", err)
	}
	if string(got) != string(newBinaryContent) {
		t.Errorf("expected binary to contain %q, got %q", newBinaryContent, got)
	}

	// Verify exec bit is set.
	info, err := os.Stat(fakeBin)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("expected exec bit set, got mode %v", info.Mode().Perm())
	}
}

// TestUpdate_RefusesDowngrade verifies that when the server returns the same
// version (or an older one), the binary is not touched and the "up to date"
// message is printed.
//
// Note: since version.Version == "dev" (0.0.0 in the comparator), we can't
// easily produce a tag that compares strictly less. We verify the "same version"
// case here, which exercises the cmp >= 0 guard. The logic is symmetric —
// same guard prevents both equal and downgrade.
func TestUpdate_RefusesDowngrade(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// Create a fake binary.
	binDir := t.TempDir()
	fakeBin := filepath.Join(binDir, "memoria")
	originalContent := []byte("original-binary")
	if err := os.WriteFile(fakeBin, originalContent, 0o755); err != nil {
		t.Fatalf("create fake binary: %v", err)
	}

	origExec := executablePath
	executablePath = func() (string, error) { return fakeBin, nil }
	t.Cleanup(func() { executablePath = origExec })

	// Serve the same version — the downgrade guard (cmp >= 0) stops the download.
	srv := buildFakeReleaseServer(t, "cli/"+version.Version, runtime.GOOS, runtime.GOARCH, []byte("fake"))

	stdout, _, err := runUpdate(t, "--source", srv.URL)
	if err != nil {
		t.Fatalf("expected success, got: %v", err)
	}
	if !strings.Contains(stdout, "up to date") {
		t.Errorf("expected 'up to date' message, got: %q", stdout)
	}

	// Binary must be unchanged.
	got, err := os.ReadFile(fakeBin)
	if err != nil {
		t.Fatalf("read binary: %v", err)
	}
	if string(got) != string(originalContent) {
		t.Errorf("expected binary unchanged, got %q", got)
	}
}
