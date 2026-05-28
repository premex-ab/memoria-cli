// Package auth resolves, stores, and clears the active Memoria API token.
//
// Resolution order:
//  1. $MEMORIA_API_KEY env var
//  2. OS keychain (macOS)
//  3. ~/.config/memoria/credentials file fallback
package auth

import "errors"

// Source describes where a token came from.
type Source int

const (
	SourceUnknown Source = iota
	SourceEnv             // $MEMORIA_API_KEY
	SourceKeychain        // OS keychain
	SourceFile            // credentials file fallback
)

func (s Source) String() string {
	switch s {
	case SourceEnv:
		return "env"
	case SourceKeychain:
		return "keychain"
	case SourceFile:
		return "file"
	default:
		return "unknown"
	}
}

// ErrNoToken is returned when no token is available from any backend.
var ErrNoToken = errors.New("no Memoria API token found; run 'memoria init <token>' or set $MEMORIA_API_KEY")

// Resolve returns the active token and its source by checking, in order:
//  1. $MEMORIA_API_KEY env var
//  2. OS keychain (macOS; stub on other platforms)
//  3. ~/.config/memoria/credentials file fallback
//
// Returns (token, source, nil) on success, ("", SourceUnknown, err) if nothing
// is available.
func Resolve() (string, Source, error) {
	// 1. Env var — fast, works everywhere.
	if token, ok := resolveEnv(); ok {
		return token, SourceEnv, nil
	}

	// 2. OS keychain.
	if token, ok := keychainGet(); ok {
		return token, SourceKeychain, nil
	}

	// 3. File fallback.
	token, err := fileRead()
	if err != nil {
		return "", SourceUnknown, ErrNoToken
	}
	return token, SourceFile, nil
}

// Store persists a token. On macOS uses the keychain. On other platforms falls
// back to the file (with 0600 perms). If $MEMORIA_API_KEY is already set in
// the environment, Store is a no-op and returns SourceEnv — the caller is
// responsible for ensuring the env var is set at runtime.
func Store(token string) (Source, error) {
	// If env-var mode is already active, don't persist.
	if _, ok := resolveEnv(); ok {
		return SourceEnv, nil
	}

	if err := keychainSet(token); err == nil {
		return SourceKeychain, nil
	}

	if err := fileWrite(token); err != nil {
		return SourceUnknown, err
	}
	return SourceFile, nil
}

// Clear removes the persisted token from whichever backend it lives in.
// In env-var mode this is a no-op (we can't unset the user's env).
func Clear() error {
	if _, ok := resolveEnv(); ok {
		// Can't clear an env var; not an error.
		return nil
	}

	// Try keychain first; ignore "not found" errors.
	keychainDelete() //nolint:errcheck — best-effort

	// Also clear the file fallback, if present.
	fileDelete() //nolint:errcheck — best-effort

	return nil
}
