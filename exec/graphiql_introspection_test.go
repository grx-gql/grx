package exec

import (
	"context"
	"fmt"
	"testing"

	"github.com/patrickkabwe/grx/core"
	"github.com/patrickkabwe/grx/schema"
)

// graphiqlIntrospectionQuery is the standard introspection query shipped by
// graphql-js (getIntrospectionQuery) and used by GraphiQL to populate its
// documentation explorer. Running it verifies compatibility with the tooling
// ecosystem that depends on this exact shape.
const graphiqlIntrospectionQuery = `
query IntrospectionQuery {
  __schema {
    queryType { name }
    mutationType { name }
    subscriptionType { name }
    types {
      ...FullType
    }
    directives {
      name
      description
      locations
      args {
        ...InputValue
      }
    }
  }
}

fragment FullType on __Type {
  kind
  name
  description
  fields(includeDeprecated: true) {
    name
    description
    args {
      ...InputValue
    }
    type {
      ...TypeRef
    }
    isDeprecated
    deprecationReason
  }
  inputFields {
    ...InputValue
  }
  interfaces {
    ...TypeRef
  }
  enumValues(includeDeprecated: true) {
    name
    description
    isDeprecated
    deprecationReason
  }
  possibleTypes {
    ...TypeRef
  }
}

fragment InputValue on __InputValue {
  name
  description
  type { ...TypeRef }
  defaultValue
}

fragment TypeRef on __Type {
  kind
  name
  ofType {
    kind
    name
    ofType {
      kind
      name
      ofType {
        kind
        name
        ofType {
          kind
          name
          ofType {
            kind
            name
            ofType {
              kind
              name
              ofType {
                kind
                name
              }
            }
          }
        }
      }
    }
  }
}
`

func buildGraphiQLTestSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s, err := schema.Build(schema.Config{
		Query: testAdvancedQuery{},
		Scalars: []schema.ScalarConfig{
			{
				Type: testDate{},
				Name: "Date",
				Parse: func(input any) (any, error) {
					value, ok := input.(string)
					if !ok {
						return nil, fmt.Errorf("expected string date input, got %T", input)
					}
					return testDate{Raw: value}, nil
				},
				Serialize: func(value any) (any, error) {
					date, ok := value.(testDate)
					if !ok {
						return nil, fmt.Errorf("expected testDate value, got %T", value)
					}
					return date.Raw, nil
				},
			},
		},
		Enums: []schema.EnumConfig{
			{
				Type: testEpisode(""),
				Name: "Episode",
				Values: []schema.EnumValueConfig{
					{Name: "NEWHOPE", Value: testEpisodeNewHope},
					{Name: "EMPIRE", Value: testEpisodeEmpire},
					{Name: "JEDI", Value: testEpisodeJedi},
				},
			},
		},
		Interfaces: []schema.InterfaceConfig{
			{
				Type:         (*testNode)(nil),
				Implementors: []any{testNodeUser{}, testNodePost{}},
			},
		},
		Unions: []schema.UnionConfig{
			{
				Type:         (*testSearchResult)(nil),
				Name:         "SearchResult",
				Implementors: []any{testNodeUser{}, testNodePost{}},
			},
		},
	})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}
	return s
}

func TestGraphiQLIntrospectionQueryCompatibility(t *testing.T) {
	executor := New(buildGraphiQLTestSchema(t), nil)
	response := executor.Execute(context.Background(), core.Request{Query: graphiqlIntrospectionQuery})

	if len(response.Errors) != 0 {
		t.Fatalf("introspection query returned errors: %#v", response.Errors)
	}

	data := responseObject(t, response.Data)
	schemaData := responseObject(t, data["__schema"])

	queryType := responseObject(t, schemaData["queryType"])
	if name, _ := queryType["name"].(string); name == "" {
		t.Fatalf("expected queryType.name to be populated")
	}

	types, ok := schemaData["types"].([]any)
	if !ok || len(types) == 0 {
		t.Fatalf("expected non-empty types list, got %#v", schemaData["types"])
	}

	// Every type must report a kind; meta types must be present. kind is the
	// __TypeKind enum (a named string type), so compare via formatting rather
	// than a plain string assertion.
	seen := map[string]bool{}
	for _, raw := range types {
		typ, ok := responseObjectValue(raw)
		if !ok {
			t.Fatalf("expected type object, got %T", raw)
		}
		if kind := fmt.Sprintf("%v", typ["kind"]); kind == "" || kind == "<nil>" {
			t.Fatalf("type %v missing kind", typ["name"])
		}
		if name := fmt.Sprintf("%v", typ["name"]); name != "" && name != "<nil>" {
			seen[name] = true
		}
	}
	for _, required := range []string{"__Schema", "__Type", "__Field", "__InputValue", "__EnumValue", "__Directive", "Episode", "SearchResult"} {
		if !seen[required] {
			t.Fatalf("expected introspection to expose type %q; saw %v", required, seen)
		}
	}

	directives, ok := schemaData["directives"].([]any)
	if !ok || len(directives) == 0 {
		t.Fatalf("expected directives, got %#v", schemaData["directives"])
	}
}

func TestGraphiQLIntrospectionDisabledIsRejected(t *testing.T) {
	executor := New(buildGraphiQLTestSchema(t), nil, WithDisableIntrospection())
	response := executor.Execute(context.Background(), core.Request{Query: graphiqlIntrospectionQuery})
	if len(response.Errors) == 0 {
		t.Fatal("expected introspection to be rejected when disabled")
	}
}
