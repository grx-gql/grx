package grx

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/grx-gql/grx/plugins/logger"
	"github.com/grx-gql/grx/schema"
	"github.com/grx-gql/grx/websocket"
)

type middlewareQuery struct{}

func (middlewareQuery) Hello() string { return "hi" }

func TestNewServer_RequiresSchema(t *testing.T) {
	_, err := NewServer()
	if !errors.Is(err, ErrMissingSchema) {
		t.Fatalf("expected ErrMissingSchema, got %v", err)
	}
}

func TestAllForwardingOptionsSmoke(t *testing.T) {
	logPlug, err := logger.New(logger.Config{Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	if err != nil {
		t.Fatalf("logger plugin: %v", err)
	}
	srv, err := NewServer(
		WithSchema(schema.Config{Query: middlewareQuery{}, Subscription: middlewareSubscription{}}),
		WithPlugins(logPlug),
		WithPlaygroundPath(""),
		WithGraphQLPath("/api/query"),
		WithSubscriptionPath("/api/sub"),
		WithTransports(websocket.New()),
		WithMiddleware(RequestID("X-Request-ID")),
		WithRequestTimeout(5*time.Minute),
		WithDisableIntrospection(),
		WithMaxHTTPRequestBytes(96<<10),
		WithMaxSelectionDepth(8),
		WithResponseGzip(),
		WithPersistedQueries(map[string]string{}),
		WithOperationAuthorizer(func(ctx context.Context, op OperationContext) error { return nil }),
		WithFieldAuthorizer(func(ctx context.Context, fc FieldAuthorizationContext) error { return nil }),
		WithSchemaSDLPath("/schema.graphql"),
	)
	if err != nil {
		t.Fatalf("new server: %v", err)
	}
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/query", bytes.NewBufferString(`{"query":"{ hello }"}`))
	req.Header.Set("Content-Type", "application/json")
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("GraphQL POST: status %d", rec.Code)
	}
}

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

type deferUser struct {
	ID   string `gql:"id,nonNull"`
	Name string `gql:"name,nonNull"`
}

type userArgs struct {
	ID string `gql:"id,nonNull"`
}

type deferQuery struct{}

func (deferQuery) User(ctx context.Context, args userArgs) (*deferUser, error) {
	return &deferUser{ID: args.ID, Name: "Ada"}, nil
}

func (deferQuery) Numbers(ctx context.Context) ([]int, error) {
	return []int{10, 20, 30}, nil
}

func newDeferServer(t *testing.T) http.Handler {
	t.Helper()
	srv, err := NewServer(WithSchema(schema.Config{Query: deferQuery{}}))
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return srv
}

func TestIncrementalDeliveryDeferOverHTTP(t *testing.T) {
	srv := newDeferServer(t)

	body := `{"query":"{ user(id: \"1\") { id ... on deferUser @defer { name } } }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "multipart/mixed")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "multipart/mixed") {
		t.Fatalf("Content-Type = %q, want multipart/mixed", ct)
	}

	out := rec.Body.String()
	// Initial part carries id and hasNext:true; the deferred part carries name.
	if !strings.Contains(out, `"id":"1"`) {
		t.Fatalf("expected initial id in body:\n%s", out)
	}
	if !strings.Contains(out, `"hasNext":true`) {
		t.Fatalf("expected hasNext:true in body:\n%s", out)
	}
	if !strings.Contains(out, `"name":"Ada"`) {
		t.Fatalf("expected deferred name in body:\n%s", out)
	}
	if !strings.Contains(out, `"hasNext":false`) {
		t.Fatalf("expected terminal hasNext:false in body:\n%s", out)
	}
	if !strings.HasSuffix(strings.TrimRight(out, "\r\n"), "----") {
		t.Fatalf("expected closing boundary, body tail:\n%q", out)
	}
}

func TestIncrementalDeliveryStreamOverHTTP(t *testing.T) {
	srv := newDeferServer(t)

	body := `{"query":"{ numbers @stream(initialCount: 1) }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "multipart/mixed")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	out := rec.Body.String()
	if !strings.Contains(out, `"numbers":[10]`) {
		t.Fatalf("expected initial single item, body:\n%s", out)
	}
	// Streamed items 20 and 30 must each appear in an incremental part.
	if !strings.Contains(out, "20") || !strings.Contains(out, "30") {
		t.Fatalf("expected streamed items 20 and 30, body:\n%s", out)
	}
}

func TestNonIncrementalRequestStaysSingleJSON(t *testing.T) {
	srv := newDeferServer(t)

	// No @defer/@stream: even with multipart/mixed Accept, a normal JSON body
	// is returned.
	body := `{"query":"{ user(id: \"1\") { id name } }"}`
	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "multipart/mixed")
	rec := httptest.NewRecorder()

	srv.ServeHTTP(rec, req)

	if ct := rec.Header().Get("Content-Type"); strings.HasPrefix(ct, "multipart/mixed") {
		t.Fatalf("did not expect multipart for non-incremental op, got %q", ct)
	}
	if !strings.Contains(rec.Body.String(), `"name":"Ada"`) {
		t.Fatalf("expected name inlined in single response: %s", rec.Body.String())
	}
}
