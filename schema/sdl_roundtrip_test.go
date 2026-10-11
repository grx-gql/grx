package schema_test

import (
	"context"
	"github.com/grx-gql/grx/exec"
	"github.com/grx-gql/grx/schema"
	"testing"
)

type roadmapQuery struct{}

func (roadmapQuery) Ping(context.Context) (string, error) { return "pong", nil }

func TestRoadmapAuditBuiltSchemaSDLParses(t *testing.T) {
	s, err := schema.Build(schema.Config{Query: roadmapQuery{}})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exec.ParseSDL(schema.PrintSDL(s)); err != nil {
		t.Fatalf("printer output from built schema cannot be parsed: %v", err)
	}
}
