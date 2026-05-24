package schema

import (
	"context"
	"strings"
	"testing"
)

func TestPrintSDLIncludesExpectedDefinitions(t *testing.T) {
	schemaValue, err := Build(Config{
		Query: buildTestQuery{},
		Enums: []EnumConfig{
			{
				Type: buildTestEpisode(""),
				Name: "Episode",
				Values: []EnumValueConfig{
					{Name: "NEWHOPE", Value: buildTestEpisodeNewHope},
					{Name: "EMPIRE", Value: buildTestEpisodeEmpire},
					{Name: "JEDI", Value: buildTestEpisodeJedi},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	sdl := PrintSDL(schemaValue)
	if !strings.Contains(sdl, "scalar String") {
		t.Fatalf("expected built-in scalar, got:\n%s", sdl)
	}
	if !strings.Contains(sdl, "enum Episode") {
		t.Fatalf("expected enum, got:\n%s", sdl)
	}
	if !strings.Contains(sdl, "type Query") || !strings.Contains(sdl, "user:") {
		t.Fatalf("expected Query root fields, got:\n%s", sdl)
	}
	if !strings.Contains(sdl, "schema {") || !strings.Contains(sdl, "query: Query") {
		t.Fatalf("expected schema block, got:\n%s", sdl)
	}
}

func TestPrintSDLNilSchema(t *testing.T) {
	if PrintSDL(nil) != "" {
		t.Fatal("expected empty string for nil schema")
	}
}

// buildSDLInputQuery is used only for SDL default formatting coverage.
type buildSDLInputQuery struct{}

type buildSDLInput struct {
	Label string `gql:"label,default=hi"`
	Count int    `gql:"count,default=3"`
}

func (buildSDLInputQuery) Echo(ctx context.Context, args struct {
	Input buildSDLInput `gql:"input,nonNull"`
}) (string, error) {
	return args.Input.Label, nil
}

func TestPrintSDLFormatsInputObjectDefaults(t *testing.T) {
	schemaValue, err := Build(Config{Query: buildSDLInputQuery{}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	sdl := PrintSDL(schemaValue)
	if !strings.Contains(sdl, "input buildSDLInput") {
		t.Fatalf("expected input type, got:\n%s", sdl)
	}
	if !strings.Contains(sdl, `label: String = "hi"`) || !strings.Contains(sdl, "count: Int = 3") {
		t.Fatalf("expected literal defaults on input fields, got:\n%s", sdl)
	}
}

// --- Issue #6: Descriptions in SDL output ---

type descSDLQuery struct{}

type descSDLUser struct {
	ID string `gql:"id,nonNull,description=The user ID"`
}

func (descSDLQuery) User(ctx context.Context) (*descSDLUser, error) { return nil, nil }

func TestPrintSDLIncludesFieldDescriptions(t *testing.T) {
	s, err := Build(Config{Query: descSDLQuery{}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	sdl := PrintSDL(s)
	if !strings.Contains(sdl, `"""The user ID"""`) {
		t.Fatalf("expected field description in SDL, got:\n%s", sdl)
	}
}

// --- Issue #6: IsOneOf in SDL output ---

func TestPrintSDLIncludesOneOf(t *testing.T) {
	s := &Schema{
		Query: &Object{TypeName: "Query", Fields: map[string]*Field{
			"q": {Name: "q", Type: &Scalar{TypeName: "String"}},
		}},
		Types: map[string]Type{
			"Query": &Object{TypeName: "Query", Fields: map[string]*Field{
				"q": {Name: "q", Type: &Scalar{TypeName: "String"}},
			}},
			"Contact": &InputObject{TypeName: "Contact", IsOneOf: true, Fields: map[string]*Field{
				"email": {Name: "email", Type: &Scalar{TypeName: "String"}},
			}},
		},
	}
	sdl := PrintSDL(s)
	if !strings.Contains(sdl, "@oneOf") {
		t.Fatalf("expected @oneOf in SDL output, got:\n%s", sdl)
	}
}

// --- Issue #6: @deprecated in SDL output ---

func TestPrintSDLIncludesDeprecatedField(t *testing.T) {
	reason := "Use id instead"
	s := &Schema{
		Query: &Object{TypeName: "Query", Fields: map[string]*Field{
			"q": {Name: "q", Type: &Scalar{TypeName: "String"}},
		}},
		Types: map[string]Type{
			"Query": &Object{TypeName: "Query", Fields: map[string]*Field{
				"q": {Name: "q", Type: &Scalar{TypeName: "String"}},
			}},
			"User": &Object{TypeName: "User", Fields: map[string]*Field{
				"id":       {Name: "id", Type: &Scalar{TypeName: "ID"}},
				"legacyId": {Name: "legacyId", Type: &Scalar{TypeName: "String"}, IsDeprecated: true, DeprecationReason: &reason},
			}},
		},
	}
	sdl := PrintSDL(s)
	if !strings.Contains(sdl, "@deprecated") {
		t.Fatalf("expected @deprecated in SDL output, got:\n%s", sdl)
	}
}

// --- Issue #5: @specifiedBy in SDL output ---

func TestPrintSDLIncludesSpecifiedBy(t *testing.T) {
	s := &Schema{
		Query: &Object{TypeName: "Query", Fields: map[string]*Field{
			"q": {Name: "q", Type: &Scalar{TypeName: "String"}},
		}},
		Types: map[string]Type{
			"Query": &Object{TypeName: "Query", Fields: map[string]*Field{
				"q": {Name: "q", Type: &Scalar{TypeName: "String"}},
			}},
			"URL": &Scalar{TypeName: "URL", SpecifiedByURL: "https://url.spec.whatwg.org/"},
		},
	}
	sdl := PrintSDL(s)
	if !strings.Contains(sdl, "@specifiedBy") {
		t.Fatalf("expected @specifiedBy in SDL output, got:\n%s", sdl)
	}
}

func TestSDLAndDiffBranches(t *testing.T) {
	reason := "old"
	oldSchema := &Schema{Types: map[string]Type{
		"Query": &Object{TypeName: "Query", Fields: map[string]*Field{
			"user": {Name: "user", Type: &Scalar{TypeName: "String"}, Args: []InputValue{{Name: "id", Type: &Scalar{TypeName: "ID"}}}},
		}},
		"Filter": &InputObject{TypeName: "Filter", Fields: map[string]*Field{
			"term": {Name: "term", Type: &Scalar{TypeName: "String"}},
		}},
		"Role": &Enum{TypeName: "Role", Values: []EnumValue{{Name: "ADMIN", IsDeprecated: true, DeprecationReason: &reason}}},
	}}
	newSchema := &Schema{Types: map[string]Type{
		"Node": &Interface{TypeName: "Node", Description: "iface", Fields: map[string]*Field{
			"id": {Name: "id", Type: &NonNull{OfType: &Scalar{TypeName: "ID"}}},
		}},
		"Query": &Object{TypeName: "Query", Fields: map[string]*Field{
			"user": {Name: "user", Type: &Scalar{TypeName: "Int"}, Args: []InputValue{{Name: "id", Type: &NonNull{OfType: &Scalar{TypeName: "ID"}}}, {Name: "limit", Type: &Scalar{TypeName: "Int"}}}},
			"post": {Name: "post", Type: &Scalar{TypeName: "String"}},
		}},
		"Filter": &InputObject{TypeName: "Filter", Fields: map[string]*Field{
			"term":  {Name: "term", Type: &Scalar{TypeName: "String"}},
			"limit": {Name: "limit", Type: &NonNull{OfType: &Scalar{TypeName: "Int"}}},
		}},
		"Role":  &Enum{TypeName: "Role", Values: []EnumValue{{Name: "ADMIN"}, {Name: "USER"}}},
		"Extra": &Scalar{TypeName: "Extra", Description: "line one\nline two", SpecifiedByURL: "https://example.com/scalar"},
	}}

	changes := Diff(oldSchema, newSchema)
	if !HasBreaking(changes) {
		t.Fatalf("expected breaking changes: %#v", changes)
	}
	if changes[0].String() == "" || Breaking.String() != "breaking" || Dangerous.String() != "dangerous" || NonBreaking.String() != "non-breaking" {
		t.Fatal("empty change string")
	}

	sdl := PrintSDL(newSchema)
	for _, want := range []string{"scalar Extra", "type Query", "input Filter", "enum Role", "interface Node"} {
		if !strings.Contains(sdl, want) {
			t.Fatalf("SDL missing %q:\n%s", want, sdl)
		}
	}
	for _, value := range []any{"x", true, false, float64(1.5), int(1), int32(2), int64(3)} {
		if text, ok := FormatSDLDefault(value); !ok || text == "" {
			t.Fatalf("default %T = %q %v", value, text, ok)
		}
	}
	if _, ok := FormatSDLDefault([]string{"x"}); ok {
		t.Fatal("unexpected default format for slice")
	}
}

func TestSDLCoordinateAndDiffEdgeBranches(t *testing.T) {
	reason := "legacy"
	schemaValue := &Schema{
		Query:        &Object{TypeName: "Query"},
		Mutation:     &Object{TypeName: "Mutation"},
		Subscription: &Object{TypeName: "Subscription"},
		Types: map[string]Type{
			"Query": &Object{TypeName: "Query", Interfaces: []*Interface{{TypeName: "Node"}}, Fields: map[string]*Field{
				"node": {Name: "node", Description: "node field", Type: &Scalar{TypeName: "String"}, IsDeprecated: true, DeprecationReason: &reason},
			}},
			"Mutation":     &Object{TypeName: "Mutation", Fields: map[string]*Field{"noop": {Name: "noop", Type: &Scalar{TypeName: "Boolean"}}}},
			"Subscription": &Object{TypeName: "Subscription", Fields: map[string]*Field{"changed": {Name: "changed", Type: &Scalar{TypeName: "String"}}}},
			"Search":       &Union{TypeName: "Search", Types: []*Object{{TypeName: "Query"}}},
			"Node":         &Interface{TypeName: "Node", Fields: map[string]*Field{"id": {Name: "id", Type: &Scalar{TypeName: "ID"}}}},
			"Input": &InputObject{TypeName: "Input", Fields: map[string]*Field{
				"limit": {Name: "limit", Type: &Scalar{TypeName: "Int"}, DefaultValue: int64(3)},
			}},
		},
	}
	sdl := PrintSDL(schemaValue)
	for _, want := range []string{"schema {", "mutation: Mutation", "subscription: Subscription", "union Search", "@deprecated"} {
		if !strings.Contains(sdl, want) {
			t.Fatalf("SDL missing %q:\n%s", want, sdl)
		}
	}
	if PrintSDL(nil) != "" {
		t.Fatal("nil schema should print empty SDL")
	}
	if resolved, err := schemaValue.ResolveCoordinate("Query.node"); err != nil || resolved == nil {
		t.Fatalf("resolve field coordinate = %#v %v", resolved, err)
	}
	for _, coord := range []string{"Query.node(arg)", "Input.limit(arg)", "Search.member"} {
		if _, err := schemaValue.ResolveCoordinate(coord); err == nil {
			t.Fatalf("expected coordinate error for %q", coord)
		}
	}
	if _, err := ParseCoordinate("Bad("); err == nil {
		t.Fatal("expected parse coordinate error")
	}

	changes := Diff(&Schema{Types: map[string]Type{
		"Input": &InputObject{TypeName: "Input", Fields: map[string]*Field{"name": {Name: "name", Type: &Scalar{TypeName: "String"}}}},
	}}, &Schema{Types: map[string]Type{
		"Input": &InputObject{TypeName: "Input", Fields: map[string]*Field{"name": {Name: "name", Type: &Scalar{TypeName: "String"}}, "age": {Name: "age", Type: &Scalar{TypeName: "Int"}}}},
	}})
	if HasBreaking(changes) {
		t.Fatalf("optional input addition should not be breaking: %#v", changes)
	}

	breaking := Diff(&Schema{Types: map[string]Type{
		"Role":   &Enum{TypeName: "Role", Values: []EnumValue{{Name: "ADMIN"}, {Name: "USER"}}},
		"Search": &Union{TypeName: "Search", Types: []*Object{{TypeName: "Query"}, {TypeName: "Mutation"}}},
	}}, &Schema{Types: map[string]Type{
		"Role":   &Enum{TypeName: "Role", Values: []EnumValue{{Name: "ADMIN"}}},
		"Search": &Union{TypeName: "Search", Types: []*Object{{TypeName: "Query"}}},
	}})
	if !HasBreaking(breaking) {
		t.Fatalf("expected enum/union breaking changes: %#v", breaking)
	}
	if typeNameOf(&NonNull{OfType: &List{OfType: &Scalar{TypeName: "String"}}}) != "[String]!" {
		t.Fatal("wrapped type name mismatch")
	}
	if !isRequiredInput(&NonNull{OfType: &Scalar{TypeName: "String"}}, nil) {
		t.Fatal("non-null without default should be required")
	}
	if isRequiredInput(&NonNull{OfType: &Scalar{TypeName: "String"}}, "x") {
		t.Fatal("non-null with default should not be required")
	}
}
