package server

import (
	"encoding/json"
	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/schema"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoadmapSingletonBatchPreservesArrayResponse(t *testing.T) {
	srv, err := New(Config{Schema: schema.Config{Query: sdlTestQuery{}}})
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(`[{"query":"{ hello }"}]`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	var responses []core.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &responses); err != nil {
		t.Fatalf("batch must return an array: %v; body=%s", err, rec.Body.String())
	}
	if len(responses) != 1 || len(responses[0].Errors) != 0 {
		t.Fatalf("unexpected response: %+v", responses)
	}
}
