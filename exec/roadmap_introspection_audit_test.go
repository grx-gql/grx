package exec

import (
	"context"
	"strings"
	"testing"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/schema"
)

func TestRoadmapIntrospectionMetaTypesHaveFieldsAndEnums(t *testing.T) {
	for _, test := range []struct{ name, query, want string }{
		{"type fields", `{ __type(name: "__Type") { fields { name } } }`, `"name":"kind"`},
		{"type kind enum", `{ __type(name: "__TypeKind") { enumValues { name } } }`, `"name":"SCALAR"`},
		{"directive location enum", `{ __type(name: "__DirectiveLocation") { enumValues { name } } }`, `"name":"FIELD"`},
	} {
		t.Run(test.name, func(t *testing.T) {
			r := New(roadmapAuditSchema(t), nil, WithExecutableIntrospection()).Execute(context.Background(), core.Request{Query: test.query})
			if got := roadmapAuditJSON(t, r.Data); !strings.Contains(got, test.want) {
				t.Fatalf("introspection metadata must include %s; got %s", test.want, got)
			}
		})
	}
}

func TestRoadmapIntrospectionCustomDirective(t *testing.T) {
	s := roadmapAuditSchema(t)
	s.DirectiveDefinitions = map[string]*schema.DirectiveDefinition{"audit": {Name: "audit", Locations: []string{"FIELD"}, Description: "Audit", IsRepeatable: true}}
	r := New(s, nil, WithExecutableIntrospection()).Execute(context.Background(), core.Request{Query: `{ __schema { directives { name } } }`})
	if got := roadmapAuditJSON(t, r.Data); !strings.Contains(got, `"name":"audit"`) {
		t.Fatalf("registered custom directives must be introspected; got %s", got)
	}
}

func TestRoadmapIntrospectionInputValueDeprecation(t *testing.T) {
	s := roadmapAuditSchema(t)
	s.Query.Fields["ok"].Args = []schema.InputValue{{Name: "old", Type: s.Types["String"], IsDeprecated: true}}
	r := New(s, nil, WithExecutableIntrospection()).Execute(context.Background(), core.Request{Query: `{ __type(name: "Query") { fields { name args(includeDeprecated: true) { name isDeprecated } } } }`})
	if got := roadmapAuditJSON(t, r.Data); !strings.Contains(got, `"name":"old","isDeprecated":true`) {
		t.Fatalf("input value metadata must preserve deprecation; got %s", got)
	}
}
func TestRoadmapIntrospectionRespectsAliasAndSelection(t *testing.T) {
	s := roadmapAuditSchema(t)
	r := New(s, nil).Execute(context.Background(), core.Request{Query: `{ root: __schema { queryType { name } } }`})
	if got := roadmapAuditJSON(t, r.Data); got != `{"root":{"queryType":{"name":"Query"}}}` {
		t.Fatalf("introspection must honor selected fields and aliases; got %s", got)
	}
}

func TestRoadmapExecutableIntrospectionMixedRootFields(t *testing.T) {
	s := roadmapAuditSchema(t)
	r := New(s, nil, WithExecutableIntrospection()).Execute(context.Background(), core.Request{Query: `{ ok __schema { queryType { name } } }`})
	if got := roadmapAuditJSON(t, r.Data); got != `{"ok":"ok","__schema":{"queryType":{"name":"Query"}}}` {
		t.Fatalf("mixed introspection must execute ordinary root fields; got %s", got)
	}
}

func TestRoadmapIntrospectionTypeArgumentsPerSelection(t *testing.T) {
	s := roadmapAuditSchema(t)
	r := New(s, nil, WithExecutableIntrospection()).Execute(context.Background(), core.Request{Query: `{ a: __type(name: "String") { name } b: __type(name: "Int") { name } }`})
	if got := roadmapAuditJSON(t, r.Data); got != `{"a":{"name":"String"},"b":{"name":"Int"}}` {
		t.Fatalf("each __type selection must use its own arguments; got %s", got)
	}
}

func TestRoadmapIntrospectionAllowsWhitespaceBeforeArguments(t *testing.T) {
	s := roadmapAuditSchema(t)
	r := New(s, nil, WithExecutableIntrospection()).Execute(context.Background(), core.Request{Query: `{ __type (name: "String") { name } }`})
	if len(r.Errors) != 0 || roadmapAuditJSON(t, r.Data) != `{"__type":{"name":"String"}}` {
		t.Fatalf("ignored whitespace must not change introspection semantics; got %s", roadmapAuditJSON(t, r))
	}
}

func TestRoadmapIntrospectionNestedTypename(t *testing.T) {
	s := roadmapAuditSchema(t)
	r := New(s, nil, WithExecutableIntrospection()).Execute(context.Background(), core.Request{Query: `{ __schema { __typename queryType { __typename } } }`})
	if got := roadmapAuditJSON(t, r.Data); got != `{"__schema":{"__typename":"__Schema","queryType":{"__typename":"__Type"}}}` {
		t.Fatalf("nested introspection __typename must identify actual type; got %s", got)
	}
}

