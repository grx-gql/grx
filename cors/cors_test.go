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

func TestPassesThroughWhenNoOrigin(t *testing.T) {
	called := false
	handler := New(Config{})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		called = true
	}))
	handler.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))
	if !called {
		t.Fatal("expected next without Origin header")
	}
}

func TestWildcardOriginWithoutCredentialsUsesStar(t *testing.T) {
	middleware := New(Config{
		AllowedOrigins: []string{"*", " ", "extra"},
		AllowedMethods: []string{http.MethodGet},
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://any.where")
	middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {})).ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "*" {
		t.Fatalf("want * Allow-Origin, got %q", got)
	}
}

func TestWildcardOriginWithCredentialsFailsClosed(t *testing.T) {
	handler := New(Config{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{http.MethodGet},
		AllowCredentials: true,
	})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("wildcard credentials should not call next handler")
	}))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "https://any.where")

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusForbidden)
	}
}

func TestExposedHeadersSet(t *testing.T) {
	middleware := New(Config{
		AllowedOrigins: []string{"http://a"},
		AllowedMethods: []string{http.MethodGet},
		ExposedHeaders: []string{"X-Custom", "X-Other"},
	})
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("Origin", "http://a")
	middleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})).ServeHTTP(rec, req)
	if got := rec.Header().Get("Access-Control-Expose-Headers"); got != "X-Custom, X-Other" {
		t.Fatalf("expose headers: got %q", got)
	}
}

func TestPreflightRejectsMethod(t *testing.T) {
	handler := New(Config{
		AllowedOrigins: []string{"http://a"},
		AllowedMethods: []string{http.MethodPost},
		AllowedHeaders: []string{"Content-Type"},
	})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("next") }))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "http://a")
	req.Header.Set("Access-Control-Request-Method", http.MethodDelete)
	req.Header.Set("Access-Control-Request-Headers", "Content-Type")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestPreflightRejectsHeaders(t *testing.T) {
	handler := New(Config{
		AllowedOrigins: []string{"http://a"},
		AllowedMethods: []string{http.MethodPost},
		AllowedHeaders: []string{"Content-Type"},
	})(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("next") }))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "http://a")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "Authorization")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status %d", rec.Code)
	}
}

func TestPreflightWildcardHeaders(t *testing.T) {
	middleware := New(Config{
		AllowedOrigins: []string{"http://a"},
		AllowedMethods: []string{http.MethodPost},
		AllowedHeaders: []string{"*"},
		MaxAge:         2 * time.Second,
	})
	handler := middleware(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { t.Fatal("next") }))
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodOptions, "/", nil)
	req.Header.Set("Origin", "http://a")
	req.Header.Set("Access-Control-Request-Method", http.MethodPost)
	req.Header.Set("Access-Control-Request-Headers", "X-Anything, Authorization")
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status %d", rec.Code)
	}
	if rec.Header().Get("Access-Control-Max-Age") != "2" {
		t.Fatalf("max-age: %s", rec.Header().Get("Access-Control-Max-Age"))
	}
}
