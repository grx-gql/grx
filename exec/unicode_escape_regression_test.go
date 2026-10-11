package exec

import "testing"

func TestRoadmapLexerRejectsInvalidUnicodeEscapes(t *testing.T) {
	for _, source := range []string{`"\uDEAD"`, `"\uD800"`, `"\u{110000}"`, `"\u{D800}"`, `"\u{FFFFFFFF}"`} {
		t.Run(source, func(t *testing.T) {
			if tokens, err := lex(source); err == nil {
				t.Fatalf("invalid Unicode escape accepted: %#v", tokens)
			}
		})
	}
}
