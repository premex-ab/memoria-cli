package auth

import (
	"os"
	"strings"
)

const envKey = "MEMORIA_API_KEY"

// resolveEnv reads $MEMORIA_API_KEY, trims whitespace, and returns it.
// Returns ("", false) if the variable is unset or empty after trimming.
func resolveEnv() (string, bool) {
	v := strings.TrimSpace(os.Getenv(envKey))
	if v == "" {
		return "", false
	}
	return v, true
}
