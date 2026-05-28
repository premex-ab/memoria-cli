//go:build !darwin

package auth

import "errors"

// keychainGet is a stub for non-darwin platforms.
// Phase 2 will add libsecret support for Linux.
func keychainGet() (string, bool) {
	return "", false
}

// keychainSet is a stub for non-darwin platforms; callers fall back to the file backend.
func keychainSet(_ string) error {
	return errors.New("keychain not supported on this platform; using file fallback")
}

// keychainDelete is a stub for non-darwin platforms.
func keychainDelete() error {
	return nil
}
