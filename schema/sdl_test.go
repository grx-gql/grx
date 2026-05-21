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
