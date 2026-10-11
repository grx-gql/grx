package http_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/grx-gql/grx/core"
	grxhttp "github.com/grx-gql/grx/http"
)

func BenchmarkHTTPTransport(b *testing.B) {
	for _, scenario := range []struct{ name, method, accept, encoding string }{
		{"post_default", http.MethodPost, "", ""},
		{"post_json", http.MethodPost, "application/json", ""},
		{"post_weighted", http.MethodPost, "application/graphql-response+json;q=1, application/json;q=0.8", ""},
		{"get_json", http.MethodGet, "application/json", ""},
		{"post_accept_encoding", http.MethodPost, "application/json", "gzip, br"},
	} {
		b.Run(scenario.name, func(b *testing.B) {
			executor := &stubExecutor{response: core.Response{Data: map[string]any{"hello": "world"}}}
			transport := grxhttp.New()
			target := "/graphql"
			if scenario.method == http.MethodGet {
				target += "?query=%7Bhello%7D"
			}
			request := httptest.NewRequest(scenario.method, target, nil)
			request.Header.Set("Content-Type", "application/json")
			if scenario.accept != "" {
				request.Header.Set("Accept", scenario.accept)
			}
			if scenario.encoding != "" {
				request.Header.Set("Accept-Encoding", scenario.encoding)
			}
			const body = `{"query":"{hello}"}`
			b.ReportAllocs()
			b.ResetTimer()
			for b.Loop() {
				if scenario.method == http.MethodPost {
					request.Body = io.NopCloser(strings.NewReader(body))
					request.ContentLength = int64(len(body))
				}
				response := httptest.NewRecorder()
				transport.Serve(response, request, executor)
				if response.Code != http.StatusOK {
					b.Fatalf("status=%d body=%s", response.Code, response.Body.String())
				}
			}
		})
	}
}
