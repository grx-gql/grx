package server

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/patrickkabwe/grx/core"
	"github.com/patrickkabwe/grx/schema"
)

type trivialQuery struct{}

func (trivialQuery) Hi() string { return "hi" }

func TestRandomRequestIDUnknownWhenRandFails(t *testing.T) {
	orig := randRead
	t.Cleanup(func() { randRead = orig })
	randRead = func([]byte) (int, error) { return 0, errors.New("entropy failure") }
	if got := randomRequestID(); got != "unknown" {
		t.Fatalf("unexpected id %q", got)
	}
}

func TestPathRestrictedMatchRequiresCanonicalPath(t *testing.T) {
	inner := &noopTransport{}
	pr := pathRestricted{path: "/graphql", Transport: inner}

	off := httptest.NewRequest(http.MethodGet, "/other", nil)
	if pr.Match(off) {
		t.Fatal("unexpected match for wrong path")
	}

	on := httptest.NewRequest(http.MethodGet, "/graphql", nil)
	if !pr.Match(on) {
		t.Fatal("expected match on configured path")
	}
}

func TestNewErrorsWhenSeparateSubsHasNoStreamingTransports(t *testing.T) {
	_, err := New(Config{
		Schema:           schema.Config{Query: trivialQuery{}},
		GraphQLPath:      "/gq",
		SubscriptionPath: "/subs",
		Transports: []core.Transport{
			customHTTPTransport{},
		},
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "SubscriptionPath differs") {
		t.Fatalf("unexpected error: %v", err)
	}
}

type noopTransport struct{}

func (noopTransport) Match(*http.Request) bool                                { return true }
func (noopTransport) Serve(http.ResponseWriter, *http.Request, core.Executor) {}

// customHTTPTransport is not *websocket.Transport or *sse.Transport.
type customHTTPTransport struct{}

func (customHTTPTransport) Match(*http.Request) bool { return true }

func (customHTTPTransport) Serve(http.ResponseWriter, *http.Request, core.Executor) {}

type failingBodyWriter struct {
	hdr  http.Header
	code int
}

func (w *failingBodyWriter) Header() http.Header {
	if w.hdr == nil {
		w.hdr = make(http.Header)
	}
	return w.hdr
}

func (w *failingBodyWriter) WriteHeader(code int) { w.code = code }

func (w *failingBodyWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestServePlaygroundReturns500WhenWriteFails(t *testing.T) {
	w := &failingBodyWriter{}
	servePlayground(w, "/graphql", "/graphql")
	if w.code != http.StatusInternalServerError {
		t.Fatalf("status = %d", w.code)
	}
}
