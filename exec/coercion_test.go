package exec

import (
	"context"
	"testing"

	"github.com/patrickkabwe/grx/core"
	"github.com/patrickkabwe/grx/schema"
)

type listCoercionQuery struct{}

func (listCoercionQuery) Echo(ctx context.Context, args struct {
	Names []string `gql:"names,nonNull"`
}) ([]string, error) {
	return args.Names, nil
}

func TestListInputCoercionFromJSONVariables(t *testing.T) {
	schemaValue, err := schema.Build(schema.Config{Query: listCoercionQuery{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	executor := New(schemaValue, nil)
	response := executor.Execute(context.Background(), core.Request{
		Query:     `query($names: [String!]!) { echo(names: $names) }`,
		Variables: map[string]any{"names": []any{"a", "b"}},
	})
	if len(response.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", response.Errors)
	}

	data := responseObject(t, response.Data)
	got, ok := data["echo"].([]any)
	if !ok {
		t.Fatalf("expected list result, got %T", data["echo"])
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("unexpected echo result: %#v", got)
	}
}

type scalarCoercionQuery struct{}

func (scalarCoercionQuery) Count(ctx context.Context) (int, error) {
	return 42, nil
}

func TestScalarResultCoercionSerializesBuiltInInt(t *testing.T) {
	schemaValue, err := schema.Build(schema.Config{Query: scalarCoercionQuery{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	executor := New(schemaValue, nil)
	response := executor.Execute(context.Background(), core.Request{
		Query: `{ count }`,
	})
	if len(response.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", response.Errors)
	}

	data := responseObject(t, response.Data)
	switch v := data["count"].(type) {
	case int:
		if v != 42 {
			t.Fatalf("expected 42, got %d", v)
		}
	case int64:
		if v != 42 {
			t.Fatalf("expected 42, got %d", v)
		}
	case float64:
		if v != 42 {
			t.Fatalf("expected 42, got %v", v)
		}
	default:
		t.Fatalf("expected numeric count, got %T %#v", data["count"], data["count"])
	}
}
