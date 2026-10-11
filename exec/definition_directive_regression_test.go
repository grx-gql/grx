package exec

import "testing"

func TestRejectInvalidDefinitionDirectives(t *testing.T) {
	e := newValExecutor(t)
	for _, query := range []string{
		`query Q @unknown { search(term: "ok") }`,
		`query Q @skip(if: true) { search(term: "ok") }`,
		`query Q($term: String! = "ok" @unknown) { search(term: $term) }`,
		`{ ...F } fragment F on Query @unknown { search(term: "ok") }`,
	} {
		if errs := valErrors(t, e, query); len(errs) == 0 {
			t.Errorf("invalid definition directive accepted: %s", query)
		}
	}
}

func TestAllowRegisteredDefinitionDirectives(t *testing.T) {
	s, err := ParseSDL(`directive @audit on QUERY | VARIABLE_DEFINITION | FRAGMENT_DEFINITION type Query { name(arg:String):String }`)
	if err != nil {
		t.Fatal(err)
	}
	query := `query Q($arg:String @audit) @audit { ...F } fragment F on Query @audit { name(arg:$arg) }`
	bundle, err := parseDocumentBundle(query, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	op, err := selectOperation(bundle, "")
	if err != nil {
		t.Fatal(err)
	}
	if errs := ValidateDocument(s, bundle, op); len(errs) > 0 {
		t.Fatal(errs)
	}
}
