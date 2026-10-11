//go:build !race

package http

import (
	"net/http/httptest"
	"testing"
)

func TestJSONAcceptSkipsMultipartAllocations(t *testing.T) {
	request := httptest.NewRequest("GET", "/graphql", nil)
	request.Header.Set("Accept", "application/graphql-response+json;q=1, application/json;q=0.8")
	allocations := testing.AllocsPerRun(100, func() {
		if acceptsMultipartMixed(request) {
			t.Fatal("JSON accepted as multipart")
		}
	})
	if allocations != 0 {
		t.Fatalf("JSON header allocated %v multipart objects", allocations)
	}
}

func TestSimpleEncodingNegotiationAvoidsAllocations(t *testing.T) {
	request := httptest.NewRequest("GET", "/graphql", nil)
	request.Header.Set("Accept-Encoding", "gzip, br")
	transport := New(Config{EnableGzip: true, EnableBrotli: true})
	allocations := testing.AllocsPerRun(100, func() {
		encoding, identity := transport.negotiateEncoding(request)
		if encoding != "br" || !identity {
			t.Fatalf("encoding=%q identity=%v", encoding, identity)
		}
	})
	if allocations != 0 {
		t.Fatalf("simple codings allocated %v objects", allocations)
	}
}
