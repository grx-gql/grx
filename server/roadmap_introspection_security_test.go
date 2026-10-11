package server

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/exec"
	"github.com/grx-gql/grx/schema"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoadmapIntrospectionEnforcesOperationSecurity(t *testing.T) {
	const trustedQuery = "{ hello }"
	sum := sha256.Sum256([]byte(trustedQuery))
	tests := []struct {
		name   string
		config Config
	}{
		{name: "authorization", config: Config{OperationAuthorizer: func(context.Context, exec.OperationContext) error { return errors.New("denied") }}},
		{name: "rate limit", config: Config{RateLimiter: func(context.Context, exec.OperationContext) error { return errors.New("rate limited") }}},
		{name: "trusted documents", config: Config{TrustedDocuments: map[string]string{hex.EncodeToString(sum[:]): trustedQuery}}},
		{name: "selection limit", config: Config{MaxSelectionCount: 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := tt.config
			config.Schema = schema.Config{Query: sdlTestQuery{}}
			srv, err := New(config)
			if err != nil {
				t.Fatal(err)
			}
			req := httptest.NewRequest(http.MethodPost, "/graphql", strings.NewReader(`{"query":"{ __schema { queryType { name } } }"}`))
			req.Header.Set("Content-Type", "application/json")
			rec := httptest.NewRecorder()
			srv.ServeHTTP(rec, req)
			var response core.Response
			if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			if len(response.Errors) == 0 || response.Data != nil {
				t.Fatalf("introspection bypassed %s: errors=%v dataPresent=%t", tt.name, response.Errors, response.Data != nil)
			}
		})
	}
}
