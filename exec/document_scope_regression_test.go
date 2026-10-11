package exec

import (
	"context"
	"github.com/grx-gql/grx/core"
	"testing"
)

func TestRoadmapValidationScopesVariablesPerOperation(t *testing.T) {
	e := newValExecutor(t)
	response := e.Execute(context.Background(), core.Request{
		Query:         `query A($a: String!) { search(term: $a) } query B($b: String!) { search(term: $b) }`,
		OperationName: "A",
		Variables:     map[string]any{"a": "ok"},
	})
	if len(response.Errors) > 0 {
		t.Fatalf("valid selected operation rejected: %#v", response.Errors)
	}
}

func TestRoadmapValidationChecksUnselectedOperation(t *testing.T) {
	e := newValExecutor(t)
	response := e.Execute(context.Background(), core.Request{
		Query:         `query A { search(term: "ok") } query B { unknownField }`,
		OperationName: "A",
	})
	if len(response.Errors) == 0 {
		t.Fatalf("document containing invalid operation executed: %#v", response.Data)
	}
}

func TestRoadmapValidationCountsFragmentsUsedByAnyOperation(t *testing.T) {
	e := newValExecutor(t)
	response := e.Execute(context.Background(), core.Request{
		Query:         `query A { search(term: "ok") } query B { ...F } fragment F on Query { search(term: "other") }`,
		OperationName: "A",
	})
	if len(response.Errors) > 0 {
		t.Fatalf("valid document rejected: %#v", response.Errors)
	}
}

func TestDocumentVariableUsesFollowReachableFragments(t *testing.T) {
	e := newValExecutor(t)
	for _, tc := range []struct {
		query   string
		invalid bool
	}{
		{`query A($a:String!){...F} query B($b:String!){search(term:$b)} fragment F on Query { search(term:$a) }`, false},
		{`query A($a:String!){search(term:"ok")} query B($a:String!){search(term:$a)}`, true},
		{`query A {search(term:"ok")} query B($skip:Boolean!){search(term:"ok") @skip(if:$skip)}`, false},
	} {
		response := e.Execute(context.Background(), core.Request{Query: tc.query, OperationName: "A", Variables: map[string]any{"a": "ok"}})
		if (len(response.Errors) > 0) != tc.invalid {
			t.Errorf("query %s: errors=%v", tc.query, response.Errors)
		}
	}
}
