package core

import (
	"net/http/httptest"
	"testing"
)

func TestValidatePostContentType(t *testing.T) {
	t.Run("missing", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/graphql", nil)
		if err := ValidatePostContentType(req); err == nil {
			t.Fatal("expected error for missing Content-Type")
		}
	})

	t.Run("application/json", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/graphql", nil)
		req.Header.Set("Content-Type", "application/json")
		if err := ValidatePostContentType(req); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("application/json with charset", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/graphql", nil)
		req.Header.Set("Content-Type", "application/json; charset=utf-8")
		if err := ValidatePostContentType(req); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
	})

	t.Run("unsupported", func(t *testing.T) {
		req := httptest.NewRequest("POST", "/graphql", nil)
		req.Header.Set("Content-Type", "text/plain")
		if err := ValidatePostContentType(req); err == nil {
			t.Fatal("expected error for unsupported Content-Type")
		}
	})
}

func TestNegotiateResponseContentType(t *testing.T) {
	cases := []struct {
		name   string
		accept []string
		want   string
		wantOK bool
	}{
		{
			name:   "missing accept defaults to legacy json",
			accept: nil,
			want:   MediaTypeJSON,
			wantOK: true,
		},
		{
			name:   "graphql response only",
			accept: []string{"application/graphql-response+json"},
			want:   MediaTypeGraphQLResponse,
			wantOK: true,
		},
		{
			name:   "prefers graphql response over json",
			accept: []string{"application/graphql-response+json, application/json;q=0.9"},
			want:   MediaTypeGraphQLResponse,
			wantOK: true,
		},
		{
			name:   "json only",
			accept: []string{"application/json"},
			want:   MediaTypeJSON,
			wantOK: true,
		},
		{
			name:   "wildcard",
			accept: []string{"*/*"},
			want:   MediaTypeGraphQLResponse,
			wantOK: true,
		},
		{
			name:   "unsupported",
			accept: []string{"text/html"},
			wantOK: false,
		},
	}

	cases = append(cases, []struct {
		name   string
		accept []string
		want   string
		wantOK bool
	}{
		{"excluded", []string{"application/graphql-response+json;q=0"}, "", false},
		{"exact overrides wildcard", []string{"application/graphql-response+json;q=0,*/*;q=1"}, MediaTypeJSON, true},
		{"both excluded", []string{"application/graphql-response+json;q=0,application/json;q=0,*/*;q=1"}, "", false},
		{"subtype wildcard", []string{"application/*;q=0.8,application/graphql-response+json;q=0.2"}, MediaTypeJSON, true},
		{"multiline", []string{"application/json;q=0.9", "application/graphql-response+json;q=0.1"}, MediaTypeJSON, true},
		{"invalid quality", []string{"application/json;q=bad"}, "", false},
		{"out of range", []string{"application/json;q=1.1"}, "", false},
		{"nonfinite", []string{"application/json;q=NaN"}, "", false},
		{"bad params", []string{"application/json;q"}, "", false},
		{"multipart excluded", []string{"multipart/mixed;q=0"}, "", false},
		{"tie", []string{"application/json;q=0.5,application/graphql-response+json;q=0.5"}, MediaTypeGraphQLResponse, true},
	}...)

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, ok := NegotiateResponseContentType(tc.accept)
			if ok != tc.wantOK {
				t.Fatalf("ok = %v, want %v", ok, tc.wantOK)
			}
			if ok && got != tc.want {
				t.Fatalf("media type = %q, want %q", got, tc.want)
			}
		})
	}
}
