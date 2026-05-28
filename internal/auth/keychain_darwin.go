//go:build darwin

package auth

import (
	"errors"
	"os"
	"os/exec"
	"strings"
)

const (
	keychainService = "memoria.premex.se"
)

// keychainGet retrieves the token from the macOS keychain.
// Returns ("", false) if not found or on any error.
func keychainGet() (string, bool) {
	user := os.Getenv("USER")
	if user == "" {
		return "", false
	}
	out, err := exec.Command(
		"/usr/bin/security",
		"find-generic-password",
		"-s", keychainService,
		"-a", user,
		"-w",
	).Output()
	if err != nil {
		return "", false
	}
	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", false
	}
	return token, true
}

// keychainSet stores the token in the macOS keychain, updating an existing
// entry if one exists (-U flag).
func keychainSet(token string) error {
	user := os.Getenv("USER")
	if user == "" {
		return errors.New("$USER is not set; cannot write to keychain")
	}
	cmd := exec.Command(
		"/usr/bin/security",
		"add-generic-password",
		"-U",
		"-s", keychainService,
		"-a", user,
		"-w", token,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return errors.New("keychain write failed: " + string(out))
	}
	return nil
}

// keychainDelete removes the keychain entry. Best-effort; ignores not-found.
func keychainDelete() error {
	user := os.Getenv("USER")
	if user == "" {
		return nil
	}
	exec.Command( //nolint:errcheck
		"/usr/bin/security",
		"delete-generic-password",
		"-s", keychainService,
		"-a", user,
	).Run()
	return nil
}
