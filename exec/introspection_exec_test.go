package exec

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/plugins"
	"github.com/grx-gql/grx/schema"
)

func TestExecutableIntrospectionMatchesFastPath(t *testing.T) {
	s := buildGraphiQLTestSchema(t)
	fast := New(s, nil)
	exe := New(s, nil, WithExecutableIntrospection())

	req := core.Request{Query: graphiqlIntrospectionQuery}
	fastResp := fast.Execute(context.Background(), req)
	exeResp := exe.Execute(context.Background(), req)

	if len(fastResp.Errors) != 0 || len(exeResp.Errors) != 0 {
		t.Fatalf("unexpected errors: fast=%#v exe=%#v", fastResp.Errors, exeResp.Errors)
	}

	// The legacy option must preserve the normal execution response exactly.
	fastMap := orderedToMap(fastResp.Data)
	exeMap := orderedToMap(exeResp.Data)
	if !reflect.DeepEqual(exeMap, fastMap) {
		t.Fatal("legacy introspection option changed execution result")
	}
}

func TestExecutableIntrospectionHonorsSelectionSet(t *testing.T) {
	s := buildGraphiQLTestSchema(t)
	exe := New(s, nil, WithExecutableIntrospection())

	resp := exe.Execute(context.Background(), core.Request{
		Query: `{ __schema { queryType { name } } }`,
	})
	if len(resp.Errors) != 0 {
		t.Fatalf("errors: %#v", resp.Errors)
	}
	schemaObj := responseObject(t, responseObject(t, resp.Data)["__schema"])
	// Only queryType was selected, so types/directives must be absent.
	if _, ok := schemaObj["types"]; ok {
		t.Fatalf("expected unselected field 'types' to be absent: %#v", schemaObj)
	}
	qt := responseObject(t, schemaObj["queryType"])
	if qt["name"] == nil {
		t.Fatalf("expected queryType.name to be present")
	}
}

func TestExecutableIntrospectionInvokesFieldHooks(t *testing.T) {
	s := buildGraphiQLTestSchema(t)

	var seen []string
	authorizer := func(ctx context.Context, fc FieldAuthorizationContext) error {
		seen = append(seen, fc.FieldName)
		return nil
	}
	exe := New(s, nil, WithExecutableIntrospection(), WithFieldAuthorizer(authorizer))

	resp := exe.Execute(context.Background(), core.Request{
		Query: `{ __schema { queryType { name } } }`,
	})
	if len(resp.Errors) != 0 {
		t.Fatalf("errors: %#v", resp.Errors)
	}
	want := map[string]bool{"__schema": false, "queryType": false, "name": false}
	for _, f := range seen {
		if _, ok := want[f]; ok {
			want[f] = true
		}
	}
	for f, ok := range want {
		if !ok {
			t.Fatalf("expected field hook to observe introspection field %q; saw %v", f, seen)
		}
	}
}

func orderedToMap(v any) any {
	switch node := v.(type) {
	case *core.OrderedObject:
		m := make(map[string]any, len(node.Fields()))
		for _, f := range node.Fields() {
			m[f.Name] = orderedToMap(f.Value)
		}
		return m
	case []any:
		out := make([]any, len(node))
		for i, item := range node {
			out[i] = orderedToMap(item)
		}
		return out
	default:
		return v
	}
}

func TestExecutableIntrospectionBranches(t *testing.T) {
	s, err := schema.Build(schema.Config{Query: thunkQuery{}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	e := New(s, []plugins.Plugin{fieldHookPlugin{}}, WithExecutableIntrospection())
	resp := e.Execute(context.Background(), core.Request{Query: `{ __schema { queryType { name } types { name kind } } }`})
	if len(resp.Errors) != 0 {
		t.Fatalf("introspection errors: %#v", resp.Errors)
	}
	if responseObject(t, resp.Data)["__schema"] == nil {
		t.Fatalf("missing schema data: %#v", resp.Data)
	}

	typeResp := e.Execute(context.Background(), core.Request{Query: `{ __type(name: "String") { name kind } }`})
	if len(typeResp.Errors) != 0 {
		t.Fatalf("type introspection errors: %#v", typeResp.Errors)
	}

	blocked := New(s, []plugins.Plugin{fieldHookPlugin{errField: "name"}}, WithExecutableIntrospection())
	errResp := blocked.Execute(context.Background(), core.Request{Query: `{ __schema { queryType { name } } }`})
	if len(errResp.Errors) == 0 {
		t.Fatal("expected introspection field hook error")
	}

	denied := New(s, nil,
		WithExecutableIntrospection(),
		WithFieldAuthorizer(func(context.Context, FieldAuthorizationContext) error {
			return errors.New("denied")
		}),
	)
	deniedResp := denied.Execute(context.Background(), core.Request{Query: `{ __schema { queryType { name } } }`})
	if len(deniedResp.Errors) == 0 {
		t.Fatal("expected introspection authorization error")
	}
}

func TestExecutableIntrospectionPropagatesMalformedQuery(t *testing.T) {
	ex := New(buildGraphiQLTestSchema(t), nil, WithExecutableIntrospection())
	resp := ex.Execute(context.Background(), core.Request{Query: `{ __schema { queryType bad`})
	if len(resp.Errors) == 0 {
		t.Fatalf("expected malformed introspection parse error, got %+v", resp)
	}
}

func TestExecutableIntrospectionFragmentSpreadInlinesSelections(t *testing.T) {
	ex := New(buildGraphiQLTestSchema(t), nil, WithExecutableIntrospection())
	q := `
		fragment SchemaChunk on Query {
			__schema { queryType { name } types { kind name } }
		}
		query {
			...SchemaChunk
		}`
	resp := ex.Execute(context.Background(), core.Request{Query: q})
	if len(resp.Errors) != 0 {
		t.Fatalf("executable introspection with fragment spread: %#v", resp.Errors)
	}
	chunk := responseObject(t, resp.Data)["__schema"]
	if chunk == nil {
		t.Fatalf("expected __schema in data, got %#v", resp.Data)
	}
	types, ok := responseObject(t, chunk)["types"].([]any)
	if !ok || len(types) < 5 {
		t.Fatalf("expected types list projected from fragment spread: %T %#v", responseObject(t, chunk)["types"], responseObject(t, chunk)["types"])
	}
}

func TestExecutableIntrospectionErrorsWhenMultipleOperationsUntargeted(t *testing.T) {
	ex := New(buildGraphiQLTestSchema(t), nil, WithExecutableIntrospection())
	resp := ex.Execute(context.Background(), core.Request{Query: `query Alpha { __schema { queryType { name } } } query Beta { __schema { queryType { name } } }`})
	if len(resp.Errors) == 0 {
		t.Fatalf("expected ambiguous operation error for introspection executor, got %+v", resp)
	}
}
