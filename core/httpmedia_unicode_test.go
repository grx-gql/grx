package core

import "testing"

func TestAcceptRejectsNonASCIIFoldEquivalent(t *testing.T) {
	if _, ok := NegotiateResponseContentType([]string{"application/jſon"}); ok {
		t.Fatal("accepted non-ASCII media token")
	}
}
