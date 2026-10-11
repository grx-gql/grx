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
