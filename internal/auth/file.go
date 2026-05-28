package auth

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const credentialsFile = "credentials"

// credentialsPath returns ~/.config/memoria/credentials.
func credentialsPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("cannot determine home directory: %w", err)
	}
	return filepath.Join(home, ".config", "memoria", credentialsFile), nil
}

// fileRead reads the token from ~/.config/memoria/credentials.
// Refuses to read the file if it has world- or group-readable permissions (>0600).
func fileRead() (string, error) {
	path, err := credentialsPath()
	if err != nil {
		return "", err
	}

	info, err := os.Stat(path)
	if errors.Is(err, os.ErrNotExist) {
		return "", errors.New("credentials file not found")
	}
	if err != nil {
		return "", fmt.Errorf("stat credentials: %w", err)
	}

	// Reject if mode is more permissive than 0600 (owner read+write only).
	if info.Mode().Perm()&0o177 != 0 {
		return "", fmt.Errorf(
			"credentials file %s has unsafe permissions (%v); run: chmod 600 %s",
			path, info.Mode().Perm(), path,
		)
	}

	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read credentials: %w", err)
	}
	token := strings.TrimSpace(string(raw))
	if token == "" {
		return "", errors.New("credentials file is empty")
	}
	return token, nil
}

// fileWrite writes the token to ~/.config/memoria/credentials with 0600 perms.
// Creates parent directories as needed.
func fileWrite(token string) error {
	path, err := credentialsPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	return os.WriteFile(path, []byte(token+"\n"), 0o600)
}

// fileDelete removes the credentials file. Ignores not-found.
func fileDelete() error {
	path, err := credentialsPath()
	if err != nil {
		return nil
	}
	err = os.Remove(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}
