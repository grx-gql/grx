package core

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestDecodeGraphQLRequestBranches(t *testing.T) {
	post := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(`{"query":"{ ok }","variables":{"id":"1"}}`))
	body, err := DecodeGraphQLRequest(post)
	if err != nil {
		t.Fatalf("decode post: %v", err)
	}
	if body.Query != "{ ok }" || body.Variables["id"] != "1" {
		t.Fatalf("post body = %#v", body)
	}

	get := httptest.NewRequest(http.MethodGet, `/graphql?query=%7B%20ok%20%7D&operationName=Q&variables=%7B%22id%22%3A%221%22%7D`, nil)
	body, err = DecodeGraphQLRequest(get)
	if err != nil {
		t.Fatalf("decode get: %v", err)
	}
	if body.OperationName != "Q" || body.Variables["id"] != "1" {
		t.Fatalf("get body = %#v", body)
	}

	if _, err := DecodeGraphQLRequest(httptest.NewRequest(http.MethodGet, `/graphql`, nil)); err == nil {
		t.Fatal("expected missing GET query error")
	}

	badVars := httptest.NewRequest(http.MethodGet, `/graphql?query=x&variables=not-json`, nil)
	if _, err := DecodeGraphQLRequest(badVars); err == nil {
		t.Fatal("expected invalid variables error")
	}

	patch := httptest.NewRequest(http.MethodPatch, "/graphql", nil)
	if _, err := DecodeGraphQLRequest(patch); err == nil {
		t.Fatal("expected unsupported method error")
	}

	missing := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(`{"variables":{}}`))
	if _, err := DecodeGraphQLRequest(missing); err == nil {
		t.Fatal("expected missing query error")
	}

	invalid := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(`{`))
	if _, err := DecodeGraphQLRequest(invalid); err == nil {
		t.Fatal("expected invalid JSON error")
	}
}

func TestWriteResponseAndHeaderHelpers(t *testing.T) {
	rec := httptest.NewRecorder()
	WriteGraphQLResponse(rec, http.StatusAccepted, MediaTypeGraphQLResponse, map[string]any{"ok": true})
	if rec.Code != http.StatusAccepted {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("Content-Type"); got != MediaTypeGraphQLResponse+"; charset=utf-8" {
		t.Fatalf("content type = %q", got)
	}
	if !HeaderContains([]string{"keep-alive, Upgrade"}, "upgrade") {
		t.Fatal("expected token match")
	}
	if HeaderContains([]string{"keep-alive"}, "upgrade") {
		t.Fatal("unexpected token match")
	}

	if got, ok := NegotiateResponseContentType([]string{"application/json;q=0.5, application/graphql-response+json;q=0.5"}); !ok || got != MediaTypeGraphQLResponse {
		t.Fatalf("negotiated = %q %v", got, ok)
	}
	if got, ok := NegotiateResponseContentType([]string{"multipart/mixed"}); !ok || got != MediaTypeJSON {
		t.Fatalf("multipart negotiated = %q %v", got, ok)
	}
	if _, ok := NegotiateResponseContentType([]string{"text/plain"}); ok {
		t.Fatal("unexpected negotiation")
	}

	req := httptest.NewRequest(http.MethodPost, "/graphql", nil)
	req.Header.Set("Content-Type", "%%%")
	if err := ValidatePostContentType(req); err == nil {
		t.Fatal("expected invalid content type")
	}
}

type failingJSONWriter struct {
	header http.Header
}

func (w *failingJSONWriter) Header() http.Header {
	if w.header == nil {
		w.header = make(http.Header)
	}
	return w.header
}

func (w *failingJSONWriter) WriteHeader(int) {}

func (w *failingJSONWriter) Write([]byte) (int, error) {
	return 0, errors.New("write failed")
}

func TestWriteJSONHandlesEncoderError(t *testing.T) {
	WriteJSON(&failingJSONWriter{}, http.StatusOK, map[string]any{"ok": true})
}
