package exec

import (
	"context"
	"github.com/grx-gql/grx/core"
	"testing"
)

func TestRejectIncompatibleVariablePositions(t *testing.T) {
	e := newValExecutor(t)
	for _, query := range []string{
		`query Q($term: String) { search(term:$term) }`,
		`query Q($term: [String]) { search(term:$term) }`,
		`query Q($term: Unknown) { find(filter:{term:$term}) }`,
		`query Q($term: Int) { find(filter:{term:$term}) }`,
		`query Q($term: String) { search(term:"ok") @skip(if:$term) }`,
	} {
		bundle, err := parseDocumentBundle(query, map[string]any{"term": "ok"}, 0)
		if err != nil {
			t.Fatal(err)
		}
		op, err := selectOperation(bundle, "")
		if err != nil {
			t.Fatal(err)
		}
		if errs := ValidateDocument(e.Schema, bundle, op); len(errs) == 0 {
			t.Errorf("incompatible variable passed: %s", query)
		}
	}
}
func TestVariableNonNullDefaultAllowsRequiredPosition(t *testing.T) {
	response := newValExecutor(t).Execute(context.Background(), core.Request{Query: `query Q($term:String="ok") { search(term:$term) }`})
	if len(response.Errors) > 0 {
		t.Fatal(response.Errors)
	}
}

func TestVariableWrapperCompatibility(t *testing.T) {
	for _, tc := range []struct {
		declared, expected string
		allowed            bool
	}{
		{"Int!", "Int", true}, {"[Int!]!", "[Int]", true}, {"[Int]", "[Int!]", false}, {"Int", "[Int]", false}, {"[Int]", "Int", false}, {"Int", "Int!", false}, {"Int", "CustomInt", false},
	} {
		if got := variableTypeAllowed(tc.declared, tc.expected); got != tc.allowed {
			t.Errorf("%s to %s: got %v", tc.declared, tc.expected, got)
		}
	}
}
func TestVariablePositionsRespectLocationDefaultsAndNestedLists(t *testing.T) {
	s, err := ParseSDL(`input Filter { values:[Int!]! } type Query { value(required:Int! = 1, filter:Filter, list:[Int!]):String }`)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		query   string
		invalid bool
	}{
		{`query Q($v:Int){value(required:$v)}`, false},
		{`query Q($v:Int){value(list:[$v])}`, true},
		{`query Q($v:Int!){value(list:[$v])}`, false},
		{`query Q($v:[Int]){value(filter:{values:$v})}`, true},
		{`query Q($v:Int=null){value(list:[$v])}`, true},
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
