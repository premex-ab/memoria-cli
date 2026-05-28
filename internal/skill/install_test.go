package skill

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/premex-ab/memoria-cli/internal/version"
)

// TestEmbeddedVersion confirms the embedded SKILL.md frontmatter parses and
// returns the expected version string.
func TestEmbeddedVersion(t *testing.T) {
	ver, err := EmbeddedVersion()
	if err != nil {
		t.Fatalf("EmbeddedVersion() returned error: %v", err)
	}
	if ver != "0.1.0" {
		t.Errorf("expected version 0.1.0, got %q", ver)
	}
}

// TestInstall_FreshHome verifies that Install writes the skill file when it
// doesn't exist yet.
func TestInstall_FreshHome(t *testing.T) {
	home := t.TempDir()
	result, err := Install(home, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Install returned error: %v", err)
	}
	if result.Outcome != OutcomeInstalled {
		t.Errorf("expected OutcomeInstalled, got %v", result.Outcome)
	}

	destPath := filepath.Join(home, ".claude", "skills", "memoria", "SKILL.md")

	raw, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("expected SKILL.md to exist at %s, got error: %v", destPath, err)
	}

	if string(raw) != string(EmbeddedSKILL) {
		t.Errorf("SKILL.md contents do not match embedded SKILL")
	}

	info, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("stat SKILL.md: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Errorf("expected perms 0600, got %04o", perm)
	}
}

// TestInstall_SkipsWhenSameVersion verifies that Install leaves an identical
// file alone and returns OutcomeSkipped.
func TestInstall_SkipsWhenSameVersion(t *testing.T) {
	home := t.TempDir()
	destDir := filepath.Join(home, ".claude", "skills", "memoria")
	destPath := filepath.Join(destDir, "SKILL.md")

	// Pre-create file with the same content as embedded.
	if err := os.MkdirAll(destDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(destPath, EmbeddedSKILL, 0o600); err != nil {
		t.Fatalf("pre-create: %v", err)
	}

	info1, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("stat before: %v", err)
	}

	result, err := Install(home, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Install returned error: %v", err)
	}
	if result.Outcome != OutcomeSkipped {
		t.Errorf("expected OutcomeSkipped, got %v", result.Outcome)
	}

	// mtime should be unchanged (file not touched).
	info2, err := os.Stat(destPath)
	if err != nil {
		t.Fatalf("stat after: %v", err)
	}
	if !info2.ModTime().Equal(info1.ModTime()) {
		t.Errorf("expected mtime unchanged, but it changed from %v to %v",
			info1.ModTime(), info2.ModTime())
	}
}

// TestInstall_UpdatesWhenEmbeddedIsNewer verifies that Install overwrites when
// the embedded version is strictly newer than the file on disk.
func TestInstall_UpdatesWhenEmbeddedIsNewer(t *testing.T) {
	home := t.TempDir()
	destDir := filepath.Join(home, ".claude", "skills", "memoria")
	destPath := filepath.Join(destDir, "SKILL.md")

	// Pre-create with an older version.
	oldContent := strings.Replace(string(EmbeddedSKILL), "version: 0.1.0", "version: 0.0.1", 1)
	if err := os.MkdirAll(destDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(destPath, []byte(oldContent), 0o600); err != nil {
		t.Fatalf("pre-create: %v", err)
	}

	result, err := Install(home, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Install returned error: %v", err)
	}
	if result.Outcome != OutcomeUpdated {
		t.Errorf("expected OutcomeUpdated, got %v", result.Outcome)
	}

	raw, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read after install: %v", err)
	}
	if string(raw) != string(EmbeddedSKILL) {
		t.Errorf("expected file to be updated to embedded content")
	}
}

// TestInstall_LeavesAloneWhenExistingIsNewer verifies that Install skips when
// the file on disk claims a newer version than the embedded copy.
func TestInstall_LeavesAloneWhenExistingIsNewer(t *testing.T) {
	home := t.TempDir()
	destDir := filepath.Join(home, ".claude", "skills", "memoria")
	destPath := filepath.Join(destDir, "SKILL.md")

	// Pre-create with a much newer version.
	newerContent := strings.Replace(string(EmbeddedSKILL), "version: 0.1.0", "version: 99.0.0", 1)
	if err := os.MkdirAll(destDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(destPath, []byte(newerContent), 0o600); err != nil {
		t.Fatalf("pre-create: %v", err)
	}

	result, err := Install(home, &bytes.Buffer{})
	if err != nil {
		t.Fatalf("Install returned error: %v", err)
	}
	if result.Outcome != OutcomeSkipped {
		t.Errorf("expected OutcomeSkipped, got %v", result.Outcome)
	}

	raw, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read after install: %v", err)
	}
	if string(raw) != newerContent {
		t.Errorf("expected file contents to be unchanged (newer version should be kept)")
	}
}

// TestInstall_OverwritesUnparseable verifies that Install replaces a file that
// has no valid frontmatter, returning OutcomeOverwrittenUnparseable, and writes
// a warning containing "unparseable" and the destination path to stderr.
func TestInstall_OverwritesUnparseable(t *testing.T) {
	home := t.TempDir()
	destDir := filepath.Join(home, ".claude", "skills", "memoria")
	destPath := filepath.Join(destDir, "SKILL.md")

	if err := os.MkdirAll(destDir, 0o700); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(destPath, []byte("this is garbage content, no frontmatter"), 0o600); err != nil {
		t.Fatalf("pre-create: %v", err)
	}

	var stderrBuf bytes.Buffer
	result, err := Install(home, &stderrBuf)
	if err != nil {
		t.Fatalf("Install returned error: %v", err)
	}
	if result.Outcome != OutcomeOverwrittenUnparseable {
		t.Errorf("expected OutcomeOverwrittenUnparseable, got %v", result.Outcome)
	}

	// Stderr must warn about the unparseable file and include the destination path.
	stderrStr := stderrBuf.String()
	if !strings.Contains(stderrStr, "unparseable") && !strings.Contains(stderrStr, "frontmatter") {
		t.Errorf("expected stderr to mention unparseable/frontmatter, got: %q", stderrStr)
	}
	if !strings.Contains(stderrStr, destPath) {
		t.Errorf("expected stderr to contain the destination path %q, got: %q", destPath, stderrStr)
	}

	raw, err := os.ReadFile(destPath)
	if err != nil {
		t.Fatalf("read after install: %v", err)
	}
	if string(raw) != string(EmbeddedSKILL) {
		t.Errorf("expected file to be replaced with embedded content")
	}
}

// TestCompareSemver_ViaVersion delegates to the version package's exported
// CompareSemver and verifies the skill package can use it correctly.
func TestCompareSemver_ViaVersion(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"0.1.0", "0.2.0", -1},
		{"0.2.0", "0.1.0", 1},
		{"1.0.0", "0.9.9", 1},
		{"0.9.9", "1.0.0", -1},
		{"0.1.0", "0.1.0", 0},
		{"1.2.3", "1.2.3", 0},
		{"1.2.4", "1.2.3", 1},
		{"1.2.3", "1.2.4", -1},
		{"2.0.0", "1.99.99", 1},
		{"0.0.0", "0.0.1", -1},
		{"99.0.0", "0.1.0", 1},
		// Malformed versions fall back to 0.0.0.
		{"", "0.0.0", 0},
		{"not-a-version", "0.0.0", 0},
	}

	for _, c := range cases {
		got := version.CompareSemver(c.a, c.b)
		if got != c.want {
			t.Errorf("version.CompareSemver(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
	}
}
