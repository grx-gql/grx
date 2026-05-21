package sse_test

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/patrickkabwe/grx/core"
	grxsse "github.com/patrickkabwe/grx/pkg/sse"
)

type streamingExecutor struct {
	gotReq  core.Request
	source  chan core.Response
	subErr  error
	subOnce sync.Once
}

func newStreamingExecutor() *streamingExecutor {
	return &streamingExecutor{source: make(chan core.Response, 8)}
}

func (e *streamingExecutor) Execute(ctx context.Context, req core.Request) core.Response {
	return core.Response{Data: map[string]any{"executed": true}}
}

func (e *streamingExecutor) OperationKind(req core.Request) (core.OperationKind, error) {
	return core.OperationSubscription, nil
}

func (e *streamingExecutor) Subscribe(ctx context.Context, req core.Request) (<-chan core.Response, error) {
	e.subOnce.Do(func() { e.gotReq = req })
	if e.subErr != nil {
		return nil, e.subErr
	}
	out := make(chan core.Response)
	go func() {
		defer close(out)
		for {
			select {
			case <-ctx.Done():
				return
			case res, open := <-e.source:
				if !open {
					return
				}
				select {
				case <-ctx.Done():
					return
				case out <- res:
				}
			}
		}
	}()
	return out, nil
}

func sseRequest(method, target string, body io.Reader) *http.Request {
	req := httptest.NewRequest(method, target, body)
	req.Header.Set("Accept", "text/event-stream")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return req
}

type sseEvent struct {
	Event string
	Data  string
}

func readSSEEvents(t *testing.T, body io.Reader, count int) []sseEvent {
	t.Helper()
	scanner := bufio.NewScanner(body)
	events := make([]sseEvent, 0, count)
	current := sseEvent{}
	deadline := time.After(5 * time.Second)
	for len(events) < count {
		select {
		case <-deadline:
			t.Fatalf("timed out waiting for SSE events; got %#v", events)
		default:
		}
		if !scanner.Scan() {
			if err := scanner.Err(); err != nil {
				t.Fatalf("scan SSE: %v", err)
			}
			t.Fatalf("EOF waiting for event %d; got %#v", len(events)+1, events)
		}
		line := scanner.Text()
		if line == "" {
			if current.Event != "" || current.Data != "" {
				events = append(events, current)
				current = sseEvent{}
			}
			continue
		}
		if after, ok := strings.CutPrefix(line, "event: "); ok {
			current.Event = after
			continue
		}
		if after, ok := strings.CutPrefix(line, "data: "); ok {
			if current.Data != "" {
				current.Data += "\n" + after
			} else {
				current.Data = after
			}
		}
	}
	return events
}

func TestMatchRequiresEventStreamAccept(t *testing.T) {
	transport := grxsse.New()
	postOK := sseRequest(http.MethodPost, "/graphql", strings.NewReader(`{"query":"subscription { x }"}`))
	if !transport.Match(postOK) {
		t.Fatal("expected Match true for POST with Accept: text/event-stream")
	}

	postNoAccept := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(`{"query":"subscription { x }"}`))
	if transport.Match(postNoAccept) {
		t.Fatal("expected Match false without Accept: text/event-stream")
	}

	getOK := sseRequest(http.MethodGet, "/graphql?query=subscription%20%7Bx%7D", nil)
	if !transport.Match(getOK) {
		t.Fatal("expected Match true for GET with Accept: text/event-stream")
	}

	put := sseRequest(http.MethodPut, "/graphql", nil)
	if transport.Match(put) {
		t.Fatal("expected Match false for PUT")
	}
}

func TestServeStreamsSubscriptionPayloads(t *testing.T) {
	executor := newStreamingExecutor()
	transport := grxsse.New()

	body := strings.NewReader(`{"query":"subscription { userCreated { id } }"}`)
	req := sseRequest(http.MethodPost, "/graphql", body)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		transport.Serve(rec, req, executor)
		close(done)
	}()

	executor.source <- core.Response{Data: map[string]any{"userCreated": map[string]any{"id": "1"}}}
	executor.source <- core.Response{Data: map[string]any{"userCreated": map[string]any{"id": "2"}}}
	close(executor.source)
	<-done

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "text/event-stream" {
		t.Fatalf("Content-Type = %q, want text/event-stream", got)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache, no-transform" {
		t.Fatalf("Cache-Control = %q", got)
	}

	events := readSSEEvents(t, rec.Body, 3)
	if events[0].Event != "next" {
		t.Fatalf("expected first event next, got %q", events[0].Event)
	}
	var first core.Response
	if err := json.Unmarshal([]byte(events[0].Data), &first); err != nil {
		t.Fatalf("decode first event: %v", err)
	}
	data := first.Data.(map[string]any)
	user := data["userCreated"].(map[string]any)
	if user["id"] != "1" {
		t.Fatalf("first id = %v", user["id"])
	}

	if events[2].Event != "complete" {
		t.Fatalf("expected complete event, got %q", events[2].Event)
	}
	if executor.gotReq.Query != "subscription { userCreated { id } }" {
		t.Fatalf("executor.Query = %q", executor.gotReq.Query)
	}
}

