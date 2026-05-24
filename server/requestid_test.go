package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/grx-gql/grx/core"
)

func TestRandomRequestIDUnknownWhenRandFails(t *testing.T) {
	orig := randRead
	t.Cleanup(func() { randRead = orig })
	randRead = func([]byte) (int, error) { return 0, errors.New("entropy failure") }
	if got := randomRequestID(); got != "unknown" {
		t.Fatalf("unexpected id %q", got)
	}
}

func TestRequestIDUsesExistingAndGeneratedValues(t *testing.T) {
	handler := RequestID("X-Trace")(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := core.RequestIDFromContext(r.Context())
		if id == "" {
			t.Fatalf("missing request id")
		}
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Trace", "incoming")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if got := rec.Header().Get("X-Trace"); got != "incoming" {
		t.Fatalf("incoming response id = %q", got)
	}

	req = httptest.NewRequest(http.MethodGet, "/", nil)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if got := rec.Header().Get("X-Trace"); got == "" || got == "unknown" {
		t.Fatalf("generated response id = %q", got)
	}
}
