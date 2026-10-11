package exec

import "testing"

func TestRoadmapLexerAllowsBOMBetweenTokens(t *testing.T) {
	if _, err := lex("{\ufeff name }"); err != nil {
		t.Fatalf("BOM is an ignored token: %v", err)
	}
}
