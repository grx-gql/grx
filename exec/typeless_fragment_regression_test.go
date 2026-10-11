package exec

import "testing"

func TestRoadmapParserAllowsTypelessInlineFragmentDirective(t *testing.T) {
	if _, err := parseDocument(`{ ... @skip(if: true) { name } }`, nil); err != nil {
		t.Fatalf("valid inline fragment rejected: %v", err)
	}
}
