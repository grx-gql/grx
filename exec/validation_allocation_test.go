//go:build !race

package exec

import (
	"testing"

	"github.com/grx-gql/grx/schema"
)

func TestFragmentValidationScratchAllocations(t *testing.T) {
	s, err := schema.Build(schema.Config{Query: orderedListQuery{}})
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := parseDocumentBundle(`{ ...Root } fragment Root on Query { __typename }`, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	allocations := testing.AllocsPerRun(100, func() {
		if errs := ValidateDocument(s, bundle, bundle.operations[0]); len(errs) != 0 {
			t.Fatal(errs)
		}
	})
	// Small fragment validation needs no heap-backed scratch storage.
	if allocations != 0 {
		t.Fatalf("fragment validation allocated %g times; want 0", allocations)
	}
}

func TestUniqueFieldMergeValidationAllocations(t *testing.T) {
	s, err := schema.Build(schema.Config{Query: orderedListQuery{}})
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := parseDocumentBundle(`{ first: __typename second: __typename third: __typename }`, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	allocations := testing.AllocsPerRun(100, func() {
		if errs := validateFieldMergeConflicts(s, s.Query, bundle.operations[0].Selections, bundle.fragments); len(errs) != 0 {
			t.Fatal(errs)
		}
	})
	if allocations != 0 {
		t.Fatalf("distinct fields allocated %g times; want 0", allocations)
	}
}

func TestParserVariableBindingAllocations(t *testing.T) {
	vars := map[string]any{"id": "user_42"}
	allocations := testing.AllocsPerRun(100, func() {
		if _, err := parseDocument(benchmarkQuery, vars); err != nil {
			t.Fatal(err)
		}
	})
	if allocations > 38 {
		t.Fatalf("parse allocated %g times; want at most 38", allocations)
	}
}

func TestStringInputCoercionPreservesBoxedValue(t *testing.T) {
	var raw any = "already boxed"
	var captured any
	allocations := testing.AllocsPerRun(100, func() {
		value, err := coerceScalarInput("String", raw)
		if err != nil {
			t.Fatal(err)
		}
		captured = value
	})
	if allocations != 0 {
		t.Fatalf("String input reboxing allocated %g times; want 0", allocations)
	}
	if captured != raw {
		t.Fatalf("coerced value = %v", captured)
	}
}

func TestEmptyDefinitionDirectivesAvoidAllocations(t *testing.T) {
	s, err := schema.Build(schema.Config{Query: orderedListQuery{}})
	if err != nil {
		t.Fatal(err)
	}
	bundle, err := parseDocumentBundle(`{ __typename }`, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	allocations := testing.AllocsPerRun(100, func() {
		if errs := validateDefinitionDirectives(s, bundle, bundle.operations[0], nil); len(errs) != 0 {
			t.Fatal(errs)
		}
	})
	if allocations != 0 {
		t.Fatalf("empty definition directives allocated %g times; want 0", allocations)
	}
}

func TestRequiredVariableCoercionAvoidsWrapperAllocation(t *testing.T) {
	e := scopedVariableExecutor(t)
	provided := map[string]any{"v": "value"}
	allocations := make([]float64, 0, 2)
	for _, reference := range []string{"String", "String!"} {
		bundle, err := parseDocumentBundle(`query Q($v:`+reference+`){echo(value:$v)}`, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		allocations = append(allocations, testing.AllocsPerRun(100, func() {
			bound, err := coerceOperationVariables(e.Schema, bundle.operations[0], provided)
			if err != nil {
				t.Fatal(err)
			}
			if bound.Selections[0].Arguments["value"].(variableRef).Value != "value" {
				t.Fatal("wrong coerced value")
			}
		}))
	}
	if allocations[1] != allocations[0] {
		t.Fatalf("required allocated %g times; nullable %g", allocations[1], allocations[0])
	}
}
