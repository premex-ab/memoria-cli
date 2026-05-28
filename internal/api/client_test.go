package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWhoami_200_ParsedResponse(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify the request targets the correct path.
		if r.URL.Path != "/v1/whoami" {
			t.Errorf("expected path /v1/whoami, got %s", r.URL.Path)
		}
		// Verify the Authorization header is present and has the right format.
		auth := r.Header.Get("Authorization")
		if auth != "Bearer mem_live_testtoken" {
			t.Errorf("expected Authorization: Bearer mem_live_testtoken, got %q", auth)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"tenantId": "t",
			"brainId":  "b",
			"scopes":   []string{"memory:read"},
			"kind":     "live",
		})
	}))
	defer srv.Close()

	client := New(srv.URL)
	resp, err := client.Whoami(context.Background(), "mem_live_testtoken")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.TenantID != "t" {
		t.Errorf("expected TenantID 't', got %q", resp.TenantID)
	}
	if resp.BrainID != "b" {
		t.Errorf("expected BrainID 'b', got %q", resp.BrainID)
	}
	if len(resp.Scopes) != 1 || resp.Scopes[0] != "memory:read" {
		t.Errorf("expected scopes [memory:read], got %v", resp.Scopes)
	}
	if resp.Kind != "live" {
		t.Errorf("expected kind 'live', got %q", resp.Kind)
	}
}

func TestWhoami_401_ReturnsAuthError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusUnauthorized)
		w.Write([]byte(`{"error":{"code":"invalid_key","message":"unknown or revoked api key"}}`))
	}))
	defer srv.Close()

	client := New(srv.URL)
	_, err := client.Whoami(context.Background(), "mem_live_badtoken")
	if err == nil {
		t.Fatal("expected error for 401, got nil")
	}

	// Must be an *AuthError.
	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Errorf("expected *AuthError, got %T: %v", err, err)
	}

	// Must satisfy errors.Is(err, ErrInvalidToken).
	if !errors.Is(err, ErrInvalidToken) {
		t.Errorf("expected errors.Is(err, ErrInvalidToken) == true, got false")
	}
}

func TestWhoami_500_ReturnsGenericError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("internal server error"))
	}))
	defer srv.Close()

	client := New(srv.URL)
	_, err := client.Whoami(context.Background(), "mem_live_anytoken")
	if err == nil {
		t.Fatal("expected error for 500, got nil")
	}

	// Must NOT be an AuthError.
	var authErr *AuthError
	if errors.As(err, &authErr) {
		t.Errorf("expected generic error (not AuthError) for 500, got *AuthError")
	}

	// Must NOT satisfy ErrInvalidToken.
	if errors.Is(err, ErrInvalidToken) {
		t.Errorf("expected errors.Is(err, ErrInvalidToken) == false for 500")
	}
}

func TestWhoami_SendsCorrectAuthHeader(t *testing.T) {
	const token = "mem_live_abc123"
	var gotAuth string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"tenantId": "x",
			"brainId":  "y",
			"scopes":   []string{},
			"kind":     "live",
		})
	}))
	defer srv.Close()

	client := New(srv.URL)
	_, err := client.Whoami(context.Background(), token)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := "Bearer " + token
	if gotAuth != expected {
		t.Errorf("expected Authorization header %q, got %q", expected, gotAuth)
	}
}
