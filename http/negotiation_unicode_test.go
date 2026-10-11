package http

import (
	"net/http/httptest"
	"testing"
)

func TestEncodingRejectsNonASCIIFoldEquivalent(t *testing.T) {
	request := httptest.NewRequest("GET", "/graphql", nil)
	request.Header.Set("Accept-Encoding", "GZİP")
	encoding, identity := New(Config{EnableGzip: true}).negotiateEncoding(request)
	if encoding != "" || !identity {
		t.Fatalf("invalid coding accepted: %q %v", encoding, identity)
	}
}
