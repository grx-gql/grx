package http_test

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
	"github.com/patrickkabwe/grx/core"
	grxhttp "github.com/patrickkabwe/grx/pkg/http"
)

// stubExecutor is a minimal core.Executor recording the request it received
// and returning a fixed response. The transport never invokes Subscribe or
// OperationKind, so they are intentionally inert.
type stubExecutor struct {
	gotReq   core.Request
	response core.Response
}

func (s *stubExecutor) Execute(ctx context.Context, req core.Request) core.Response {
	s.gotReq = req
	return s.response
}

func (s *stubExecutor) Subscribe(ctx context.Context, req core.Request) (<-chan core.Response, error) {
	return nil, errors.New("not used")
}

func (s *stubExecutor) OperationKind(req core.Request) (core.OperationKind, error) {
	q := strings.TrimSpace(req.Query)
	switch {
	case strings.HasPrefix(q, "mutation"):
		return core.OperationMutation, nil
	case strings.HasPrefix(q, "subscription"):
		return core.OperationSubscription, nil
	default:
		return core.OperationQuery, nil
	}
}

func postGraphQLRequest(path string, body string) *http.Request {
	req := httptest.NewRequest(http.MethodPost, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestMatchAcceptsPostAndGetWithQuery(t *testing.T) {
	transport := grxhttp.New()
	if !transport.Match(httptest.NewRequest(http.MethodPost, "/graphql", nil)) {
		t.Fatal("expected Match true for POST")
	}
	if !transport.Match(httptest.NewRequest(http.MethodGet, "/graphql?query=%7B__typename%7D", nil)) {
		t.Fatal("expected Match true for GET with query parameter")
	}
	if transport.Match(httptest.NewRequest(http.MethodGet, "/graphql", nil)) {
		t.Fatal("expected Match false for GET without query parameter")
	}
	for _, method := range []string{http.MethodPut, http.MethodDelete, http.MethodHead, http.MethodOptions} {
		req := httptest.NewRequest(method, "/graphql", nil)
		if transport.Match(req) {
			t.Fatalf("Match(%s) = true, want false", method)
		}
	}
}

func TestMatchRespectsConfigPathWhenSet(t *testing.T) {
	transport := grxhttp.New(grxhttp.Config{Path: "/api/graphql"})
	reqOK := httptest.NewRequest(http.MethodPost, "/api/graphql", strings.NewReader(`{"query":"{__typename}"}`))
	reqWrong := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(`{"query":"{__typename}"}`))
	if !transport.Match(reqOK) {
		t.Fatal("expected Match true for POST at configured Path")
	}
	if transport.Match(reqWrong) {
		t.Fatal("expected Match false when URL path mismatches configured Path")
	}
}

func TestServeExecutesQueryAndWritesJSON(t *testing.T) {
	executor := &stubExecutor{response: core.Response{Data: map[string]any{"hello": "world"}}}
	transport := grxhttp.New()

	req := postGraphQLRequest("/graphql", `{"query":"{ hello }","variables":{"name":"ada"},"operationName":"Hello"}`)
	rec := httptest.NewRecorder()

	transport.Serve(rec, req, executor)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want application/json; charset=utf-8", got)
	}

	if executor.gotReq.Query != "{ hello }" {
		t.Fatalf("executor.Query = %q", executor.gotReq.Query)
	}
	if executor.gotReq.OperationName != "Hello" {
		t.Fatalf("executor.OperationName = %q", executor.gotReq.OperationName)
	}
	if name, _ := executor.gotReq.Variables["name"].(string); name != "ada" {
		t.Fatalf("executor.Variables[name] = %v", executor.gotReq.Variables["name"])
	}

	var decoded core.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	data, ok := decoded.Data.(map[string]any)
	if !ok {
		t.Fatalf("response.Data type = %T", decoded.Data)
	}
	if data["hello"] != "world" {
		t.Fatalf("response.Data[hello] = %v", data["hello"])
	}
}

