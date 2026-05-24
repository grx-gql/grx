package exec

import (
	"context"
	"fmt"
	"testing"

	"github.com/patrickkabwe/grx/core"
	"github.com/patrickkabwe/grx/schema"
)

func TestIntrospectionLiteralFormattingBranches(t *testing.T) {
	stringType := &schema.Scalar{TypeName: "String"}
	intType := &schema.Scalar{TypeName: "Int"}
	floatType := &schema.Scalar{TypeName: "Float"}
	boolType := &schema.Scalar{TypeName: "Boolean"}
	idType := &schema.Scalar{TypeName: "ID"}
	enumType := &schema.Enum{TypeName: "Role"}
	inputType := &schema.InputObject{TypeName: "Input", Fields: map[string]*schema.Field{
		"name": {Name: "name", Type: stringType},
		"ids":  {Name: "ids", Type: &schema.List{OfType: idType}},
	}}

	cases := []struct {
		typ   schema.Type
		value any
		want  string
	}{
		{stringType, "Ada", `"Ada"`},
		{intType, int64(3), "3"},
		{floatType, float32(1.5), "1.5"},
		{floatType, int16(2), "2"},
		{boolType, true, "true"},
		{idType, uint16(7), "7"},
		{idType, "x", `"x"`},
		{enumType, "RAW", "RAW"},
		{&schema.List{OfType: intType}, []int{1, 2}, "[1, 2]"},
		{inputType, map[string]any{"name": "Ada", "ids": []string{"1", "2"}}, `{ids: ["1", "2"], name: "Ada"}`},
		{&schema.NonNull{OfType: stringType}, "Ada", `"Ada"`},
	}
	for _, tc := range cases {
		got, ok := formatGraphQLValueLiteral(tc.typ, tc.value)
		if !ok || got != tc.want {
			t.Fatalf("format %T %#v = %q %v, want %q", tc.typ, tc.value, got, ok, tc.want)
		}
	}
	for _, tc := range []struct {
		typ   schema.Type
		value any
	}{
		{stringType, 1},
		{floatType, "x"},
		{idType, struct{}{}},
		{&schema.List{OfType: intType}, "x"},
		{inputType, "x"},
		{inputType, map[string]any{"name": 1}},
		{&schema.Object{TypeName: "Obj"}, "x"},
	} {
		if got, ok := formatGraphQLValueLiteral(tc.typ, tc.value); ok {
			t.Fatalf("unexpected format %q for %#v", got, tc.value)
		}
	}
	if introspectionDefaultValue(stringType, nil) != nil {
		t.Fatal("nil default should format to nil")
	}
	if got := formatDefaultValue("x"); got != `"x"` {
		t.Fatalf("formatDefaultValue = %q", got)
	}
	if got := nullableString("desc"); got != "desc" {
		t.Fatalf("nullableString = %#v", got)
	}
	if formatDefaultValue([]string{"x"}) != nil {
		t.Fatal("unsupported default should be nil")
	}
	ref := introspectionTypeRef(&schema.NonNull{OfType: &schema.List{OfType: stringType}})
	if ref == nil {
		t.Fatal("expected wrapped introspection ref")
	}
	introspectionSchemaValue := &schema.Schema{Types: map[string]schema.Type{"String": stringType}}
	if introspectionNamedType(introspectionSchemaValue, core.Request{Variables: map[string]any{"name": "String"}}, false) == nil {
		t.Fatal("expected named introspection type from variables")
	}
	if name, ok := introspectionTypeName(core.Request{Query: `{ __type(name: "String") { name } }`}); !ok || name != "String" {
		t.Fatalf("type name = %q %v", name, ok)
	}
	if _, ok := introspectionTypeName(core.Request{Query: `{ __type(name: bad) { name } }`}); ok {
		t.Fatal("expected invalid literal type name")
	}
}

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

type introQuery struct{}

func (introQuery) Ping(ctx context.Context) (string, error) {
	return "pong", nil
}

