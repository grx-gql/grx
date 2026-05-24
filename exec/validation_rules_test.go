package exec

import (
	"context"
	"strings"
	"testing"

	"github.com/patrickkabwe/grx/core"
	"github.com/patrickkabwe/grx/schema"
)

type valRole string

const (
	valRoleAdmin valRole = "ADMIN"
	valRoleUser  valRole = "USER"
)

type valFilter struct {
	Term  string `gql:"term,nonNull"`
	Limit int    `gql:"limit"`
}

type valQuery struct{}

func (valQuery) Search(ctx context.Context, args struct {
	Term  string  `gql:"term,nonNull"`
	Count int     `gql:"count"`
	Role  valRole `gql:"role"`
}) (string, error) {
	return args.Term, nil
}

func (valQuery) Find(ctx context.Context, args struct {
	Filter valFilter `gql:"filter,nonNull"`
}) (string, error) {
	return args.Filter.Term, nil
}

func newValExecutor(t *testing.T) *Executor {
	t.Helper()
	s, err := schema.Build(schema.Config{
		Query: valQuery{},
		Enums: []schema.EnumConfig{{
			Type:   valRole(""),
			Name:   "valRole",
			Values: []schema.EnumValueConfig{{Name: "ADMIN", Value: valRoleAdmin}, {Name: "USER", Value: valRoleUser}},
		}},
	})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}
	return New(s, nil)
}

func valErrors(t *testing.T, e *Executor, query string) []core.Error {
	t.Helper()
	return e.Execute(context.Background(), core.Request{Query: query}).Errors
}

func hasErrContaining(errs []core.Error, substr string) bool {
	for _, e := range errs {
		if strings.Contains(e.Message, substr) {
			return true
		}
	}
	return false
}

func TestValidationScalarLiteralTypes(t *testing.T) {
	e := newValExecutor(t)
	cases := []struct {
		name  string
		query string
		want  string
	}{
		{"string for int", `{ search(term: "x", count: "nope") }`, "Int cannot represent"},
		{"bool for string", `{ search(term: true) }`, "String cannot represent"},
		{"null for non-null", `{ search(term: null) }`, "found null"},
		{"bad enum value", `{ search(term: "x", role: MAYBE) }`, "does not exist in"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := valErrors(t, e, tc.query)
			if !hasErrContaining(errs, tc.want) {
				t.Fatalf("expected error containing %q, got %#v", tc.want, errs)
			}
		})
	}
}

func TestValidationScalarLiteralAccepts(t *testing.T) {
	e := newValExecutor(t)
	// Valid literals must not produce validation errors.
	if errs := valErrors(t, e, `{ search(term: "x", count: 3, role: ADMIN) }`); len(errs) != 0 {
		t.Fatalf("unexpected errors for valid literals: %#v", errs)
	}
}

func TestValidationInputObjectRules(t *testing.T) {
	e := newValExecutor(t)

	if errs := valErrors(t, e, `{ find(filter: {limit: 2}) }`); !hasErrContaining(errs, "required type") {
		t.Fatalf("expected missing-required-field error, got %#v", errs)
	}
	if errs := valErrors(t, e, `{ find(filter: {term: "x", bogus: 1}) }`); !hasErrContaining(errs, "is not defined by type") {
		t.Fatalf("expected unknown-input-field error, got %#v", errs)
	}
	if errs := valErrors(t, e, `{ find(filter: {term: 5}) }`); !hasErrContaining(errs, "String cannot represent") {
		t.Fatalf("expected nested field type error, got %#v", errs)
	}
	if errs := valErrors(t, e, `{ find(filter: {term: "ok"}) }`); len(errs) != 0 {
		t.Fatalf("valid input object should pass, got %#v", errs)
	}
}

func TestValidationVariableUniqueness(t *testing.T) {
	e := newValExecutor(t)
	errs := valErrors(t, e, `query ($x: String!, $x: String!) { search(term: $x) }`)
	if !hasErrContaining(errs, `only one variable named "$x"`) {
		t.Fatalf("expected duplicate variable error, got %#v", errs)
	}
}

func TestValidationVariablesMustBeInputTypes(t *testing.T) {
	e := newValExecutor(t)
	// String is a valid input type — the variable definition must not be
	// rejected as a non-input type.
	resp := e.Execute(context.Background(), core.Request{
		Query:     `query ($t: String!) { search(term: $t) }`,
		Variables: map[string]any{"t": "hello"},
	})
	if hasErrContaining(resp.Errors, "non-input type") {
		t.Fatalf("String! variable wrongly flagged as non-input: %#v", resp.Errors)
	}
	if len(resp.Errors) != 0 {
		t.Fatalf("valid input-typed variable should pass, got %#v", resp.Errors)
	}
}

func TestValidationDeferLabelUniqueness(t *testing.T) {
	e := newValExecutor(t)
	query := `{
		a: search(term: "x")
		... on valQuery @defer(label: "dup") { b: search(term: "y") }
		... on valQuery @defer(label: "dup") { c: search(term: "z") }
	}`
	errs := valErrors(t, e, query)
	if !hasErrContaining(errs, `label "dup" must be unique`) {
		t.Fatalf("expected duplicate defer label error, got %#v", errs)
	}
}