func TestServeSupportsGetWithQueryParameters(t *testing.T) {
	executor := newStreamingExecutor()
	transport := grxsse.New()

	values := url.Values{
		"query":     {"subscription { userCreated { id } }"},
		"variables": {`{"limit":3}`},
	}
	req := sseRequest(http.MethodGet, "/graphql?"+values.Encode(), nil)
	rec := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		transport.Serve(rec, req, executor)
		close(done)
	}()

	executor.source <- core.Response{Data: map[string]any{"userCreated": map[string]any{"id": "1"}}}
	close(executor.source)
	<-done

	events := readSSEEvents(t, rec.Body, 2)
	if events[0].Event != "next" || events[1].Event != "complete" {
		t.Fatalf("unexpected events: %#v", events)
	}
	if executor.gotReq.Variables["limit"] != float64(3) {
		t.Fatalf("variables limit = %#v", executor.gotReq.Variables)
	}
}

func TestServeReturns400ForInvalidJSON(t *testing.T) {
	transport := grxsse.New()
	req := sseRequest(http.MethodPost, "/graphql", strings.NewReader(`{"query":`))
	rec := httptest.NewRecorder()

	transport.Serve(rec, req, &streamingExecutor{})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	var decoded core.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(decoded.Errors) != 1 {
		t.Fatalf("errors = %#v", decoded.Errors)
	}
}

func TestServeReturns400ForMissingQueryOnGet(t *testing.T) {
	transport := grxsse.New()
	req := sseRequest(http.MethodGet, "/graphql", nil)
	rec := httptest.NewRecorder()

	transport.Serve(rec, req, &streamingExecutor{})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	var decoded core.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(decoded.Errors) != 1 || decoded.Errors[0].Message != "missing GraphQL query" {
		t.Fatalf("errors = %#v", decoded.Errors)
	}
}

func TestServeEmitsErrorAndCompleteOnSubscribeFailure(t *testing.T) {
	executor := &streamingExecutor{subErr: errors.New("subscribe failed")}
	transport := grxsse.New()

	req := sseRequest(http.MethodPost, "/graphql", strings.NewReader(`{"query":"subscription { x }"}`))
	rec := httptest.NewRecorder()
	transport.Serve(rec, req, executor)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	events := readSSEEvents(t, rec.Body, 2)
	if events[0].Event != "next" {
		t.Fatalf("expected next event, got %q", events[0].Event)
	}
	var res core.Response
	if err := json.Unmarshal([]byte(events[0].Data), &res); err != nil {
		t.Fatalf("decode event: %v", err)
	}
	if len(res.Errors) != 1 || res.Errors[0].Message != "subscribe failed" {
		t.Fatalf("errors = %#v", res.Errors)
	}
	if events[1].Event != "complete" {
		t.Fatalf("expected complete event, got %q", events[1].Event)
	}
}

func TestServeReturns500WhenStreamingUnsupported(t *testing.T) {
	transport := grxsse.New()
	req := sseRequest(http.MethodPost, "/graphql", strings.NewReader(`{"query":"subscription { x }"}`))
	w := &nonFlusherResponseWriter{header: make(http.Header)}

	transport.Serve(w, req, &streamingExecutor{})

	if w.status != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", w.status, http.StatusInternalServerError)
	}
}

type nonFlusherResponseWriter struct {
	header http.Header
	status int
	body   strings.Builder
}

func (w *nonFlusherResponseWriter) Header() http.Header         { return w.header }
func (w *nonFlusherResponseWriter) Write(b []byte) (int, error) { return w.body.Write(b) }
func (w *nonFlusherResponseWriter) WriteHeader(status int)      { w.status = status }

func TestSatisfiesCoreTransport(t *testing.T) {
	var _ core.Transport = grxsse.New()
}