func TestServeReturns400ForInvalidJSON(t *testing.T) {
	transport := grxhttp.New()
	req := postGraphQLRequest("/graphql", `{"query":`)
	rec := httptest.NewRecorder()

	transport.Serve(rec, req, &stubExecutor{})

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
	if !strings.Contains(decoded.Errors[0].Message, "invalid GraphQL JSON body") {
		t.Fatalf("error message = %q", decoded.Errors[0].Message)
	}
}

func TestServeExecutesGetQueryWithVariablesAndOperationName(t *testing.T) {
	executor := &stubExecutor{response: core.Response{Data: map[string]any{"hello": "world"}}}
	transport := grxhttp.New()

	req := httptest.NewRequest(http.MethodGet, "/graphql?query=%7B+hello+%7D&operationName=Hello&variables=%7B%22name%22%3A%22ada%22%7D", nil)
	rec := httptest.NewRecorder()

	transport.Serve(rec, req, executor)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json; charset=utf-8" {
		t.Fatalf("Content-Type = %q, want application/json; charset=utf-8", got)
	}
	if executor.gotReq.Query != "{ hello }" {
		t.Fatalf("executor.Query = %q", executor.gotReq.Query)
	}
	if executor.gotReq.OperationName != "Hello" {
		t.Fatalf("executor.OperationName = %q", executor.gotReq.OperationName)
	}
	if name, _ := executor.gotReq.Variables["name"].(string); name != "ada" {
		t.Fatalf("executor.Variables[name] = %v", executor.gotReq.Variables["name"])
	}
}

func TestServeReturns400ForGetInvalidVariablesJSON(t *testing.T) {
	transport := grxhttp.New()
	req := httptest.NewRequest(http.MethodGet, "/graphql?query=%7B__typename%7D&variables=not-json", nil)
	rec := httptest.NewRecorder()

	transport.Serve(rec, req, &stubExecutor{})

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	var decoded core.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(decoded.Errors) != 1 || !strings.Contains(decoded.Errors[0].Message, "invalid GraphQL variables") {
		t.Fatalf("errors = %#v", decoded.Errors)
	}
}

func TestServeReturns415WhenPostMissingContentType(t *testing.T) {
	transport := grxhttp.New()
	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(`{"query":"{__typename}"}`))
	rec := httptest.NewRecorder()

	transport.Serve(rec, req, &stubExecutor{})

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnsupportedMediaType)
	}
}

func TestServeReturns415WhenPostContentTypeUnsupported(t *testing.T) {
	transport := grxhttp.New()
	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(`{"query":"{__typename}"}`))
	req.Header.Set("Content-Type", "text/plain")
	rec := httptest.NewRecorder()

	transport.Serve(rec, req, &stubExecutor{})

	if rec.Code != http.StatusUnsupportedMediaType {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnsupportedMediaType)
	}
}

func TestServeUsesGraphQLResponseMediaTypeWhenAccepted(t *testing.T) {
	executor := &stubExecutor{response: core.Response{Data: map[string]any{"ok": true}}}
	transport := grxhttp.New()

	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(`{"query":"{__typename}"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/graphql-response+json")
	rec := httptest.NewRecorder()

	transport.Serve(rec, req, executor)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/graphql-response+json; charset=utf-8" {
		t.Fatalf("Content-Type = %q", got)
	}
}

func TestServeReturns406WhenAcceptUnsupported(t *testing.T) {
	transport := grxhttp.New()
	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(`{"query":"{__typename}"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "text/html")
	rec := httptest.NewRecorder()

	transport.Serve(rec, req, &stubExecutor{})

	if rec.Code != http.StatusNotAcceptable {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNotAcceptable)
	}
}

