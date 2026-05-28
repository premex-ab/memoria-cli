package auth

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ErrUnsafePermissions is returned by fileRead when the credentials file has
// permissions more permissive than 0600. Callers can match with errors.Is.
var ErrUnsafePermissions = errors.New("credentials file has unsafe permissions")

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
		return "", os.ErrNotExist
	}
	if err != nil {
		return "", fmt.Errorf("stat credentials: %w", err)
	}

	// Reject if mode is more permissive than 0600 (owner read+write only).
	if info.Mode().Perm()&0o177 != 0 {
		return "", fmt.Errorf(
			"%w: %s has permissions %v; run: chmod 600 %s",
			ErrUnsafePermissions, path, info.Mode().Perm(), path,
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

// fileWrite atomically writes the token to ~/.config/memoria/credentials with
// 0600 perms. Uses a temp file in the same directory so the rename is atomic.
// Creates parent directories as needed.
func fileWrite(token string) error {
	path, err := credentialsPath()
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}

	// Write to a temp file in the same directory so the rename is atomic.
	tmp, err := os.CreateTemp(dir, "credentials-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp credentials file: %w", err)
	}
	tmpName := tmp.Name()
	committed := false
	defer func() {
		if !committed {
			os.Remove(tmpName)
		}
	}()

	if _, err := tmp.WriteString(token + "\n"); err != nil {
		tmp.Close()
		return fmt.Errorf("write temp credentials: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return fmt.Errorf("chmod temp credentials: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close temp credentials: %w", err)
	}

	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("rename credentials file: %w", err)
	}
	committed = true
	return nil
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
