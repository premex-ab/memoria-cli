package auth

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// -- helpers -------------------------------------------------------------------

// isolateEnv clears MEMORIA_API_KEY for the duration of the test and restores
// the original value on cleanup.
func isolateEnv(t *testing.T) {
	t.Helper()
	prev, had := os.LookupEnv("MEMORIA_API_KEY")
	os.Unsetenv("MEMORIA_API_KEY")
	t.Cleanup(func() {
		if had {
			os.Setenv("MEMORIA_API_KEY", prev)
		} else {
			os.Unsetenv("MEMORIA_API_KEY")
		}
	})
}

// overrideHome redirects all home-dir lookups to a temp directory for the
// duration of the test by setting $HOME. Returns the temp dir path.
func overrideHome(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	prev := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	t.Cleanup(func() { os.Setenv("HOME", prev) })
	return tmp
}

// -- env tests -----------------------------------------------------------------

func TestResolveEnv_Basic(t *testing.T) {
	os.Setenv("MEMORIA_API_KEY", "mem_live_from_env")
	t.Cleanup(func() { os.Unsetenv("MEMORIA_API_KEY") })

	tok, src, err := Resolve()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if src != SourceEnv {
		t.Errorf("expected SourceEnv, got %v", src)
	}
	if tok != "mem_live_from_env" {
		t.Errorf("expected mem_live_from_env, got %q", tok)
	}
}

func TestResolveEnv_TrimWhitespace(t *testing.T) {
	os.Setenv("MEMORIA_API_KEY", "  mem_live_padded  \n")
	t.Cleanup(func() { os.Unsetenv("MEMORIA_API_KEY") })

	tok, src, err := Resolve()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if src != SourceEnv {
		t.Errorf("expected SourceEnv, got %v", src)
	}
	if tok != "mem_live_padded" {
		t.Errorf("expected mem_live_padded, got %q", tok)
	}
}

// -- file tests ----------------------------------------------------------------

func TestResolveFile_Basic(t *testing.T) {
	isolateEnv(t)
	home := overrideHome(t)

	// Write a properly-permissioned credentials file.
	dir := filepath.Join(home, ".config", "memoria")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "credentials")
	if err := os.WriteFile(path, []byte("mem_live_from_file\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	tok, src, err := Resolve()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if src != SourceFile {
		t.Errorf("expected SourceFile, got %v", src)
	}
	if tok != "mem_live_from_file" {
		t.Errorf("expected mem_live_from_file, got %q", tok)
	}
}

func TestResolveFile_RejectsUnsafePerms(t *testing.T) {
	isolateEnv(t)
	home := overrideHome(t)

	dir := filepath.Join(home, ".config", "memoria")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "credentials")
	if err := os.WriteFile(path, []byte("mem_live_unsafe\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	_, _, err := Resolve()
	if err == nil {
		t.Fatal("expected error for 0644 permissions file, got nil")
	}
	// Fix 5: unsafe permissions must NOT collapse into ErrNoToken — the user
	// needs an actionable error message, not a "run memoria init" prompt.
	if errors.Is(err, ErrNoToken) {
		t.Errorf("expected a non-ErrNoToken error for unsafe permissions, got ErrNoToken")
	}
	// The error message should mention permissions so the user knows what to fix.
	if !strings.Contains(err.Error(), "permission") {
		t.Errorf("expected error message to mention 'permission', got: %v", err)
	}
}

func TestResolveFile_NotFound(t *testing.T) {
	isolateEnv(t)
	overrideHome(t)
	// No credentials file created.

	_, src, err := Resolve()
	if err == nil {
		t.Fatal("expected ErrNoToken, got nil")
	}
	if src != SourceUnknown {
		t.Errorf("expected SourceUnknown, got %v", src)
	}
}

// -- resolution order ----------------------------------------------------------

// TestResolutionOrder_EnvBeatsFile ensures $MEMORIA_API_KEY wins over file.
func TestResolutionOrder_EnvBeatsFile(t *testing.T) {
	home := overrideHome(t)

	dir := filepath.Join(home, ".config", "memoria")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "credentials")
	if err := os.WriteFile(path, []byte("mem_live_from_file\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	os.Setenv("MEMORIA_API_KEY", "mem_live_env_wins")
	t.Cleanup(func() { os.Unsetenv("MEMORIA_API_KEY") })

	tok, src, err := Resolve()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if src != SourceEnv {
		t.Errorf("expected SourceEnv, got %v", src)
	}
	if tok != "mem_live_env_wins" {
		t.Errorf("expected mem_live_env_wins, got %q", tok)
	}
}

// -- fileWrite atomicity -------------------------------------------------------

// TestFileWrite_NoTmpFilesAfterSuccess verifies that fileWrite leaves no
// .tmp files in the credentials directory after a successful write (Fix 4).
func TestFileWrite_NoTmpFilesAfterSuccess(t *testing.T) {
	isolateEnv(t)
	home := overrideHome(t)

	const token = "mem_live_atomic_test"
	if err := fileWrite(token); err != nil {
		t.Fatalf("fileWrite: %v", err)
	}

	dir := filepath.Join(home, ".config", "memoria")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("leftover temp file after successful fileWrite: %s", e.Name())
		}
	}

	// Verify the credentials file itself is present and readable.
	tok, err := fileRead()
	if err != nil {
		t.Fatalf("fileRead after fileWrite: %v", err)
	}
	if tok != token {
		t.Errorf("expected %q, got %q", token, tok)
	}
}

// -- keychain roundtrip (darwin only) -----------------------------------------

func TestKeychainRoundtrip(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("keychain test only runs on darwin")
	}

	const testToken = "mem_live_keychain_test_roundtrip"

	// Clean up before and after in case a previous run left a stale entry.
	keychainDelete() //nolint:errcheck

	t.Cleanup(func() { keychainDelete() }) //nolint:errcheck

	// Store into the keychain.
	if err := keychainSet(testToken); err != nil {
		t.Fatalf("keychainSet: %v", err)
	}

	// Retrieve.
	tok, ok := keychainGet()
	if !ok {
		t.Fatal("keychainGet returned false after storing token")
	}
	if tok != testToken {
		t.Errorf("expected %q, got %q", testToken, tok)
	}

	// Clear.
	keychainDelete() //nolint:errcheck

	// Should be gone now.
	_, ok = keychainGet()
	if ok {
		t.Error("keychainGet returned true after keychainDelete")
	}
}
