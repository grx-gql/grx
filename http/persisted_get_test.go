package http_test

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	grxhttp "github.com/grx-gql/grx/http"
)

func TestPersistedGetRetainsRequestGuards(t *testing.T) {
	const mutation = "mutation { update }"
	hash := fmt.Sprintf("%x", sha256.Sum256([]byte(mutation)))
	extensions := fmt.Sprintf(`{"persistedQuery":{"version":1,"sha256Hash":%q}}`, hash)
	tests := []struct {
		name       string
		extensions string
		config     grxhttp.Config
		status     int
	}{
		{name: "mutation", extensions: extensions, config: grxhttp.Config{PersistedQueries: map[string]string{hash: mutation}}, status: http.StatusMethodNotAllowed},
		{name: "malformed extensions", extensions: "{", status: http.StatusBadRequest},
		{name: "wrong extension shape", extensions: "[]", status: http.StatusBadRequest},
		{name: "missing hash", extensions: `{}`, status: http.StatusBadRequest},
		{name: "extension size", extensions: extensions, config: grxhttp.Config{MaxRequestBytes: 32}, status: http.StatusRequestEntityTooLarge},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/graphql?"+url.Values{"extensions": {tt.extensions}}.Encode(), nil)
			transport := grxhttp.New(tt.config)
			if !transport.Match(req) {
				t.Fatal("persisted GET did not match")
			}
			rec := httptest.NewRecorder()
			executor := &stubExecutor{}
			transport.Serve(rec, req, executor)
			if rec.Code != tt.status || executor.gotReq.Query != "" {
				t.Fatalf("status=%d body=%s executed=%q", rec.Code, rec.Body.String(), executor.gotReq.Query)
			}
		})
	}
}
