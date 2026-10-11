package exec

import (
	"context"
	"encoding/json"
	"reflect"
	"strconv"
	"testing"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/schema"
)

func TestFlattenSelectionsPreservesSharedDocument(t *testing.T) {
	children := make([]selection, 1, 4)
	children[0] = selection{Name: "id"}
	input := []selection{
		{Name: "user", Selections: children},
		{Name: "user", Selections: []selection{{Name: "name"}}},
	}
	executor := &Executor{}
	flat, _, errs := executor.flattenSelections(false, &schema.Object{TypeName: "Query"}, input, nil)
	if len(errs) != 0 || len(flat) != 1 || len(flat[0].Selections) != 2 {
		t.Fatalf("unexpected merge: flat=%+v errors=%+v", flat, errs)
	}
	if got := children[:cap(children)][1].Name; got != "" {
		t.Fatalf("merging wrote %q into the shared document backing array", got)
	}
}

func TestFlattenSimpleSelectionsDoesNotAllocate(t *testing.T) {
	executor := &Executor{}
	object := &schema.Object{TypeName: "User"}
	selections := []selection{{Name: "id"}, {Name: "name", Alias: "displayName"}}
	allocations := testing.AllocsPerRun(100, func() {
		flat, defers, errs := executor.flattenSelections(false, object, selections, nil)
		if len(flat) != 2 || len(defers) != 0 || len(errs) != 0 {
			t.Fatalf("unexpected selections: %+v, defers=%+v, errors=%+v", flat, defers, errs)
		}
	})
	if allocations != 0 {
		t.Fatalf("simple selections allocated %g times", allocations)
	}
}

func TestFlattenRepeatedSelectionsGrowAmortized(t *testing.T) {
	executor := &Executor{}
	object := &schema.Object{TypeName: "Query"}
	selections := make([]selection, 64)
	for index := range selections {
		selections[index] = selection{Name: "items", Selections: []selection{{Name: "id"}}}
	}
	allocations := testing.AllocsPerRun(100, func() {
		flat, _, errs := executor.flattenSelections(false, object, selections, nil)
		if len(errs) != 0 || len(flat) != 1 || len(flat[0].Selections) != 64 {
			t.Fatalf("unexpected merge: %+v, errors=%+v", flat, errs)
		}
	})
	if allocations > 12 {
		t.Fatalf("repeated field merging allocated %g times; expected amortized growth", allocations)
	}
}

func TestSelectionPlansPreserveListDirectivesAndAliases(t *testing.T) {
	compiled, err := schema.Build(schema.Config{Query: orderedListQuery{}})
	if err != nil {
		t.Fatal(err)
	}
	executor := New(compiled, nil)
	query := `query Items($show: Boolean!) {
        items {
            ...Row
            alias: name @include(if: $show)
        }
    }
    fragment Row on orderedListRow {
        id
        name @skip(if: $show)
    }`
	for _, show := range []bool{true, false, true} {
		response := executor.Execute(context.Background(), core.Request{
			Query: query, Variables: map[string]any{"show": show},
		})
		if len(response.Errors) != 0 {
			t.Fatal(response.Errors)
		}
		encoded, err := json.Marshal(response.Data)
		if err != nil {
			t.Fatal(err)
		}
		expected := `{"items":[{"id":"1","name":"Ada"},{"id":"2","name":"Grace"}]}`
		if show {
			expected = `{"items":[{"id":"1","alias":"Ada"},{"id":"2","alias":"Grace"}]}`
		}
		if string(encoded) != expected {
			t.Fatalf("show=%v: got %s, want %s", show, encoded, expected)
		}
	}
}

func TestSelectionPlansKeepConcreteTypesSeparate(t *testing.T) {
	user := &schema.Object{TypeName: "User"}
	post := &schema.Object{TypeName: "Post"}
	selections := []selection{
		{InlineFragmentOn: "User", Selections: []selection{{Name: "name"}}},
		{InlineFragmentOn: "Post", Selections: []selection{{Name: "title"}}},
	}
	executor := &Executor{}
	arena := &pathArena{}
	for _, object := range []*schema.Object{user, post, user, post} {
		flat, _, errs := executor.prepareSelections(arena, object, selections, nil)
		expected := []selection{{Name: "name"}}
		if object == post {
			expected = []selection{{Name: "title"}}
		}
		if len(errs) != 0 || !reflect.DeepEqual(flat, expected) {
			t.Fatalf("type=%s: flat=%+v errors=%+v", object.Name(), flat, errs)
		}
	}
}

