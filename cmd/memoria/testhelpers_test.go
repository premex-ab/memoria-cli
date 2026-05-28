package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// buildFakeWhoamiServer returns a test server that asserts the inbound
// Authorization header matches "Bearer <expectedToken>" (responding 401
// otherwise) and replies 200 with the given identity JSON on success.
func buildFakeWhoamiServer(t *testing.T, expectedToken, tenantID, brainID string, scopes []string) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/whoami" {
			http.NotFound(w, r)
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+expectedToken {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"missing or invalid Authorization header"}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]any{
			"tenantId": tenantID,
			"brainId":  brainID,
			"scopes":   scopes,
			"kind":     "live",
		})
	}))
}
