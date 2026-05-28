//go:build darwin

package auth

import (
	"bytes"
	"fmt"
	"os/exec"
	"os/user"
	"strings"
)

const (
	keychainService = "memoria.premex.se"
)

// currentUser returns the OS username via os/user.Current, which is reliable
// under launchd-spawned processes where $USER may be unset.
func currentUser() (string, error) {
	u, err := user.Current()
	if err != nil {
		return "", fmt.Errorf("look up current user: %w", err)
	}
	return u.Username, nil
}

// keychainGet retrieves the token from the macOS keychain.
// Returns ("", false) if not found or on any error.
func keychainGet() (string, bool) {
	username, err := currentUser()
	if err != nil {
		return "", false
	}
	out, err := exec.Command(
		"/usr/bin/security",
		"find-generic-password",
		"-s", keychainService,
		"-a", username,
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
	username, err := currentUser()
	if err != nil {
		return err
	}
	cmd := exec.Command(
		"/usr/bin/security",
		"add-generic-password",
		"-U",
		"-s", keychainService,
		"-a", username,
		"-w", token,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("keychain write failed (%w): %s", err, bytes.TrimSpace(out))
	}
	return nil
}

// keychainDelete removes the keychain entry. Best-effort; ignores not-found.
func keychainDelete() error {
	username, err := currentUser()
	if err != nil {
		return nil
	}
	out, err := exec.Command(
		"/usr/bin/security",
		"delete-generic-password",
		"-s", keychainService,
		"-a", username,
	).CombinedOutput()
	if err != nil {
		// "could not be found" is the expected output when the entry doesn't exist.
		if bytes.Contains(out, []byte("could not be found")) {
			return nil
		}
		return fmt.Errorf("keychain delete failed (%w): %s", err, bytes.TrimSpace(out))
	}
	return nil
}
