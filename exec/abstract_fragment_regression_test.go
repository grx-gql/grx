package exec

import "testing"

func TestRoadmapValidationAbstractFragmentRules(t *testing.T) {
	s, err := ParseSDL(`
		interface Node { id: ID }
		type User implements Node { id: ID name: String }
		type Other { id: ID }
		type Unrelated { id: ID }
		union Search = User | Other
 union OtherSearch = Unrelated
		type Query { node: Node result: Search unrelated: Unrelated }
	`)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name      string
		query     string
		wantError bool
	}{
		{name: "valid concrete spread within interface", query: `{ node { ... on User { name } } }`, wantError: false},
		{name: "invalid field inside union fragment", query: `{ result { ... on Search { ... on User { missing } } } }`, wantError: true},
		{name: "unrelated object spread inside union", query: `{ result { ... on Unrelated { id } } }`, wantError: true},
		{name: "interface fragment inside union", query: `{ result { ... on Node { id } } }`, wantError: false},
		{name: "union fragment inside object", query: `{ node { ... on User { ... on Search { ... on User { name } } } } }`, wantError: false},
		{name: "disjoint unions", query: `{ result { ... on OtherSearch { __typename } } }`, wantError: true},
		{name: "interface direct field", query: `{ node { id } }`, wantError: false},
		{name: "union direct field", query: `{ result { id } }`, wantError: true},
		{name: "cycle through union fragment", query: `{ result { ...F } } fragment F on Search { ...F }`, wantError: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			bundle, err := parseDocumentBundle(tc.query, nil, 0)
			if err != nil {
				t.Fatal(err)
			}
			op, err := selectOperation(bundle, "")
			if err != nil {
				t.Fatal(err)
			}
			errs := ValidateDocument(s, bundle, op)
			if (len(errs) > 0) != tc.wantError {
				t.Fatalf("validation errors = %v, wantError = %v", errs, tc.wantError)
			}
		})
	}
}
