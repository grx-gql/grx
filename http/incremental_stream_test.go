package http_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/exec"
	grxhttp "github.com/grx-gql/grx/http"
	"github.com/grx-gql/grx/schema"
)

type incrementalFlushRecorder struct {
	*httptest.ResponseRecorder
	flushed chan string
}

func (w *incrementalFlushRecorder) Flush() {
	w.ResponseRecorder.Flush()
	w.flushed <- w.Body.String()
}

func TestIncrementalHTTPFlushesBeforeDeferredResolverCompletes(t *testing.T) {
	release := make(chan struct{})
	entered := make(chan struct{})
	done := make(chan struct{})
	scalar := &schema.Scalar{TypeName: "String"}
	query := &schema.Object{TypeName: "Query", Fields: map[string]*schema.Field{
		"fast": {Name: "fast", Type: scalar, Resolver: func(context.Context, schema.ResolveParams) (any, error) { return "ready", nil }},
		"slow": {Name: "slow", Type: scalar, Resolver: func(ctx context.Context, _ schema.ResolveParams) (any, error) {
			close(entered)
			select {
			case <-release:
				return "finished", nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		}},
	}}
	executor := exec.New(&schema.Schema{Query: query, Types: map[string]schema.Type{"Query": query, "String": scalar}}, nil)
	req := postGraphQLRequest("/graphql", `{"query":"{ fast ... on Query @defer { slow } }"}`)
	req.Header.Set("Accept", "multipart/mixed")
	recorder := &incrementalFlushRecorder{ResponseRecorder: httptest.NewRecorder(), flushed: make(chan string, 4)}
	go func() { defer close(done); grxhttp.New().Serve(recorder, req, executor) }()
	defer func() { close(release); <-done }()
	select {
	case body := <-recorder.flushed:
		if !strings.Contains(body, `"fast":"ready"`) || strings.Contains(body, `"slow"`) {
			t.Fatalf("initial body = %s", body)
		}
	case <-time.After(time.Second):
		t.Fatal("initial part was not flushed while deferred resolver was blocked")
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("deferred resolver did not start")
	}
}

type incrementalFailWriter struct {
	*httptest.ResponseRecorder
	calls        int
	failAt       int
	failure      error
	short        bool
	flushFailure error
}

func (w *incrementalFailWriter) Write(body []byte) (int, error) {
	w.calls++
	if w.calls == w.failAt {
		if w.short {
			return len(body) - 1, nil
		}
		return 0, w.failure
	}
	return w.ResponseRecorder.Write(body)
}
func (w *incrementalFailWriter) WriteString(body string) (int, error) { return w.Write([]byte(body)) }

func (w *incrementalFailWriter) FlushError() error { return w.flushFailure }

func TestIncrementalHTTPStopsOnWriterFailure(t *testing.T) {
	for _, tc := range []struct {
		name       string
		failAt     int
		short      bool
		flushError bool
		wantCalls  int
	}{
		{"initial", 1, false, false, 0}, {"patch", 2, false, false, 1}, {"short write", 1, true, false, 0}, {"flush", 0, false, true, 0}, {"closing", 4, false, false, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			scalar := &schema.Scalar{TypeName: "String"}
			root := &schema.Object{TypeName: "Query", Fields: map[string]*schema.Field{}}
			for _, name := range []string{"one", "two"} {
				root.Fields[name] = &schema.Field{Name: name, Type: scalar, Resolver: func(context.Context, schema.ResolveParams) (any, error) { calls++; return "ok", nil }}
			}
			executor := exec.New(&schema.Schema{Query: root, Types: map[string]schema.Type{"Query": root, "String": scalar}}, nil)
			req := postGraphQLRequest("/graphql", `{"query":"{ ... on Query @defer { one } ... on Query @defer { two } }"}`)
			req.Header.Set("Accept", "multipart/mixed")
			failure := errors.New("client disconnected")
			writer := &incrementalFailWriter{ResponseRecorder: httptest.NewRecorder(), failAt: tc.failAt, failure: failure, short: tc.short}
			if tc.flushError {
				writer.flushFailure = failure
			}
			grxhttp.New().Serve(writer, req, executor)
			if calls != tc.wantCalls {
				t.Fatalf("resolved fields=%d; want %d", calls, tc.wantCalls)
			}
			if strings.Contains(writer.Body.String(), "\r\n-----\r\n") {
				t.Fatal("closing boundary after failure")
			}
		})
	}
}

func TestIncrementalHTTPValidationErrorStatus(t *testing.T) {
	scalar := &schema.Scalar{TypeName: "String"}
	root := &schema.Object{TypeName: "Query", Fields: map[string]*schema.Field{"ok": {Name: "ok", Type: scalar}}}
	executor := exec.New(&schema.Schema{Query: root, Types: map[string]schema.Type{"Query": root, "String": scalar}}, nil)
	for _, accept := range []string{"multipart/mixed", "multipart/mixed, application/graphql-response+json"} {
		t.Run(accept, func(t *testing.T) {
			req := postGraphQLRequest("/graphql", `{"query":"{ ... on Query @defer { unknown } }"}`)
			req.Header.Set("Accept", accept)
			recorder := httptest.NewRecorder()
			grxhttp.New().Serve(recorder, req, executor)
			if recorder.Code != http.StatusBadRequest || !strings.Contains(recorder.Body.String(), `"errors"`) {
				t.Fatalf("status=%d body=%s", recorder.Code, recorder.Body)
			}
			if strings.Contains(accept, "graphql-response") && !strings.Contains(recorder.Header().Get("Content-Type"), "graphql-response") {
				t.Fatalf("content type=%s", recorder.Header().Get("Content-Type"))
			}
		})
	}
}

func TestIncrementalHTTPBufferedExecutorStopsAfterFailedWrite(t *testing.T) {
	executor := &incStubExecutor{hasInc: true, initial: core.Response{Data: map[string]any{"ok": true}}, payloads: []core.IncrementalPayload{{Data: map[string]any{"one": true}}, {Data: map[string]any{"two": true}}}}
	req := postGraphQLRequest("/graphql", `{"query":"{ ok }"}`)
	req.Header.Set("Accept", "multipart/mixed")
	writer := &incrementalFailWriter{ResponseRecorder: httptest.NewRecorder(), failAt: 2, failure: errors.New("closed")}
	grxhttp.New().Serve(writer, req, executor)
	if writer.calls != 2 {
		t.Fatalf("writes=%d; want 2", writer.calls)
	}
}
