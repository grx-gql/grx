package core

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestDecodeGraphQLParametersPreservesExtensions(t *testing.T) {
	body, err := DecodeGraphQLParameters(url.Values{
		"extensions":    {`{"persistedQuery":{"version":1,"sha256Hash":"hash"}}`},
		"variables":     {`{"id":7}`},
		"operationName": {"Lookup"},
	})
	if err != nil {
		t.Fatal(err)
	}
	persisted, ok := body.Extensions["persistedQuery"].(map[string]any)
	if !ok || persisted["sha256Hash"] != "hash" || body.Variables["id"] != float64(7) || body.OperationName != "Lookup" || body.Query != "" {
		t.Fatalf("decoded parameters=%+v", body)
	}
}

func TestDecodeGraphQLRequestRejectsInvalidGetExtensions(t *testing.T) {
	for _, raw := range []string{"{", "[]", "true"} {
		req := httptest.NewRequest(http.MethodGet, "/graphql?"+url.Values{"query": {"{ hello }"}, "extensions": {raw}}.Encode(), nil)
		if _, err := DecodeGraphQLRequest(req); err == nil {
			t.Fatalf("invalid extensions accepted: %q", raw)
		}
	}
}
