package exec

import (
	"context"
	"testing"

	"github.com/patrickkabwe/grx/core"
	"github.com/patrickkabwe/grx/schema"
)

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
				Type:          testDate{},
				Name:          "Date",
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
