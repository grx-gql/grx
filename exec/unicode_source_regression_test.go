package exec

import "testing"

func TestRoadmapLexerRejectsInvalidUTF8Source(t *testing.T) {
	for _, source := range []string{"\"\xff\"", "\"\"\"\xff\"\"\"", "#\xff\n{ name }"} {
		t.Run(source, func(t *testing.T) {
			if _, err := lex(source); err == nil {
				t.Fatal("invalid UTF-8 source accepted")
			}
		})
	}
}
