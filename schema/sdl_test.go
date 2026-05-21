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
