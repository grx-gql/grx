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

func TestRoadmapGraphQLResponseRequestErrorsUseFailureStatus(t *testing.T) {
	srv, err := New(Config{Schema: schema.Config{Query: sdlTestQuery{}}})
	if err != nil {
		t.Fatal(err)
	}
	for _, query := range []string{"{", "{ missingField }"} {
		t.Run(query, func(t *testing.T) {
			body, err := json.Marshal(core.GraphQLBody{Query: query})
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(string(body)))
			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Accept", core.MediaTypeGraphQLResponse)
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			if rec.Code < 400 || rec.Code >= 500 {
				t.Fatalf("request error returned status %d, want 4xx: %s", rec.Code, rec.Body.String())
			}
		})
	}
}
