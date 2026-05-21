package cors

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewHandlesPreflight(t *testing.T) {
	middleware := New(Config{
		AllowedOrigins: []string{"http://localhost:4000"},
		AllowedMethods: []string{http.MethodPost, http.MethodOptions},
		AllowedHeaders: []string{"Content-Type", "Authorization"},
		MaxAge:         time.Minute,
	})
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("preflight should not call next handler")
	}))
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodOptions, "/graphql", nil)
	request.Header.Set("Origin", "http://localhost:4000")
	request.Header.Set("Access-Control-Request-Method", http.MethodPost)
	request.Header.Set("Access-Control-Request-Headers", "Content-Type, Authorization")

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusNoContent {
		t.Fatalf("expected status %d, got %d", http.StatusNoContent, response.Code)
	}
	if got := response.Header().Get("Access-Control-Allow-Origin"); got != "http://localhost:4000" {
		t.Fatalf("unexpected allow origin header %q", got)
	}
	if got := response.Header().Get("Access-Control-Allow-Methods"); got != "POST, OPTIONS" {
		t.Fatalf("unexpected allow methods header %q", got)
	}
	if got := response.Header().Get("Access-Control-Allow-Headers"); got != "Content-Type, Authorization" {
		t.Fatalf("unexpected allow headers header %q", got)
	}
	if got := response.Header().Get("Access-Control-Max-Age"); got != "60" {
		t.Fatalf("unexpected max age header %q", got)
	}
}

func TestNewRejectsDisallowedOrigin(t *testing.T) {
	middleware := New(Config{
		AllowedOrigins: []string{"http://localhost:4000"},
		AllowedMethods: []string{http.MethodPost},
	})
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("disallowed origin should not call next handler")
	}))
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/graphql", nil)
	request.Header.Set("Origin", "http://evil.example.com")

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d", http.StatusForbidden, response.Code)
	}
}

func TestNewWritesCredentialHeaders(t *testing.T) {
	middleware := New(Config{
		AllowedOrigins:   []string{"http://localhost:4000"},
		AllowedMethods:   []string{http.MethodPost},
		AllowCredentials: true,
	})
	handler := middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusAccepted)
	}))
	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/graphql", nil)
	request.Header.Set("Origin", "http://localhost:4000")

	handler.ServeHTTP(response, request)

	if response.Code != http.StatusAccepted {
		t.Fatalf("expected status %d, got %d", http.StatusAccepted, response.Code)
	}
	if got := response.Header().Get("Access-Control-Allow-Credentials"); got != "true" {
		t.Fatalf("unexpected allow credentials header %q", got)
	}
}