func TestServeReturns413WhenPostBodyExceedsMaxRequestBytes(t *testing.T) {
	executor := &stubExecutor{response: core.Response{Data: map[string]any{"ok": true}}}
	transport := grxhttp.New(grxhttp.Config{MaxRequestBytes: 32})

	body := `{"query":"` + strings.Repeat("a", 64) + `"}`
	req := postGraphQLRequest("/graphql", body)
	req.ContentLength = int64(len(body))
	rec := httptest.NewRecorder()

	transport.Serve(rec, req, executor)

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
	if executor.gotReq.Query != "" {
		t.Fatal("executor should not run for oversized POST body")
	}
	var decoded core.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(decoded.Errors) != 1 || !strings.Contains(decoded.Errors[0].Message, "request exceeds") {
		t.Fatalf("errors = %#v", decoded.Errors)
	}
}

func TestServeReturns413WhenGetQueryExceedsMaxRequestBytes(t *testing.T) {
	transport := grxhttp.New(grxhttp.Config{MaxRequestBytes: 16})
	query := strings.Repeat("a", 32)
	req := httptest.NewRequest(http.MethodGet, "/graphql?query="+query, nil)
	rec := httptest.NewRecorder()

	transport.Serve(rec, req, &stubExecutor{})

	if rec.Code != http.StatusRequestEntityTooLarge {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusRequestEntityTooLarge)
	}
}

func TestServeAllowsLargeRequestWhenMaxRequestBytesZero(t *testing.T) {
	executor := &stubExecutor{response: core.Response{Data: map[string]any{"ok": true}}}
	transport := grxhttp.New()

	req := postGraphQLRequest("/graphql", `{"query":"`+strings.Repeat("a", 256)+`"}`)
	rec := httptest.NewRecorder()

	transport.Serve(rec, req, executor)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
}

