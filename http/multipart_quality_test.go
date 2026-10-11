package http

import (
	"net/http/httptest"
	"testing"
)

func TestMultipartQualityExclusions(t *testing.T) {
	for _, accept := range []string{"multipart/mixed;q=0", "multipart/mixed;q=invalid", "multipart/mixed;q=NaN", "multipart/mixed;q=2", "multipart/mixed;q"} {
		req := httptest.NewRequest("POST", "/graphql", nil)
		req.Header.Set("Accept", accept+",application/json")
		if acceptsMultipartMixed(req) {
			t.Errorf("accepted %s", accept)
		}
	}
	req := httptest.NewRequest("POST", "/graphql", nil)
	req.Header.Set("Accept", "multipart/mixed;q=0.5")
	if !acceptsMultipartMixed(req) {
		t.Fatal("rejected positive quality")
	}
}
