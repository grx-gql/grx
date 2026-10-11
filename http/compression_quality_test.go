package http_test

import (
	"github.com/grx-gql/grx/core"
	grxhttp "github.com/grx-gql/grx/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRoadmapCompressionAcceptsQualityValues(t *testing.T) {
	tests := []struct {
		name   string
		config grxhttp.Config
		accept string
		want   string
	}{
		{name: "gzip", config: grxhttp.Config{EnableGzip: true}, accept: "gzip;q=1", want: "gzip"},
		{name: "brotli", config: grxhttp.Config{EnableBrotli: true}, accept: "br;q=1", want: "br"},
		{name: "prefer highest quality", config: grxhttp.Config{EnableGzip: true, EnableBrotli: true}, accept: "br;q=0.2,gzip;q=0.8", want: "gzip"},
		{name: "tie", config: grxhttp.Config{EnableGzip: true, EnableBrotli: true}, accept: "gzip;q=0.8,br;q=0.8", want: "br"},
		{name: "explicit exclusion", config: grxhttp.Config{EnableGzip: true, EnableBrotli: true}, accept: "*;q=1,br;q=0", want: "gzip"},
		{name: "excluded", config: grxhttp.Config{EnableGzip: true}, accept: "gzip;q=0"},
		{name: "invalid quality", config: grxhttp.Config{EnableGzip: true}, accept: "gzip;q=invalid"},
		{name: "out of range", config: grxhttp.Config{EnableGzip: true}, accept: "gzip;q=2"},
		{name: "nonfinite", config: grxhttp.Config{EnableGzip: true}, accept: "gzip;q=NaN"},
		{name: "invalid parameter", config: grxhttp.Config{EnableGzip: true}, accept: "gzip;q"},
		{name: "wildcard", config: grxhttp.Config{EnableBrotli: true}, accept: "*;q=0.5", want: "br"},
		{name: "omitted", config: grxhttp.Config{EnableGzip: true}},
		{name: "prefer identity", config: grxhttp.Config{EnableGzip: true}, accept: "identity;q=1,gzip;q=0.5"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			transport := grxhttp.New(tt.config)
			executor := &stubExecutor{response: core.Response{Data: map[string]any{"hello": strings.Repeat("world", 40)}}}
			req := postGraphQLRequest("/graphql", `{"query":"{ hello }"}`)
			req.Header.Set("Accept-Encoding", tt.accept)
			rec := httptest.NewRecorder()
			transport.Serve(rec, req, executor)
			if rec.Header().Get("Vary") != "Accept-Encoding" {
				t.Fatalf("missing Vary: %v", rec.Header())
			}
			if got := rec.Header().Get("Content-Encoding"); got != tt.want {
				t.Fatalf("encoding=%q, want %q", got, tt.want)
			}
		})
	}
}

func TestCompressionIdentityExclusion(t *testing.T) {
	for _, tc := range []struct {
		accept   string
		size     int
		status   int
		encoding string
	}{
		{"gzip;q=0,identity;q=0", 100, 406, ""},
		{"gzip;q=1,identity;q=0", 1, 200, "gzip"},
		{"*;q=0", 100, 406, ""},
		{"*;q=0,identity;q=1", 100, 200, ""},
	} {
		transport := grxhttp.New(grxhttp.Config{EnableGzip: true})
		req := postGraphQLRequest("/graphql", `{"query":"{ hello }"}`)
		req.Header.Set("Accept-Encoding", tc.accept)
		rec := httptest.NewRecorder()
		transport.Serve(rec, req, &stubExecutor{response: core.Response{Data: strings.Repeat("x", tc.size)}})
		if rec.Code != tc.status || rec.Header().Get("Content-Encoding") != tc.encoding {
			t.Errorf("%s size=%d: status=%d encoding=%s", tc.accept, tc.size, rec.Code, rec.Header().Get("Content-Encoding"))
		}
	}
}
