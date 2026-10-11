//go:build !race

package http

import (
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/grx-gql/grx/core"
)

type responseAllocationWriter struct{ header nethttp.Header }

func (w *responseAllocationWriter) Header() nethttp.Header       { return w.header }
func (*responseAllocationWriter) WriteHeader(int)                {}
func (*responseAllocationWriter) Write(body []byte) (int, error) { return len(body), nil }

func TestOrdinaryResponseAvoidsOuterEncodingAllocations(t *testing.T) {
	object := core.NewOrderedObject(1)
	object.Set("hello", "world")
	response := core.Response{Data: object}
	writer := &responseAllocationWriter{header: make(nethttp.Header)}
	request := httptest.NewRequest(nethttp.MethodGet, "/graphql", nil)
	transport := New(Config{})
	allocations := testing.AllocsPerRun(100, func() { transport.writeGraphQLPayload(writer, request, nethttp.StatusOK, core.MediaTypeJSON, response) })
	t.Logf("ordinary response allocations: %v", allocations)
	// One owned JSON result, the content-type string, and its header value slice.
	if allocations > 3 {
		t.Fatalf("ordinary response allocated %v objects; want at most 3", allocations)
	}
}
