package exec

import (
	"context"
	"github.com/grx-gql/grx/core"
	"testing"
)

func TestRoadmapValidationAllowsVariableIncrementalArguments(t *testing.T) {
	e := newValExecutor(t)
	response := e.Execute(context.Background(), core.Request{
		Query:     `query Q($active: Boolean!) { ... on Query @defer(if: $active) { search(term: "ok") } }`,
		Variables: map[string]any{"active": false},
	})
	if len(response.Errors) > 0 {
		t.Fatalf("valid variable directive argument rejected: %#v", response.Errors)
	}
}

func TestIncrementalDirectiveBoundAndOmittedVariables(t *testing.T) {
	e := newValExecutor(t)
	for _, tc := range []struct {
		query string
		vars  map[string]any
	}{
		{`query Q($label: String, $count: Int) { search(term:"ok") @stream(label:$label, initialCount:$count, if:false) }`, map[string]any{"label": "later", "count": 2}},
		{`query Q($active: Boolean) { ... on Query @defer(if:$active) { search(term:"ok") } }`, nil},
	} {
		bundle, err := parseDocumentBundle(tc.query, tc.vars, 0)
		if err != nil {
			t.Fatal(err)
		}
		op, err := selectOperation(bundle, "")
		if err != nil {
			t.Fatal(err)
		}
		if errs := ValidateDocument(e.Schema, bundle, op); len(errs) > 0 {
			t.Errorf("valid directive variables rejected: %v", errs)
		}
	}
}
