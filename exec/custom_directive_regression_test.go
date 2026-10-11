package exec

import "testing"

func TestRoadmapValidationAllowsRegisteredExecutableDirective(t *testing.T) {
	s, err := ParseSDL(`directive @audit repeatable on FIELD type Query { name: String }`)
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := parseDocumentBundle(`{ name @audit @audit }`, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	op, err := selectOperation(bundle, "")
	if err != nil {
		t.Fatal(err)
	}
	if errs := ValidateDocument(s, bundle, op); len(errs) > 0 {
		t.Fatalf("registered repeatable executable directive rejected: %v", errs)
	}
}

func TestCustomDirectiveValidationRules(t *testing.T) {
	s, err := ParseSDL(`directive @audit(value: Int!) on FIELD directive @optional(value: Int! = 1) on FIELD type Query { name: String }`)
	if err != nil {
		t.Fatal(err)
	}
	s.DirectiveDefinitions["optional"].Args[0].DefaultValue = 1
	for _, tc := range []struct {
		query   string
		invalid bool
	}{
		{`{ name @audit(value: 1) }`, false},
		{`{ name @optional }`, false},
		{`{ name @audit }`, true},
		{`{ name @audit(value: "wrong") }`, true},
		{`{ name @audit(value: 1, unknown: 2) }`, true},
		{`{ name @audit(value: 1) @audit(value: 2) }`, true},
		{`{ ... on Query @audit(value: 1) { name } }`, true},
	} {
		bundle, err := parseDocumentBundle(tc.query, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		op, err := selectOperation(bundle, "")
		if err != nil {
			t.Fatal(err)
		}
		if errs := ValidateDocument(s, bundle, op); (len(errs) > 0) != tc.invalid {
			t.Errorf("%s: errors=%v", tc.query, errs)
		}
	}
}
