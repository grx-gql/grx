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

func TestMultipartAcceptDoesNotAuthorizeJSON(t *testing.T) {
	for _, body := range []string{`{"query":"{ hello }"}`, `[{"query":"{ hello }"}]`} {
		req := postGraphQLRequest("/graphql", body)
		req.Header.Set("Accept", "application/json;q=0,application/graphql-response+json;q=0,multipart/mixed;q=1")
		rec := httptest.NewRecorder()
		grxhttp.New().Serve(rec, req, &stubExecutor{})
		if rec.Code != 406 {
			t.Errorf("excluded JSON returned: %d %s", rec.Code, rec.Body.String())
		}
	}
}
