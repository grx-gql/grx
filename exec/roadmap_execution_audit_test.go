package exec

import (
	"context"
	"encoding/json"
	"errors"
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
