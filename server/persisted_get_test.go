package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/schema"
)

func TestRoadmapPersistedQueryGET(t *testing.T) {
	const query = "{ hello }"
	sum := sha256.Sum256([]byte(query))
	hash := hex.EncodeToString(sum[:])
	srv, err := New(Config{Schema: schema.Config{Query: sdlTestQuery{}}, PersistedQueries: map[string]string{hash: query}, RequirePersistedQuery: true})
	if err != nil {
		t.Fatal(err)
	}
	params := url.Values{"extensions": {`{"persistedQuery":{"version":1,"sha256Hash":"` + hash + `"}}`}}
	req := httptest.NewRequest(http.MethodGet, "/graphql?"+params.Encode(), nil)
	rec := httptest.NewRecorder()
	srv.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("persisted GET status=%d body=%s", rec.Code, rec.Body.String())
	}
	var response core.Response
	if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
		t.Fatal(err)
	}
	data, ok := response.Data.(map[string]any)
	if len(response.Errors) != 0 || !ok || data["hello"] != "hi" {
		t.Fatalf("persisted GET response=%+v", response)
	}
}