func TestIntrospectionBuiltinDirectives(t *testing.T) {
	schemaValue, err := schema.Build(schema.Config{Query: introQuery{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	executor := New(schemaValue, nil)
	response := executor.Execute(context.Background(), core.Request{
		Query: `{ __schema { directives { name locations args { name } } } }`,
	})
	if len(response.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", response.Errors)
	}

	data := responseObject(t, response.Data)
	schemaData := responseObject(t, data["__schema"])
	directives, ok := schemaData["directives"].([]any)
	if !ok || len(directives) < 2 {
		t.Fatalf("expected built-in directives, got %#v", schemaData["directives"])
	}

	names := map[string]bool{}
	for _, raw := range directives {
		dir, ok := responseObjectValue(raw)
		if !ok {
			t.Fatalf("expected directive object, got %T", raw)
		}
		name, _ := dir["name"].(string)
		names[name] = true
	}
	if !names["skip"] || !names["include"] {
		t.Fatalf("expected skip and include directives, got %#v", names)
	}
}

func TestIntrospectionDefaultValuesAreGraphQLLiterals(t *testing.T) {
	schemaValue, err := schema.Build(schema.Config{Query: introQuery{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	executor := New(schemaValue, nil)
	response := executor.Execute(context.Background(), core.Request{
		Query: `{ __schema { directives { name args { name defaultValue } } } }`,
	})
	if len(response.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", response.Errors)
	}

	data := responseObject(t, response.Data)
	schemaData := responseObject(t, data["__schema"])
	directives := schemaData["directives"].([]any)
	for _, rawDirective := range directives {
		directive := responseObject(t, rawDirective)
		if directive["name"] != "deprecated" {
			continue
		}
		args := directive["args"].([]any)
		reason := responseObject(t, args[0])
		if reason["defaultValue"] != `"No longer supported"` {
			t.Fatalf("expected quoted GraphQL literal, got %#v", reason["defaultValue"])
		}
		return
	}
	t.Fatal("expected deprecated directive")
}

func TestIntrospectionOmitsDeprecatedFieldsByDefault(t *testing.T) {
	schemaValue, err := schema.Build(schema.Config{Query: introQuery{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	queryType := schemaValue.Query
	queryType.Fields["legacy"] = &schema.Field{
		Name:         "legacy",
		Type:         schemaValue.Types["String"],
		IsDeprecated: true,
		Resolver: func(ctx context.Context, params schema.ResolveParams) (any, error) {
			return "old", nil
		},
	}

	executor := New(schemaValue, nil)
	response := executor.Execute(context.Background(), core.Request{
		Query: `{ __type(name: "Query") { fields { name } } }`,
	})
	if len(response.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", response.Errors)
	}

	data := responseObject(t, response.Data)
	typeData := responseObject(t, data["__type"])
	fields, ok := typeData["fields"].([]any)
	if !ok {
		t.Fatalf("expected fields list, got %T", typeData["fields"])
	}
	for _, raw := range fields {
		field, ok := responseObjectValue(raw)
		if !ok {
			continue
		}
		if field["name"] == "legacy" {
			t.Fatal("expected deprecated field omitted by default")
		}
	}
}

func TestIntrospectionIncludesDeprecatedFieldsWhenRequested(t *testing.T) {
	schemaValue, err := schema.Build(schema.Config{Query: introQuery{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	queryType := schemaValue.Query
	queryType.Fields["legacy"] = &schema.Field{
		Name:         "legacy",
		Type:         schemaValue.Types["String"],
		IsDeprecated: true,
		Resolver: func(ctx context.Context, params schema.ResolveParams) (any, error) {
			return "old", nil
		},
	}

	executor := New(schemaValue, nil)
	response := executor.Execute(context.Background(), core.Request{
		Query: `{ __type(name: "Query") { fields(includeDeprecated: true) { name isDeprecated } } }`,
	})
	if len(response.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", response.Errors)
	}

	data := responseObject(t, response.Data)
	typeData := responseObject(t, data["__type"])
	fields, ok := typeData["fields"].([]any)
	if !ok {
		t.Fatalf("expected fields list, got %T", typeData["fields"])
	}

	foundLegacy := false
	for _, raw := range fields {
		field, ok := responseObjectValue(raw)
		if !ok {
			continue
		}
		if field["name"] == "legacy" {
			foundLegacy = true
			if field["isDeprecated"] != true {
				t.Fatalf("expected legacy field deprecated, got %#v", field["isDeprecated"])
			}
		}
	}
	if !foundLegacy {
		t.Fatal("expected legacy field when includeDeprecated is true")
	}
}

func TestIntrospectionRegistersMetaTypes(t *testing.T) {
	schemaValue, err := schema.Build(schema.Config{Query: introQuery{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	for _, name := range []string{"__Schema", "__Type", "__Field", "__InputValue", "__EnumValue", "__Directive", "__TypeKind", "__DirectiveLocation"} {
		if _, ok := schemaValue.Types[name]; !ok {
			t.Fatalf("expected introspection meta type %q in schema registry", name)
		}
	}
}

func TestIntrospectionScalarSpecifiedByURL(t *testing.T) {
	schemaValue, err := schema.Build(schema.Config{
		Query: introQuery{},
		Scalars: []schema.ScalarConfig{
			{
				Type:           testDate{},
				Name:           "Date",
				SpecifiedByURL: "https://example.com/date",
				Parse: func(input any) (any, error) {
					return testDate{Raw: input.(string)}, nil
				},
				Serialize: func(value any) (any, error) {
					return value.(testDate).Raw, nil
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	executor := New(schemaValue, nil)
	response := executor.Execute(context.Background(), core.Request{
		Query: `{ __type(name: "Date") { kind name specifiedByURL } }`,
	})
	if len(response.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", response.Errors)
	}

	data := responseObject(t, response.Data)
	typeData := responseObject(t, data["__type"])
	if typeData["specifiedByURL"] != "https://example.com/date" {
		t.Fatalf("expected specifiedByURL, got %#v", typeData["specifiedByURL"])
	}
}

func TestIntrospectionUnknownNamedTypeYieldsNil(t *testing.T) {
	executor := New(buildGraphiQLTestSchema(t), nil)
	response := executor.Execute(context.Background(), core.Request{
		Query: `{ __type(name: "ZZZUnknownTypeZZZ") { name } }`,
	})
	if len(response.Errors) != 0 {
		t.Fatalf("unexpected execute errors for unknown type introspection: %#v", response.Errors)
	}
	data := responseObject(t, response.Data)
	if data["__type"] != nil {
		t.Fatalf("expected nil __type, got %#v", data["__type"])
	}
}

func TestIntrospectionNamedTypeUnsetWhenVariablesNameIsWrongType(t *testing.T) {
	ex := New(buildGraphiQLTestSchema(t), nil)
	response := ex.Execute(context.Background(), core.Request{
		Query:     `{ __type(name: "Episode") { name } }`,
		Variables: map[string]any{"name": struct{}{}},
	})
	if len(response.Errors) != 0 {
		t.Fatalf("unexpected errors when variable name shadows query literal: %#v", response.Errors)
	}
	data := responseObject(t, response.Data)
	if data["__type"] != nil {
		t.Fatalf("__type variables branch should fail string cast and omit type, got %#v", data["__type"])
	}
}
