package exec

import (
	"context"
	"testing"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/schema"
)

func TestIncrementalDirectiveDetectionBranches(t *testing.T) {
	fragments := map[string]*fragmentDef{
		"A": {Name: "A", Selections: []selection{{FragmentSpread: "B"}}},
		"B": {Name: "B", Selections: []selection{{FragmentSpread: "A"}}},
		"C": {Name: "C", Selections: []selection{{Name: "name", Directives: []directive{{Name: "stream", Args: map[string]any{"if": true}}}}}},
	}
	if selectionsUseIncremental([]selection{{FragmentSpread: "A"}}, fragments, map[string]bool{}) {
		t.Fatal("recursive fragments without directives should not use incremental")
	}
	if !selectionsUseIncremental([]selection{{FragmentSpread: "C"}}, fragments, map[string]bool{}) {
		t.Fatal("fragment stream directive should use incremental")
	}
	if !selectionsUseIncremental([]selection{{Selections: []selection{{Directives: []directive{{Name: "defer", Args: map[string]any{"if": true}}}}}}}, nil, map[string]bool{}) {
		t.Fatal("nested defer directive should use incremental")
	}
}

func TestHasIncrementalDirectiveFalseOnMalformedOrMissingOps(t *testing.T) {
	s, err := schema.Build(schema.Config{Query: testQuery{}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	e := New(s, nil)
	if e.HasIncrementalDirectives(core.Request{Query: `{ broken`}) {
		t.Fatal("malformed queries should not report incremental directives")
	}
	if e.HasIncrementalDirectives(core.Request{Query: `{ __typename }`, OperationName: "nope"}) {
		t.Fatal("missing operation name should disable incremental probing")
	}
}

func TestCompleteListStreamedRegistersTrailingItemsAndRejectsScalars(t *testing.T) {
	t.Parallel()

	e := &Executor{}
	intTyp := &schema.Scalar{TypeName: "Int"}
	collector := &incrementalCollector{}
	vals, errs := e.completeListStreamed(context.Background(), intTyp, []int{10, 20, 30}, nil, nil, []any{"items"}, 1, "batch", collector)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors %#v", errs)
	}
	initial, ok := vals.([]any)
	if !ok || len(initial) != 1 || initial[0] != 10 {
		t.Fatalf("initial streamed items = %#v", vals)
	}
	if len(collector.work) != 2 {
		t.Fatalf("expected two trailing stream payloads, got %d", len(collector.work))
	}
	for _, w := range collector.work {
		if !w.stream || w.label != "batch" {
			t.Fatalf("unexpected work item %#v", w)
		}
	}

	if _, errs := e.completeListStreamed(context.Background(), intTyp, 42, nil, nil, []any{"solo"}, 1, "", nil); len(errs) == 0 {
		t.Fatal("expected coercion error completing list from scalar value")
	}
}
