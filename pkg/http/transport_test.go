package http_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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
	return core.OperationQuery, nil
}

func TestMatchAcceptsPostOnly(t *testing.T) {
	transport := grxhttp.New()
	cases := map[string]bool{
		http.MethodPost:    true,
		http.MethodGet:     false,
		http.MethodPut:     false,
		http.MethodDelete:  false,
		http.MethodHead:    false,
		http.MethodOptions: false,
	}
	for method, expected := range cases {
		req := httptest.NewRequest(method, "/graphql", nil)
		if got := transport.Match(req); got != expected {
			t.Fatalf("Match(%s) = %v, want %v", method, got, expected)
		}
	}
}

func TestServeExecutesQueryAndWritesJSON(t *testing.T) {
	executor := &stubExecutor{response: core.Response{Data: map[string]any{"hello": "world"}}}
	transport := grxhttp.New()

	body := strings.NewReader(`{"query":"{ hello }","variables":{"name":"ada"},"operationName":"Hello"}`)
	req := httptest.NewRequest(http.MethodPost, "/graphql", body)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()

	transport.Serve(rec, req, executor)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", got)
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
	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(`{"query":`))
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

func TestServeReturns400ForMissingQuery(t *testing.T) {
	transport := grxhttp.New()
	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(`{"variables":{}}`))
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

// TestSatisfiesCoreTransport ensures the package's exported type continues
// to fulfil the core.Transport contract; a compile error here is the
// earliest signal that the interface drifted.
func TestSatisfiesCoreTransport(t *testing.T) {
	var _ core.Transport = grxhttp.New()
}