func TestServeReturns400ForMissingQuery(t *testing.T) {
	transport := grxhttp.New()
	req := postGraphQLRequest("/graphql", `{"variables":{}}`)
	rec := httptest.NewRecorder()

	transport.Serve(rec, req, &stubExecutor{})

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

func TestServeReturns405ForGetMutation(t *testing.T) {
	transport := grxhttp.New()
	req := httptest.NewRequest(http.MethodGet, "/graphql?query=mutation%20%7B%20x%20%7D", nil)
	rec := httptest.NewRecorder()
	transport.Serve(rec, req, &stubExecutor{})

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestServeReturns405ForGetSubscription(t *testing.T) {
	transport := grxhttp.New()
	req := httptest.NewRequest(http.MethodGet, "/graphql?query=subscription%20%7B%20x%20%7D", nil)
	rec := httptest.NewRecorder()
	transport.Serve(rec, req, &stubExecutor{})

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestServeReturns405ForPostSubscription(t *testing.T) {
	transport := grxhttp.New()
	req := postGraphQLRequest("/graphql", `{"query":"subscription s { x }"}`)
	rec := httptest.NewRecorder()
	transport.Serve(rec, req, &stubExecutor{})

	if rec.Code != http.StatusMethodNotAllowed {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusMethodNotAllowed)
	}
}

func TestServeUsesGzipWhenEnabledAndAccepted(t *testing.T) {
	large := map[string]any{"pad": strings.Repeat("x", 128)}
	executor := &stubExecutor{response: core.Response{Data: large}}
	transport := grxhttp.New(grxhttp.Config{EnableGzip: true})

	req := postGraphQLRequest("/graphql", `{"query":"{__typename}"}`)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	transport.Serve(rec, req, executor)

	if rec.Header().Get("Content-Encoding") != "gzip" {
		t.Fatalf("Content-Encoding = %q", rec.Header().Get("Content-Encoding"))
	}
	zr, err := gzip.NewReader(rec.Body)
	if err != nil {
		t.Fatalf("gzip reader: %v", err)
	}
	defer zr.Close()
	decoded, err := io.ReadAll(zr)
	if err != nil {
		t.Fatalf("read gzip: %v", err)
	}
	var payload core.Response
	if err := json.Unmarshal(decoded, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
}

func TestServeUsesBrotliWhenEnabledAndAccepted(t *testing.T) {
	large := map[string]any{"pad": strings.Repeat("x", 128)}
	executor := &stubExecutor{response: core.Response{Data: large}}
	transport := grxhttp.New(grxhttp.Config{EnableBrotli: true})

	req := postGraphQLRequest("/graphql", `{"query":"{__typename}"}`)
	req.Header.Set("Accept-Encoding", "br")
	rec := httptest.NewRecorder()

	transport.Serve(rec, req, executor)

	if rec.Header().Get("Content-Encoding") != "br" {
		t.Fatalf("Content-Encoding = %q", rec.Header().Get("Content-Encoding"))
	}
	decoded, err := io.ReadAll(brotli.NewReader(rec.Body))
	if err != nil {
		t.Fatalf("read brotli: %v", err)
	}
	var payload core.Response
	if err := json.Unmarshal(decoded, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
}

func TestServePrefersBrotliOverGzipWhenBothAccepted(t *testing.T) {
	large := map[string]any{"pad": strings.Repeat("x", 128)}
	executor := &stubExecutor{response: core.Response{Data: large}}
	transport := grxhttp.New(grxhttp.Config{EnableBrotli: true, EnableGzip: true})

	req := postGraphQLRequest("/graphql", `{"query":"{__typename}"}`)
	req.Header.Set("Accept-Encoding", "gzip, br")
	rec := httptest.NewRecorder()

	transport.Serve(rec, req, executor)

	if got := rec.Header().Get("Content-Encoding"); got != "br" {
		t.Fatalf("expected brotli to win, Content-Encoding = %q", got)
	}
}

func TestServeFallsBackToGzipWhenBrotliNotAccepted(t *testing.T) {
	large := map[string]any{"pad": strings.Repeat("x", 128)}
	executor := &stubExecutor{response: core.Response{Data: large}}
	transport := grxhttp.New(grxhttp.Config{EnableBrotli: true, EnableGzip: true})

	req := postGraphQLRequest("/graphql", `{"query":"{__typename}"}`)
	req.Header.Set("Accept-Encoding", "gzip")
	rec := httptest.NewRecorder()

	transport.Serve(rec, req, executor)

	if got := rec.Header().Get("Content-Encoding"); got != "gzip" {
		t.Fatalf("expected gzip fallback, Content-Encoding = %q", got)
	}
}

func multipartUploadRequest(t *testing.T, operations, fileMap string, parts map[string]string) *http.Request {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	if err := mw.WriteField("operations", operations); err != nil {
		t.Fatalf("write operations: %v", err)
	}
	if err := mw.WriteField("map", fileMap); err != nil {
		t.Fatalf("write map: %v", err)
	}
	for name, content := range parts {
		fw, err := mw.CreateFormFile(name, name+".txt")
		if err != nil {
			t.Fatalf("create file part: %v", err)
		}
		if _, err := io.WriteString(fw, content); err != nil {
			t.Fatalf("write file part: %v", err)
		}
	}
	if err := mw.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, "/graphql", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func TestServeMultipartUploadInjectsFileIntoVariables(t *testing.T) {
	executor := &stubExecutor{response: core.Response{Data: map[string]any{"ok": true}}}
	transport := grxhttp.New()

	operations := `{"query":"mutation($file: Upload!){ upload(file: $file) }","variables":{"file":null}}`
	fileMap := `{"0":["variables.file"]}`
	req := multipartUploadRequest(t, operations, fileMap, map[string]string{"0": "hello world"})
	rec := httptest.NewRecorder()

	transport.Serve(rec, req, executor)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	upload, ok := executor.gotReq.Variables["file"].(*core.Upload)
	if !ok {
		t.Fatalf("expected *core.Upload in variables, got %T", executor.gotReq.Variables["file"])
	}
	f, err := upload.Open()
	if err != nil {
		t.Fatalf("open upload: %v", err)
	}
	defer f.Close()
	got, _ := io.ReadAll(f)
	if string(got) != "hello world" {
		t.Fatalf("upload contents = %q", string(got))
	}
}

func TestServeMultipartUploadIntoListVariable(t *testing.T) {
	executor := &stubExecutor{response: core.Response{Data: map[string]any{"ok": true}}}
	transport := grxhttp.New()

	operations := `{"query":"mutation($files: [Upload!]!){ uploadMany(files: $files) }","variables":{"files":[null,null]}}`
	fileMap := `{"0":["variables.files.0"],"1":["variables.files.1"]}`
	req := multipartUploadRequest(t, operations, fileMap, map[string]string{"0": "a", "1": "b"})
	rec := httptest.NewRecorder()

	transport.Serve(rec, req, executor)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	files, ok := executor.gotReq.Variables["files"].([]any)
	if !ok || len(files) != 2 {
		t.Fatalf("expected 2-element files list, got %#v", executor.gotReq.Variables["files"])
	}
	for i, v := range files {
		if _, ok := v.(*core.Upload); !ok {
			t.Fatalf("files[%d] = %T, want *core.Upload", i, v)
		}
	}
}

func TestServeMultipartMissingMapFails(t *testing.T) {
	executor := &stubExecutor{}
	transport := grxhttp.New()

	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	_ = mw.WriteField("operations", `{"query":"{__typename}"}`)
	_ = mw.Close()
	req := httptest.NewRequest(http.MethodPost, "/graphql", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()

	transport.Serve(rec, req, executor)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
	}
}

type incStubExecutor struct {
	stubExecutor
	initial  core.Response
	payloads []core.IncrementalPayload
	hasInc   bool
}

func (s *incStubExecutor) HasIncrementalDirectives(req core.Request) bool { return s.hasInc }

func (s *incStubExecutor) ExecuteIncremental(ctx context.Context, req core.Request) (core.Response, []core.IncrementalPayload) {
	return s.initial, s.payloads
}

func TestServeStreamsIncrementalDeliveryAsMultipartMixed(t *testing.T) {
	hasNext := true
	executor := &incStubExecutor{
		hasInc:  true,
		initial: core.Response{Data: map[string]any{"id": "1"}, HasNext: &hasNext},
		payloads: []core.IncrementalPayload{
			{Path: []any{"user"}, Data: map[string]any{"name": "Ada"}},
			{Path: []any{"items", 1}, Items: []any{42}},
		},
	}
	transport := grxhttp.New()

	req := postGraphQLRequest("/graphql", `{"query":"{ id ... @defer { name } }"}`)
	req.Header.Set("Accept", "multipart/mixed")
	rec := httptest.NewRecorder()

	transport.Serve(rec, req, executor)

	ct := rec.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "multipart/mixed") {
		t.Fatalf("Content-Type = %q, want multipart/mixed", ct)
	}
	body := rec.Body.String()
	if !strings.Contains(body, `"hasNext":true`) {
		t.Fatalf("expected initial hasNext:true in body:\n%s", body)
	}
	if !strings.Contains(body, `"name":"Ada"`) {
		t.Fatalf("expected first deferred payload in body:\n%s", body)
	}
	if !strings.Contains(body, `"hasNext":false`) {
		t.Fatalf("expected terminal hasNext:false in body:\n%s", body)
	}
	if !strings.HasSuffix(strings.TrimRight(body, "\r\n"), "----") {
		t.Fatalf("expected closing boundary, body tail:\n%q", body)
	}
}

func TestServeIgnoresIncrementalWithoutMultipartAccept(t *testing.T) {
	hasNext := true
	executor := &incStubExecutor{
		hasInc:   true,
		initial:  core.Response{Data: map[string]any{"id": "1"}, HasNext: &hasNext},
		payloads: []core.IncrementalPayload{{Path: []any{"user"}, Data: map[string]any{"name": "Ada"}}},
	}
	executor.stubExecutor.response = core.Response{Data: map[string]any{"id": "1"}}
	transport := grxhttp.New()

	// No multipart/mixed in Accept: falls back to a normal single JSON response.
	req := postGraphQLRequest("/graphql", `{"query":"{ id ... @defer { name } }"}`)
	rec := httptest.NewRecorder()

	transport.Serve(rec, req, executor)

	if ct := rec.Header().Get("Content-Type"); strings.HasPrefix(ct, "multipart/mixed") {
		t.Fatalf("did not expect multipart response, got %q", ct)
	}
}

type panicExecutor struct{}

func (panicExecutor) Execute(ctx context.Context, req core.Request) core.Response {
	panic("resolver panic")
}

func (panicExecutor) Subscribe(ctx context.Context, req core.Request) (<-chan core.Response, error) {
	return nil, errors.New("not used")
}

func (panicExecutor) OperationKind(req core.Request) (core.OperationKind, error) {
	return core.OperationQuery, nil
}

func TestServeReturns500WhenExecutePanics(t *testing.T) {
	transport := grxhttp.New()
	req := postGraphQLRequest("/graphql", `{"query":"{__typename}"}`)
	rec := httptest.NewRecorder()

	transport.Serve(rec, req, panicExecutor{})

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusInternalServerError)
	}
	var decoded core.Response
	if err := json.NewDecoder(rec.Body).Decode(&decoded); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(decoded.Errors) != 1 || decoded.Errors[0].Message != "internal server error" {
		t.Fatalf("errors = %#v", decoded.Errors)
	}
}

func TestServeWritesIncrementalDeliveryFields(t *testing.T) {
	hasNext := true
	executor := &stubExecutor{response: core.Response{
		Data:    map[string]any{"user": map[string]any{"id": "1"}},
		HasNext: &hasNext,
		Incremental: []core.IncrementalPayload{
			{Label: "friends-stream", Path: []any{"user", "friends"}},
		},
		Extensions: map[string]any{"requestId": "req_123"},
	}}
	transport := grxhttp.New()

	req := postGraphQLRequest("/graphql", `{"query":"{ user { id } }"}`)
	rec := httptest.NewRecorder()
	transport.Serve(rec, req, executor)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}

	var decoded map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &decoded); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if decoded["hasNext"] != true {
		t.Fatalf("hasNext = %#v", decoded["hasNext"])
	}
	extensions, ok := decoded["extensions"].(map[string]any)
	if !ok || extensions["requestId"] != "req_123" {
		t.Fatalf("extensions = %#v", decoded["extensions"])
	}
	incremental, ok := decoded["incremental"].([]any)
	if !ok || len(incremental) != 1 {
		t.Fatalf("incremental = %#v", decoded["incremental"])
	}
	entry, ok := incremental[0].(map[string]any)
	if !ok || entry["label"] != "friends-stream" {
		t.Fatalf("incremental entry = %#v", incremental[0])
	}
}