func TestSelectionPlansReuseExpandedFragments(t *testing.T) {
	executor := &Executor{}
	arena := &pathArena{}
	object := &schema.Object{TypeName: "User"}
	selections := []selection{{FragmentSpread: "UserFields"}}
	fragments := map[string]*fragmentDef{
		"UserFields": {TypeCondition: "User", Selections: []selection{{Name: "id"}, {Name: "name"}}},
	}
	allocations := testing.AllocsPerRun(100, func() {
		flat, defers, errs := executor.prepareSelections(arena, object, selections, fragments)
		if len(errs) != 0 || len(defers) != 0 || len(flat) != 2 || flat[0].Name != "id" || flat[1].Name != "name" {
			t.Fatalf("unexpected plan: %+v, defers=%+v, errors=%+v", flat, defers, errs)
		}
	})
	if allocations != 0 {
		t.Fatalf("repeated fragment expansion allocated %g times", allocations)
	}
}

func TestSelectionPlanSingleLayoutAllocation(t *testing.T) {
	executor := &Executor{}
	object := &schema.Object{TypeName: "Query"}
	selections := []selection{{InlineFragmentOn: "Query", Selections: []selection{{Name: "__typename"}}}}
	allocations := testing.AllocsPerRun(100, func() {
		arena := &pathArena{}
		flat, _, errs := executor.prepareSelections(arena, object, selections, nil)
		if len(errs) != 0 || len(flat) != 1 || flat[0].Name != "__typename" {
			t.Fatalf("unexpected plan: %+v errors=%+v", flat, errs)
		}
	})
	if allocations > 1 {
		t.Fatalf("single layout allocated %g times; only the expanded fields need storage", allocations)
	}
}

func TestSelectionPlansPreserveDeferredListPaths(t *testing.T) {
	compiled, err := schema.Build(schema.Config{Query: orderedListQuery{}})
	if err != nil {
		t.Fatal(err)
	}
	executor := New(compiled, nil, WithDocumentCache(2))
	request := core.Request{Query: `{ items { id ... on orderedListRow @defer(label: "names") { name } } }`}
	for range 2 {
		initial, payloads := executor.ExecuteIncremental(context.Background(), request)
		encoded, err := json.Marshal(initial.Data)
		if err != nil {
			t.Fatal(err)
		}
		if len(initial.Errors) != 0 || string(encoded) != `{"items":[{"id":"1"},{"id":"2"}]}` || len(payloads) != 2 {
			t.Fatalf("initial=%s errors=%+v payloads=%+v", encoded, initial.Errors, payloads)
		}
		for index, expected := range []string{`{"name":"Ada"}`, `{"name":"Grace"}`} {
			payload := payloads[index]
			encoded, err := json.Marshal(payload.Data)
			if err != nil {
				t.Fatal(err)
			}
			if len(payload.Errors) != 0 || payload.Label != "names" || !reflect.DeepEqual(payload.Path, []any{"items", index}) || string(encoded) != expected {
				t.Fatalf("item %d: payload=%+v data=%s", index, payload, encoded)
			}
		}
		response := executor.Execute(context.Background(), request)
		encoded, err = json.Marshal(response.Data)
		if err != nil {
			t.Fatal(err)
		}
		if len(response.Errors) != 0 || string(encoded) != `{"items":[{"id":"1","name":"Ada"},{"id":"2","name":"Grace"}]}` {
			t.Fatalf("non-incremental response=%s errors=%+v", encoded, response.Errors)
		}
	}
}

func TestSelectionPlansWithConcurrentCachedDocuments(t *testing.T) {
	compiled, err := schema.Build(schema.Config{Query: orderedListQuery{}})
	if err != nil {
		t.Fatal(err)
	}
	executor := New(compiled, nil, WithDocumentCache(2))
	request := core.Request{Query: `{ items { id } items { ...Row name } } fragment Row on orderedListRow { id }`}
	if response := executor.Execute(context.Background(), request); len(response.Errors) != 0 {
		t.Fatal(response.Errors)
	}
	for index := range 16 {
		t.Run(strconv.Itoa(index), func(t *testing.T) {
			t.Parallel()
			for range 10 {
				response := executor.Execute(context.Background(), request)
				encoded, err := json.Marshal(response.Data)
				if err != nil {
					t.Fatal(err)
				}
				if len(response.Errors) != 0 || string(encoded) != `{"items":[{"id":"1","name":"Ada"},{"id":"2","name":"Grace"}]}` {
					t.Fatalf("response=%s errors=%+v", encoded, response.Errors)
				}
			}
		})
	}
}