func TestRoadmapIntrospectionDeprecationArgumentsAreLocal(t *testing.T) {
	s := roadmapAuditSchema(t)
	s.Query.Fields["ok"].IsDeprecated = true
	r := New(s, nil, WithExecutableIntrospection()).Execute(context.Background(), core.Request{Query: `{ __type(name: "Query") { hidden: fields { name } shown: fields(includeDeprecated: true) { name } } }`})
	data := responseObject(t, responseObject(t, r.Data)["__type"])
	if got := roadmapAuditJSON(t, data["hidden"]); strings.Contains(got, `"name":"ok"`) {
		t.Fatalf("includeDeprecated on sibling selection must not affect omitted argument; got %s", got)
	}
}

func TestRoadmapExecutableIntrospectionRequestErrorOmitsData(t *testing.T) {
	r := New(roadmapAuditSchema(t), nil, WithExecutableIntrospection()).Execute(context.Background(), core.Request{Query: `{ __schema {`})
	if len(r.Errors) == 0 || r.Data != nil || r.DataNull {
		t.Fatalf("malformed introspection request must omit data; got %s", roadmapAuditJSON(t, r))
	}
}

func TestRoadmapIntrospectionRejectsCyclicFragments(t *testing.T) {
	r := New(roadmapAuditSchema(t), nil).Execute(context.Background(), core.Request{Query: `{ __schema { ...Cycle } } fragment Cycle on __Schema { ...Cycle }`})
	if len(r.Errors) == 0 || r.Data != nil || r.DataNull {
		t.Fatalf("cyclic introspection must fail before execution: %s", roadmapAuditJSON(t, r))
	}
}

func TestRoadmapIntrospectionDisableUsesSelections(t *testing.T) {
	e := New(roadmapAuditSchema(t), nil, WithDisableIntrospection())
	r := e.Execute(context.Background(), core.Request{Query: `{ ok # __schema
 }`})
	if len(r.Errors) != 0 || roadmapAuditJSON(t, r.Data) != `{"ok":"ok"}` {
		t.Fatalf("comment must not trigger introspection disable: %s", roadmapAuditJSON(t, r))
	}
	r = e.Execute(context.Background(), core.Request{Query: `{ ...S } fragment S on Query { __schema { queryType { name } } }`})
	if len(r.Errors) == 0 || r.Data != nil {
		t.Fatalf("reachable fragment must honor introspection disable: %s", roadmapAuditJSON(t, r))
	}
}

func TestRoadmapIntrospectionInputMetadataAndLocalFiltering(t *testing.T) {
	s := roadmapAuditSchema(t)
	reason := "replaced"
	input := &schema.InputObject{TypeName: "AuditInput", Fields: map[string]*schema.Field{"old": {Name: "old", Type: s.Types["String"], IsDeprecated: true, DeprecationReason: &reason}, "current": {Name: "current", Type: s.Types["String"]}}}
	s.Types[input.Name()] = input
	r := New(s, nil).Execute(context.Background(), core.Request{Query: `{ __type(name:"AuditInput") { isOneOf interfaces { name } inputFields { name } all:inputFields(includeDeprecated:true) { name isDeprecated deprecationReason } } }`})
	want := `{"__type":{"isOneOf":false,"interfaces":null,"inputFields":[{"name":"current"}],"all":[{"name":"current","isDeprecated":false,"deprecationReason":null},{"name":"old","isDeprecated":true,"deprecationReason":"replaced"}]}}`
	if got := roadmapAuditJSON(t, r.Data); got != want || len(r.Errors) != 0 {
		t.Fatalf("got %s errors=%+v", got, r.Errors)
	}
}

func TestRoadmapIntrospectionDirectiveDefinitionsAreUnique(t *testing.T) {
	s := roadmapAuditSchema(t)
	s.DirectiveDefinitions = map[string]*schema.DirectiveDefinition{"skip": {Name: "skip", Description: "registered", Locations: []string{"FIELD"}}}
	r := New(s, nil).Execute(context.Background(), core.Request{Query: `{ __schema { directives { name description } } }`})
	got := roadmapAuditJSON(t, r.Data)
	if len(r.Errors) != 0 || strings.Count(got, `"name":"skip"`) != 1 || !strings.Contains(got, `"description":"registered"`) {
		t.Fatalf("definitions must be unique: %s %+v", got, r.Errors)
	}
}

func TestRoadmapIntrospectionDeprecatedArgumentIsNonNullBoolean(t *testing.T) {
	s := roadmapAuditSchema(t)
	e := New(s, nil)
	field := e.Schema.Types["__Type"].(*schema.Object).Fields["fields"]
	if field.Args[0].Type.Kind() != schema.NonNullKind || field.Args[0].Type.(*schema.NonNull).OfType.Name() != "Boolean" || field.Args[0].DefaultValue != false {
		t.Fatalf("unexpected includeDeprecated definition: %+v", field.Args[0])
	}
	r := e.Execute(context.Background(), core.Request{Query: `{ __type(name:"Query") { fields(includeDeprecated:null) { name } } }`})
	if len(r.Errors) == 0 || r.Data != nil {
		t.Fatalf("September 2025 non-null argument must reject null: %+v", r)
	}
}