func TestServeBatchUnsupportedOperationIncludesRequestClassification(t *testing.T) {
	transport := grxhttp.New()
	req := postGraphQLRequest("/graphql", `[{"query":"mutation { __typename }"},{"query":"{ __typename }"}]`)
	rec := httptest.NewRecorder()
	transport.Serve(rec, req, &stubExecutor{response: core.Response{Data: map[string]any{"__typename": "Query"}}})

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	var batch []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &batch); err != nil {
		t.Fatalf("decode batch: %v", err)
	}
	if len(batch) != 2 {
		t.Fatalf("batch = %#v", batch)
	}
	errors, ok := batch[0]["errors"].([]any)
	if !ok || len(errors) != 1 {
		t.Fatalf("errors = %#v", batch[0]["errors"])
	}
	entry, ok := errors[0].(map[string]any)
	if !ok {
		t.Fatalf("error entry type = %T", errors[0])
	}
	extensions, ok := entry["extensions"].(map[string]any)
	if !ok || extensions["classification"] != "request" {
		t.Fatalf("extensions = %#v", entry["extensions"])
	}
}

// TestSatisfiesCoreTransport ensures the package's exported type continues
// to fulfil the core.Transport contract; a compile error here is the
// earliest signal that the interface drifted.
func TestSatisfiesCoreTransport(t *testing.T) {
	var _ core.Transport = grxhttp.New()
}
