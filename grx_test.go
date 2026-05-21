package grx

import (
	"bytes"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/patrickkabwe/grx/pkg/websocket"
	"github.com/patrickkabwe/grx/schema"
)

type middlewareQuery struct{}

func (middlewareQuery) Hello() string { return "hi" }

func TestWithMiddlewareWrapsGraphQLRequests(t *testing.T) {
	srv, err := NewServer(
		WithSchema(schema.Config{Query: middlewareQuery{}}),
		WithMiddleware(func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("X-Test-Middleware", "yes")
				next.ServeHTTP(w, r)
			})
		}),
	)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	response := httptest.NewRecorder()
	request := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(`{"query":"{ hello }"}`))
	request.Header.Set("Content-Type", "application/json")

	srv.ServeHTTP(response, request)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}
	if got := response.Header().Get("X-Test-Middleware"); got != "yes" {
		t.Fatalf("expected middleware header, got %q", got)
	}
}

func TestCorsHandlesPreflightAndGraphQLRequests(t *testing.T) {
	srv, err := NewServer(
		WithSchema(schema.Config{Query: middlewareQuery{}}),
		WithMiddleware(Cors(CorsConfig{
			AllowedOrigins: []string{"http://localhost:4000"},
			AllowedMethods: []string{http.MethodPost, http.MethodOptions},
			AllowedHeaders: []string{"Content-Type", "Authorization"},
			MaxAge:         time.Minute,
		})),
	)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}

	t.Run("preflight", func(t *testing.T) {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodOptions, "/graphql", nil)
		request.Header.Set("Origin", "http://localhost:4000")
		request.Header.Set("Access-Control-Request-Method", http.MethodPost)
		request.Header.Set("Access-Control-Request-Headers", "Content-Type, Authorization")

		srv.ServeHTTP(response, request)

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
	})

	t.Run("disallowed origin", func(t *testing.T) {
		response := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodPost, "/graphql", bytes.NewBufferString(`{"query":"{ hello }"}`))
		request.Header.Set("Content-Type", "application/json")
		request.Header.Set("Origin", "http://evil.example.com")

		srv.ServeHTTP(response, request)

		if response.Code != http.StatusForbidden {
			t.Fatalf("expected status %d, got %d", http.StatusForbidden, response.Code)
		}
	})
}

func TestCorsChecksWebSocketOrigin(t *testing.T) {
	srv, err := NewServer(
		WithSchema(schema.Config{Query: middlewareQuery{}, Subscription: middlewareSubscription{}}),
		WithMiddleware(Cors(CorsConfig{
			AllowedOrigins: []string{"http://localhost:4000"},
			AllowedMethods: []string{http.MethodGet, http.MethodPost, http.MethodOptions},
			AllowedHeaders: []string{"Content-Type", "Authorization"},
		})),
		WithTransports(websocket.New()),
	)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	ts := httptest.NewServer(srv)
	defer ts.Close()

	disallowedStatus := readHandshakeStatus(t, ts.URL, "http://evil.example.com")
	if !strings.Contains(disallowedStatus, "403") {
		t.Fatalf("expected forbidden handshake, got %q", disallowedStatus)
	}

	allowedStatus := readHandshakeStatus(t, ts.URL, "http://localhost:4000")
	if !strings.Contains(allowedStatus, "101") {
		t.Fatalf("expected switching protocols handshake, got %q", allowedStatus)
	}
}

type middlewareSubscription struct{}

func (middlewareSubscription) Hello() <-chan string {
	out := make(chan string)
	close(out)
	return out
}

func readHandshakeStatus(t *testing.T, baseURL string, origin string) string {
	t.Helper()
	parsed, err := url.Parse(baseURL)
	if err != nil {
		t.Fatalf("parse base url: %v", err)
	}
	conn, err := net.DialTimeout("tcp", parsed.Host, 5*time.Second)
	if err != nil {
		t.Fatalf("dial tcp: %v", err)
	}
	defer conn.Close()

	req := "GET /graphql HTTP/1.1\r\n" +
		"Host: " + parsed.Host + "\r\n" +
		"Origin: " + origin + "\r\n" +
		"Upgrade: websocket\r\n" +
		"Connection: Upgrade\r\n" +
		"Sec-WebSocket-Version: 13\r\n" +
		"Sec-WebSocket-Key: dGhlIHNhbXBsZSBub25jZQ==\r\n" +
		"Sec-WebSocket-Protocol: " + websocket.Subprotocol + "\r\n\r\n"
	if _, err := conn.Write([]byte(req)); err != nil {
		t.Fatalf("send handshake: %v", err)
	}
	buf := make([]byte, 2048)
	n, err := conn.Read(buf)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	raw := string(buf[:n])
	return strings.TrimSpace(strings.Split(raw, "\n")[0])
}
