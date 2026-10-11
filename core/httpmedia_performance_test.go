//go:build !race

package core

import "testing"

func TestCommonAcceptHeadersAvoidAllocations(t *testing.T) {
	for _, header := range []string{MediaTypeJSON, MediaTypeGraphQLResponse, "*/*", "application/*", " APPLICATION/JSON "} {
		t.Run(header, func(t *testing.T) {
			accept := []string{header}
			allocations := testing.AllocsPerRun(100, func() {
				if _, ok := NegotiateResponseContentType(accept); !ok {
					t.Fatal("valid media type rejected")
				}
			})
			if allocations != 0 {
				t.Fatalf("common header allocated %v objects", allocations)
			}
		})
	}
}
