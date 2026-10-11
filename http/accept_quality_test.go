package http_test

import (
	"github.com/grx-gql/grx/core"
	grxhttp "github.com/grx-gql/grx/http"
	"net/http/httptest"
	"testing"
)

func TestRoadmapRejectsUnacceptableResponseMediaType(t *testing.T) {
	transport := grxhttp.New()
	executor := &stubExecutor{response: core.Response{Data: map[string]any{"hello": "world"}}}
	req := postGraphQLRequest("/graphql", `{"query":"{ hello }"}`)
	req.Header.Set("Accept", "application/graphql-response+json;q=0")
	rec := httptest.NewRecorder()
	transport.Serve(rec, req, executor)
	if rec.Code != 406 {
		t.Fatalf("excluded media type accepted: status=%d content-type=%q", rec.Code, rec.Header().Get("Content-Type"))
	}
}
