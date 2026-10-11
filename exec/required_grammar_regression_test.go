package exec

import "testing"

func TestRoadmapParserRejectsEmptyRequiredGrammarLists(t *testing.T) {
	for _, source := range []string{`{}`, `query Q() { name }`, `{ name() }`, `{ name @skip() }`} {
		t.Run(source, func(t *testing.T) {
			if _, err := parseDocument(source, nil); err == nil {
				t.Fatal("invalid empty grammar list accepted")
			}
		})
	}
}
