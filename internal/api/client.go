// Package api provides a thin HTTP client for the Memoria API endpoints used by the CLI.
package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/premex-ab/memoria-cli/internal/version"
)

// ErrInvalidToken is the sentinel error returned (wrapped in AuthError) when
// the server responds with HTTP 401.
var ErrInvalidToken = errors.New("invalid or revoked token")

// AuthError is returned when the Memoria API rejects the supplied token.
// It wraps ErrInvalidToken so callers can use errors.Is.
type AuthError struct {
	// Body is the raw response body from the server.
	Body string
}

func (e *AuthError) Error() string {
	return fmt.Sprintf("token rejected by server: %s", e.Body)
}

// Is implements the errors.Is interface so that errors.Is(err, ErrInvalidToken)
// returns true when err is an *AuthError.
func (e *AuthError) Is(target error) bool {
	return target == ErrInvalidToken
}

// WhoamiResponse is the parsed body from GET /v1/whoami.
// JSON tags use camelCase to match the server's response shape exactly.
type WhoamiResponse struct {
	TenantID string   `json:"tenantId"`
	BrainID  string   `json:"brainId"`
	Scopes   []string `json:"scopes"`
	Kind     string   `json:"kind"`
}

// Client is a minimal HTTP client for the Memoria API.
type Client struct {
	BaseURL    string
	HTTPClient *http.Client
}

// New returns a Client targeting baseURL with a 10-second timeout.
func New(baseURL string) *Client {
	return &Client{
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// Whoami calls GET {BaseURL}/v1/whoami with the supplied token in the
// Authorization header and returns the parsed identity response.
//
// On 401, it returns a typed *AuthError wrapping ErrInvalidToken.
// On other non-2xx responses it returns a generic error with the status and body.
func (c *Client) Whoami(ctx context.Context, token string) (*WhoamiResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/v1/whoami", nil)
	if err != nil {
		return nil, fmt.Errorf("whoami: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "memoria-cli/"+version.Version)

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("whoami: request: %w", err)
	}
	defer resp.Body.Close()

	// Limit the response body to 64 KiB to guard against a misbehaving server
	// returning a huge error body that would OOM the CLI.
	bodyBytes, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return nil, fmt.Errorf("whoami: read body: %w", err)
	}
	body := string(bodyBytes)

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, &AuthError{Body: body}
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("whoami: HTTP %d: %s", resp.StatusCode, body)
	}

	var result WhoamiResponse
	if err := json.Unmarshal(bodyBytes, &result); err != nil {
		return nil, fmt.Errorf("whoami: decode response: %w", err)
	}
	return &result, nil
}
