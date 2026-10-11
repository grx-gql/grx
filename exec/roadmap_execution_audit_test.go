package exec

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/schema"
)

func roadmapAuditSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s, err := schema.Build(schema.Config{Query: introQuery{}})
	if err != nil {
		t.Fatal(err)
	}
	s.Query.Fields["ok"] = &schema.Field{Name: "ok", Type: s.Types["String"], Resolver: func(context.Context, schema.ResolveParams) (any, error) { return "ok", nil }}
	return s
}

func roadmapAuditJSON(t *testing.T, value any) string {
	t.Helper()
	b, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestRoadmapNullableResolverErrorKeepsNullField(t *testing.T) {
	s := roadmapAuditSchema(t)
	s.Query.Fields["broken"] = &schema.Field{Name: "broken", Type: s.Types["String"], Resolver: func(context.Context, schema.ResolveParams) (any, error) { return nil, errors.New("broken") }}
	r := New(s, nil).Execute(context.Background(), core.Request{Query: `{ alias: broken ok }`})
	if got := roadmapAuditJSON(t, r.Data); got != `{"alias":null,"ok":"ok"}` {
		t.Fatalf("nullable resolver error must preserve selected alias as null; got %s, errors=%+v", got, r.Errors)
	}
}

func TestRoadmapNonNullFailureBubblesDespiteSibling(t *testing.T) {
	s := roadmapAuditSchema(t)
	s.Query.Fields["broken"] = &schema.Field{Name: "broken", Type: &schema.NonNull{OfType: s.Types["String"]}, Resolver: func(context.Context, schema.ResolveParams) (any, error) { return nil, nil }}
	r := New(s, nil).Execute(context.Background(), core.Request{Query: `{ broken ok }`})
	if !r.DataNull || r.Data != nil {
		t.Fatalf("non-null root failure must null all data despite successful sibling; got %s", roadmapAuditJSON(t, r))
	}
}

func TestRoadmapNonNullResolverErrorReportedOnce(t *testing.T) {
	s := roadmapAuditSchema(t)
	s.Query.Fields["broken"] = &schema.Field{Name: "broken", Type: &schema.NonNull{OfType: s.Types["String"]}, Resolver: func(context.Context, schema.ResolveParams) (any, error) { return nil, errors.New("broken") }}
	r := New(s, nil).Execute(context.Background(), core.Request{Query: `{ broken }`})
	if len(r.Errors) != 1 {
		t.Fatalf("one failed response position must yield exactly one error; got %+v", r.Errors)
	}
}

func TestRoadmapNullableChildErrorPreservesRequiredParent(t *testing.T) {
	s := roadmapAuditSchema(t)
	obj := &schema.Object{TypeName: "Nested", Fields: map[string]*schema.Field{
		"ok":     s.Query.Fields["ok"],
		"broken": {Name: "broken", Type: s.Types["String"], Resolver: func(context.Context, schema.ResolveParams) (any, error) { return nil, errors.New("broken") }},
	}}
	s.Types["Nested"] = obj
	s.Query.Fields["nested"] = &schema.Field{Name: "nested", Type: &schema.NonNull{OfType: obj}, Resolver: func(context.Context, schema.ResolveParams) (any, error) { return struct{}{}, nil }}
	r := New(s, nil).Execute(context.Background(), core.Request{Query: `{ nested { broken ok } }`})
	if got := roadmapAuditJSON(t, r.Data); got != `{"nested":{"broken":null,"ok":"ok"}}` || len(r.Errors) != 1 {
		t.Fatalf("got %s errors=%+v", got, r.Errors)
	}
}
func TestRoadmapNonNullListElementNullsList(t *testing.T) {
	s := roadmapAuditSchema(t)
	s.Query.Fields["values"] = &schema.Field{Name: "values", Type: &schema.List{OfType: &schema.NonNull{OfType: s.Types["String"]}}, Resolver: func(context.Context, schema.ResolveParams) (any, error) { return []any{"ok", nil}, nil }}
	r := New(s, nil).Execute(context.Background(), core.Request{Query: `{ values ok }`})
	if got := roadmapAuditJSON(t, r.Data); got != `{"values":null,"ok":"ok"}` {
		t.Fatalf("null non-null list element must null containing list; got %s, errors=%+v", got, r.Errors)
	}
}

func TestRoadmapObjectListFastPathPreservesNullElement(t *testing.T) {
	s, err := schema.Build(schema.Config{Query: orderedListQuery{}})
	if err != nil {
		t.Fatal(err)
	}
	s.Query.Fields["items"].Resolver = func(context.Context, schema.ResolveParams) (any, error) {
		return []*orderedListRow{nil}, nil
	}
	r := New(s, nil).Execute(context.Background(), core.Request{Query: `{ items { name } }`})
	if len(r.Errors) != 0 || roadmapAuditJSON(t, r.Data) != `{"items":[null]}` {
		t.Fatalf("nullable nil object list element must remain null without resolver errors; got %s", roadmapAuditJSON(t, r))
	}
}

func TestRoadmapNonNullStreamInitialElementNullsList(t *testing.T) {
	s := roadmapAuditSchema(t)
	s.Query.Fields["values"] = &schema.Field{Name: "values", Type: &schema.List{OfType: &schema.NonNull{OfType: s.Types["String"]}}, Resolver: func(context.Context, schema.ResolveParams) (any, error) { return []any{nil, "next"}, nil }}
	initial, patches := New(s, nil).ExecuteIncremental(context.Background(), core.Request{Query: `{ values @stream(initialCount:1) ok }`})
	if got := roadmapAuditJSON(t, initial.Data); got != `{"values":null,"ok":"ok"}` || len(initial.Errors) != 1 || len(patches) != 0 {
		t.Fatalf("data=%s errors=%+v patches=%+v", got, initial.Errors, patches)
	}
}

func TestRoadmapObjectListNonNullElementBubbles(t *testing.T) {
	s, err := schema.Build(schema.Config{Query: orderedListQuery{}})
	if err != nil {
		t.Fatal(err)
	}
	list := s.Query.Fields["items"].Type.(*schema.List)
	s.Query.Fields["items"].Type = &schema.List{OfType: &schema.NonNull{OfType: list.OfType}}
	s.Query.Fields["items"].Resolver = func(context.Context, schema.ResolveParams) (any, error) { return []*orderedListRow{nil}, nil }
	r := New(s, nil).Execute(context.Background(), core.Request{Query: `{ items { name } }`})
	if got := roadmapAuditJSON(t, r.Data); got != `{"items":null}` || len(r.Errors) != 1 {
		t.Fatalf("data=%s errors=%+v", got, r.Errors)
	}
}
func TestRoadmapPanicRecoveryPreservesPartialData(t *testing.T) {
	s := roadmapAuditSchema(t)
	s.Query.Fields["broken"] = &schema.Field{Name: "broken", Type: s.Types["String"], Resolver: func(context.Context, schema.ResolveParams) (any, error) { panic("broken") }}
	r := New(s, nil).Execute(context.Background(), core.Request{Query: `{ ok broken }`})
	if got := roadmapAuditJSON(t, r.Data); got != `{"ok":"ok","broken":null}` {
		t.Fatalf("recover nullable resolver panic as field error with partial data; got %s", roadmapAuditJSON(t, r))
	}
}

func TestRoadmapIncrementalResolverPanicRecovered(t *testing.T) {
	s := roadmapAuditSchema(t)
	s.Query.Fields["broken"] = &schema.Field{Name: "broken", Type: s.Types["String"], Resolver: func(context.Context, schema.ResolveParams) (any, error) { panic("broken") }}
	defer func() {
		if value := recover(); value != nil {
			t.Fatalf("ExecuteIncremental leaked resolver panic: %v", value)
		}
	}()
	initial, patches := New(s, nil).ExecuteIncremental(context.Background(), core.Request{Query: `{ ok ... on Query @defer { broken } }`})
	if len(patches) != 1 || len(patches[0].Errors) != 1 {
		t.Fatalf("expected deferred field error; got initial=%+v patches=%+v", initial, patches)
	}
}
func TestRoadmapScalarCompletionErrorLocation(t *testing.T) {
	s := roadmapAuditSchema(t)
	s.Query.Fields["broken"] = &schema.Field{Name: "broken", Type: s.Types["Int"], Resolver: func(context.Context, schema.ResolveParams) (any, error) { return "invalid", nil }}
	r := New(s, nil).Execute(context.Background(), core.Request{Query: "{\n alias: broken\n}"})
	if len(r.Errors) != 1 || !reflect.DeepEqual(r.Errors[0].Locations, []core.Location{{Line: 2, Column: 2}}) {
		t.Fatalf("scalar completion error must identify selected field source location; got %+v", r.Errors)
	}
}
func TestRoadmapResolverCacheDoesNotCollideArguments(t *testing.T) {
	s := roadmapAuditSchema(t)
	s.Query.Fields["join"] = &schema.Field{Name: "join", Type: s.Types["String"], Args: []schema.InputValue{{Name: "a", Type: s.Types["String"]}, {Name: "b", Type: s.Types["String"]}}, Resolver: func(_ context.Context, p schema.ResolveParams) (any, error) {
		return p.Args["a"].(string) + "|" + p.Args["b"].(string), nil
	}}
	r := New(s, nil, WithResolverCache()).Execute(context.Background(), core.Request{Query: `{ first: join(a: "x b:y", b: "z") second: join(a: "x", b: "y b:z") }`})
	if len(r.Errors) != 0 {
		t.Fatal(r.Errors)
	}
	if got := roadmapAuditJSON(t, r.Data); got != `{"first":"x b:y|z","second":"x|y b:z"}` {
		t.Fatalf("different argument tuples must not share cached result; got %s", got)
	}
}

func TestRoadmapResolverCacheSeparatesSameNamedFields(t *testing.T) {
	s := roadmapAuditSchema(t)
	for _, name := range []string{"A", "B"} {
		object := &schema.Object{TypeName: name, Fields: map[string]*schema.Field{"value": {Name: "value", Type: s.Types["String"], Resolver: func(context.Context, schema.ResolveParams) (any, error) { return name, nil }}}}
		s.Types[name] = object
		s.Query.Fields[name] = &schema.Field{Name: name, Type: object, Resolver: func(context.Context, schema.ResolveParams) (any, error) { return "same-source", nil }}
	}
	r := New(s, nil, WithResolverCache()).Execute(context.Background(), core.Request{Query: `{ A { value } B { value } }`})
	if got := roadmapAuditJSON(t, r.Data); got != `{"A":{"value":"A"},"B":{"value":"B"}}` || len(r.Errors) != 0 {
		t.Fatalf("data=%s errors=%+v", got, r.Errors)
	}
}

type roadmapCacheCustomValue struct{ Secret string }

func (roadmapCacheCustomValue) MarshalJSON() ([]byte, error) { return []byte(`"same"`), nil }

func TestRoadmapResolverCacheDoesNotUseApplicationJSON(t *testing.T) {
	field := &schema.Field{Name: "custom"}
	if _, ok := resolverCacheKey(field, nil, map[string]any{"value": roadmapCacheCustomValue{Secret: "a"}}); ok {
		t.Fatal("custom runtime value must not use potentially lossy application JSON as a cache key")
	}
	first, firstOK := resolverCacheKey(field, nil, map[string]any{"value": int(1)})
	second, secondOK := resolverCacheKey(field, nil, map[string]any{"value": float64(1)})
	if !firstOK || !secondOK || first == second {
		t.Fatal("numeric Go type distinctions must be preserved")
	}
}
