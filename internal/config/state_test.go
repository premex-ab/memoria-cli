package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// overrideHome redirects $HOME to a temp directory for the test.
func overrideHome(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	prev := os.Getenv("HOME")
	os.Setenv("HOME", tmp)
	t.Cleanup(func() { os.Setenv("HOME", prev) })
	return tmp
}

func TestWriteRead_RoundTrip(t *testing.T) {
	overrideHome(t)

	want := State{
		InstalledAt: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC),
		LastUpdated: time.Date(2026, 2, 3, 4, 5, 6, 0, time.UTC),
		LastChecked: time.Date(2026, 3, 4, 5, 6, 7, 0, time.UTC),
		TokenSource: "keychain",
		BoundTenant: "tenant-abc",
		BoundBrain:  "brain-xyz",
	}

	if err := Write(want); err != nil {
		t.Fatalf("Write: %v", err)
	}

	got, err := Read()
	if err != nil {
		t.Fatalf("Read: %v", err)
	}

	if !got.InstalledAt.Equal(want.InstalledAt) {
		t.Errorf("InstalledAt: want %v, got %v", want.InstalledAt, got.InstalledAt)
	}
	if !got.LastUpdated.Equal(want.LastUpdated) {
		t.Errorf("LastUpdated: want %v, got %v", want.LastUpdated, got.LastUpdated)
	}
	if !got.LastChecked.Equal(want.LastChecked) {
		t.Errorf("LastChecked: want %v, got %v", want.LastChecked, got.LastChecked)
	}
	if got.TokenSource != want.TokenSource {
		t.Errorf("TokenSource: want %q, got %q", want.TokenSource, got.TokenSource)
	}
	if got.BoundTenant != want.BoundTenant {
		t.Errorf("BoundTenant: want %q, got %q", want.BoundTenant, got.BoundTenant)
	}
	if got.BoundBrain != want.BoundBrain {
		t.Errorf("BoundBrain: want %q, got %q", want.BoundBrain, got.BoundBrain)
	}
}

func TestRead_MissingFile_ReturnsZero(t *testing.T) {
	overrideHome(t)
	// No state file written.

	s, err := Read()
	if err != nil {
		t.Fatalf("Read returned error for missing file: %v", err)
	}
	var zero State
	if s != zero {
		t.Errorf("expected zero State, got %+v", s)
	}
}

func TestWrite_SecurePerms(t *testing.T) {
	overrideHome(t)

	if err := Write(State{TokenSource: "env"}); err != nil {
		t.Fatalf("Write: %v", err)
	}

	path, err := Path()
	if err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	perm := info.Mode().Perm()
	if perm != 0o600 {
		t.Errorf("expected perm 0600, got %v", perm)
	}
}

func TestWrite_Atomic(t *testing.T) {
	tmp := overrideHome(t)

	first := State{TokenSource: "keychain", BoundTenant: "first"}
	if err := Write(first); err != nil {
		t.Fatalf("first Write: %v", err)
	}

	// Simulate the previous state surviving: write a second value and verify
	// the first can be read back if we restore it. Really what we're testing
	// is that Write produces a consistent file (not a half-written one). We
	// verify by checking the temp file was cleaned up.
	second := State{TokenSource: "file", BoundTenant: "second"}
	if err := Write(second); err != nil {
		t.Fatalf("second Write: %v", err)
	}

	// Verify no leftover .tmp files in the config dir.
	dir := filepath.Join(tmp, ".config", "memoria")
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	for _, e := range entries {
		if e.Name() != "state.json" {
			t.Errorf("unexpected file in config dir: %s", e.Name())
		}
	}

	// Verify the second state is what was written (not the first or something corrupt).
	got, err := Read()
	if err != nil {
		t.Fatalf("Read after second Write: %v", err)
	}
	if got.TokenSource != second.TokenSource {
		t.Errorf("expected TokenSource %q, got %q", second.TokenSource, got.TokenSource)
	}
	if got.BoundTenant != second.BoundTenant {
		t.Errorf("expected BoundTenant %q, got %q", second.BoundTenant, got.BoundTenant)
	}
}
