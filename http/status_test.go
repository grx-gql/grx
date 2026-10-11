package http_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/grx-gql/grx/core"
	grxhttp "github.com/grx-gql/grx/http"
)

func TestServeResponseStatusPreservesExecutionResults(t *testing.T) {
	tests := []struct {
		name     string
		response core.Response
	}{
		{name: "success", response: core.Response{Data: map[string]any{"hello": "world"}}},
		{name: "partial data", response: core.Response{Data: map[string]any{"hello": nil}, Errors: []core.Error{{Message: "field failed"}}}},
		{name: "null data", response: core.Response{DataNull: true, Errors: []core.Error{{Message: "field failed"}}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := postGraphQLRequest("/graphql", `{"query":"{ hello }"}`)
			req.Header.Set("Accept", core.MediaTypeGraphQLResponse)
			rec := httptest.NewRecorder()
			grxhttp.New().Serve(rec, req, &stubExecutor{response: tt.response})
			if rec.Code != http.StatusOK {
				t.Fatalf("status=%d, want 200", rec.Code)
			}
		})
	}
}

func TestServeLegacyRequestErrorKeepsSuccessStatus(t *testing.T) {
	req := postGraphQLRequest("/graphql", `{"query":"{"}`)
	req.Header.Set("Accept", core.MediaTypeJSON)
	rec := httptest.NewRecorder()
	grxhttp.New().Serve(rec, req, &stubExecutor{response: core.Response{Errors: []core.Error{{Message: "parse failed"}}}})
	if rec.Code != http.StatusOK {
		t.Fatalf("legacy status=%d, want 200", rec.Code)
	}
}
