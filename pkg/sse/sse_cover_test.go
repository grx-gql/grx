package sse

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestReadRequestRejectsUnsupportedMethod(t *testing.T) {
	req := httptest.NewRequest(http.MethodPut, "/", nil)
	_, err := readRequest(req)
	if err == nil || !strings.Contains(err.Error(), "not allowed") {
		t.Fatalf("expected method error, got %v", err)
	}
}

func TestWriteEventFallsBackOnMarshalError(t *testing.T) {
	unencodable := map[string]any{"x": make(chan int)}
	rec := httptest.NewRecorder()
	writeEvent(rec, "next", unencodable)
	body := rec.Body.String()
	if !strings.Contains(body, "failed to encode payload") {
		t.Fatalf("unexpected SSE body:\n%s", body)
	}
}

func TestWriteEventEmptyEventName(t *testing.T) {
	rec := httptest.NewRecorder()
	writeEvent(rec, "", map[string]string{"ping": "pong"})
	raw := strings.TrimSpace(rec.Body.String())
	if strings.Contains(raw, "event:") {
		t.Fatalf("empty event must omit event line\n%s", raw)
	}
	if !strings.Contains(raw, `"pong"`) {
		t.Fatalf("expected ping/pong payload\n%s", raw)
	}
}
