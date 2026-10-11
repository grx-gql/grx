package exec

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/schema"
)

func TestLateNonNullStreamItemStopsAtStreamBoundary(t *testing.T) {
	scalar := &schema.Scalar{TypeName: "Int"}
	list := &schema.List{OfType: &schema.NonNull{OfType: scalar}}
	root := &schema.Object{TypeName: "Query", Fields: map[string]*schema.Field{
		"values": {Name: "values", Type: list, Resolver: func(context.Context, schema.ResolveParams) (any, error) { return []any{1, nil, 3}, nil }},
		"other":  {Name: "other", Type: scalar, Resolver: func(context.Context, schema.ResolveParams) (any, error) { return 42, nil }},
	}}
	e := New(&schema.Schema{Query: root, Types: map[string]schema.Type{"Query": root, "Int": scalar}}, nil)
	initial, patches := e.ExecuteIncremental(context.Background(), core.Request{Query: `{ values @stream(initialCount:1,label:"values") ... on Query @defer(label:"other") { other } }`})
	if len(initial.Errors) != 0 {
		t.Fatalf("initial errors=%v", initial.Errors)
	}
	if len(patches) != 2 {
		t.Fatalf("patches=%#v; want one stream failure and unrelated deferred patch", patches)
	}
	failure := patches[0]
	if failure.Label != "values" {
		failure = patches[1]
	}
	raw, err := json.Marshal(failure)
	if err != nil {
		t.Fatal(err)
	}
	var wire map[string]any
	if err := json.Unmarshal(raw, &wire); err != nil {
		t.Fatal(err)
	}
	items, present := wire["items"]
	if !present || items != nil {
		t.Fatalf("failure patch=%s; want items:null", raw)
	}
	if !reflect.DeepEqual(failure.Path, []any{"values", 1}) || len(failure.Errors) != 1 {
		t.Fatalf("failure patch=%#v", failure)
	}
	if !reflect.DeepEqual(failure.Errors[0].Path, []any{"values", 1}) {
		t.Fatalf("error path=%v", failure.Errors[0].Path)
	}
	if patches[0].Label != "other" && patches[1].Label != "other" {
		t.Fatal("unrelated deferred work was lost")
	}
}

func TestStreamNullItemPrunesOnlyAffectedPendingWork(t *testing.T) {
	for _, nonNull := range []bool{false, true} {
		t.Run(map[bool]string{false: "nullable", true: "non-null"}[nonNull], func(t *testing.T) {
			scalar := &schema.Scalar{TypeName: "Int"}
			var deferred []int
			item := &schema.Object{TypeName: "Item", Fields: map[string]*schema.Field{
				"must": {Name: "must", Type: &schema.NonNull{OfType: scalar}, Resolver: func(_ context.Context, p schema.ResolveParams) (any, error) {
					if p.Source.(int) == 1 {
						return nil, nil
					}
					return p.Source, nil
				}},
				"later": {Name: "later", Type: scalar, Resolver: func(_ context.Context, p schema.ResolveParams) (any, error) {
					deferred = append(deferred, p.Source.(int))
					return p.Source, nil
				}},
			}}
			var itemType schema.Type = item
			if nonNull {
				itemType = &schema.NonNull{OfType: item}
			}
			root := &schema.Object{TypeName: "Query", Fields: map[string]*schema.Field{
				"values": {Name: "values", Type: &schema.List{OfType: itemType}, Resolver: func(context.Context, schema.ResolveParams) (any, error) { return []int{0, 1, 2}, nil }},
				"other":  {Name: "other", Type: &schema.List{OfType: scalar}, Resolver: func(context.Context, schema.ResolveParams) (any, error) { return []int{9}, nil }},
			}}
			e := New(&schema.Schema{Query: root, Types: map[string]schema.Type{"Query": root, "Item": item, "Int": scalar}}, nil)
			var patches []core.IncrementalPayload
			var more []bool
			err := e.ExecuteIncrementalStream(context.Background(), core.Request{Query: `{ values @stream(initialCount:1,label:"values") { must ... on Item @defer {later} } other @stream(initialCount:0,label:"other") }`}, func(initial core.Response) error {
				if len(initial.Errors) > 0 {
					t.Fatalf("initial=%#v", initial)
				}
				return nil
			}, func(p core.IncrementalPayload, next bool) error {
				patches = append(patches, p)
				more = append(more, next)
				return nil
			})
			if err != nil {
				t.Fatal(err)
			}
			if len(more) == 0 || more[len(more)-1] {
				t.Fatalf("hasNext=%v", more)
			}
			wantDeferred := []int{0, 2}
			if nonNull {
				wantDeferred = []int{0}
			}
			if !reflect.DeepEqual(deferred, wantDeferred) {
				t.Fatalf("deferred=%v; want %v", deferred, wantDeferred)
			}
			errorsSeen := 0
			otherSeen := false
			for _, patch := range patches {
				if patch.Label == "other" {
					otherSeen = true
				}
				if len(patch.Errors) > 0 {
					errorsSeen += len(patch.Errors)
					if patch.ItemsNull != nonNull {
						t.Fatalf("patch=%#v", patch)
					}
					if !reflect.DeepEqual(patch.Errors[0].Path, []any{"values", 1, "must"}) {
						t.Fatalf("error path=%v", patch.Errors[0].Path)
					}
				}
			}
			if errorsSeen != 1 || !otherSeen {
				t.Fatalf("errors=%d other=%v", errorsSeen, otherSeen)
			}
		})
	}
}

func TestDeferredNullBoundaryCancelsNestedWork(t *testing.T) {
	scalar := &schema.Scalar{TypeName: "String"}
	calls := 0
	user := &schema.Object{TypeName: "User", Fields: map[string]*schema.Field{
		"must":  {Name: "must", Type: &schema.NonNull{OfType: scalar}, Resolver: func(context.Context, schema.ResolveParams) (any, error) { return nil, nil }},
		"later": {Name: "later", Type: scalar, Resolver: func(context.Context, schema.ResolveParams) (any, error) { calls++; return "late", nil }},
	}}
	root := &schema.Object{TypeName: "Query", Fields: map[string]*schema.Field{
		"user":  {Name: "user", Type: user, Resolver: func(context.Context, schema.ResolveParams) (any, error) { return struct{}{}, nil }},
		"other": {Name: "other", Type: scalar, Resolver: func(context.Context, schema.ResolveParams) (any, error) { return "ok", nil }},
	}}
	e := New(&schema.Schema{Query: root, Types: map[string]schema.Type{"Query": root, "User": user, "String": scalar}}, nil)
	initial, patches := e.ExecuteIncremental(context.Background(), core.Request{Query: `{user { ... on User @defer(label:"outer") {must ... on User @defer(label:"nested"){later}} } ... on Query @defer(label:"other"){other}}`})
	if len(initial.Errors) > 0 {
		t.Fatalf("initial=%#v", initial)
	}
	if calls != 0 || len(patches) != 2 {
		t.Fatalf("nested resolver calls=%d patches=%#v", calls, patches)
	}
	for _, patch := range patches {
		if patch.Label != "outer" {
			continue
		}
		raw, err := json.Marshal(patch)
		if err != nil {
			t.Fatal(err)
		}
		var wire map[string]json.RawMessage
		if err := json.Unmarshal(raw, &wire); err != nil {
			t.Fatal(err)
		}
		if string(wire["data"]) != "null" || len(patch.Errors) != 1 {
			t.Fatalf("patch=%s", raw)
		}
	}
}
