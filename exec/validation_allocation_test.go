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
	// The fragment recursion stack is the only required heap allocation.
	if allocations > 1 {
		t.Fatalf("fragment validation allocated %g times; want at most 1", allocations)
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
	if allocations > 44 {
		t.Fatalf("parse allocated %g times; want at most 44", allocations)
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
