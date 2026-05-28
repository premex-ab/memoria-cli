package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// buildBinary compiles the memoria binary into a temp directory and returns its
// path. Skips the test if compilation fails.
func buildBinary(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	bin := filepath.Join(tmp, "memoria")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}

	cmd := exec.Command("go", "build", "-o", bin, ".")
	cmd.Dir = filepath.Join(getRootDir(t), "cmd", "memoria")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build failed: %v\n%s", err, out)
	}
	return bin
}

// getRootDir returns the module root directory (two levels up from cmd/memoria).
func getRootDir(t *testing.T) string {
	t.Helper()
	// __file__ is cmd/memoria/main_test.go → go up two dirs.
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(file), "..", "..")
}

func TestHeaders_WithEnvVar(t *testing.T) {
	bin := buildBinary(t)

	cmd := exec.Command(bin, "headers")
	cmd.Env = append(os.Environ(), "MEMORIA_API_KEY=mem_live_foo")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		t.Fatalf("headers exited non-zero: %v\nstderr: %s", err, stderr.String())
	}

	got := strings.TrimSpace(stdout.String())
	want := `{"Authorization":"Bearer mem_live_foo"}`
	if got != want {
		t.Errorf("stdout: want %q, got %q", want, got)
	}
}

func TestHeaders_NoToken_NonZeroExit(t *testing.T) {
	bin := buildBinary(t)

	// Use a fresh HOME with no credentials file and no MEMORIA_API_KEY.
	tmp := t.TempDir()
	cmd := exec.Command(bin, "headers")
	cmd.Env = []string{
		"HOME=" + tmp,
		"PATH=" + os.Getenv("PATH"),
		"USER=" + os.Getenv("USER"),
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err == nil {
		t.Fatal("expected non-zero exit when no token is configured")
	}
	stderrStr := stderr.String()
	if stderrStr == "" {
		t.Error("expected something on stderr when no token is configured")
	}
	// Verify our styled error message appears exactly once — if cobra's SilenceErrors
	// is not set, the error gets printed twice (once by our RunE, once by cobra).
	if count := strings.Count(stderrStr, "no API token found"); count != 1 {
		t.Errorf("expected 'no API token found' to appear exactly once in stderr, got %d occurrences:\n%s", count, stderrStr)
	}
}

func TestVersion(t *testing.T) {
	bin := buildBinary(t)

	cmd := exec.Command(bin, "--version")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		t.Fatalf("--version exited non-zero: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "dev") {
		t.Errorf("expected 'dev' in version output, got %q", out)
	}
}

func TestHelp(t *testing.T) {
	bin := buildBinary(t)

	cmd := exec.Command(bin, "--help")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		t.Fatalf("--help exited non-zero: %v", err)
	}

	out := stdout.String()
	if !strings.Contains(out, "headers") {
		t.Errorf("expected 'headers' subcommand in help output, got:\n%s", out)
	}
}
