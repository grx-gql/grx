package exec

import (
	"context"
	"testing"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/schema"
)

type tracingUser struct {
	ID   string `gql:"id,nonNull"`
	Name string `gql:"name,nonNull"`
}

type tracingQuery struct{}

func (tracingQuery) User(ctx context.Context, args struct {
	ID string `gql:"id,nonNull"`
}) (*tracingUser, error) {
	return &tracingUser{ID: args.ID, Name: "Ada"}, nil
}

func TestApolloTracingExtension(t *testing.T) {
	s, err := schema.Build(schema.Config{Query: tracingQuery{}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	e := New(s, nil, WithApolloTracing())

	resp := e.Execute(context.Background(), core.Request{Query: `{ user(id: "1") { id name } }`})
	if len(resp.Errors) != 0 {
		t.Fatalf("errors: %#v", resp.Errors)
	}

	tracing, ok := resp.Extensions["tracing"].(map[string]any)
	if !ok {
		t.Fatalf("expected tracing extension, got %#v", resp.Extensions)
	}
	if tracing["version"] != 1 {
		t.Fatalf("version = %v, want 1", tracing["version"])
	}
	if _, ok := tracing["startTime"].(string); !ok {
		t.Fatalf("expected startTime string")
	}
	if d, ok := tracing["duration"].(int64); !ok || d <= 0 {
		t.Fatalf("expected positive duration, got %v", tracing["duration"])
	}
	execution, ok := tracing["execution"].(map[string]any)
	if !ok {
		t.Fatalf("expected execution object")
	}
	resolvers, ok := execution["resolvers"].([]any)
	if !ok || len(resolvers) != 3 { // user, user.id, user.name
		t.Fatalf("expected 3 resolver traces, got %#v", execution["resolvers"])
	}
	first := resolvers[0].(map[string]any)
	for _, key := range []string{"path", "parentType", "fieldName", "returnType", "startOffset", "duration"} {
		if _, ok := first[key]; !ok {
			t.Fatalf("resolver trace missing %q: %#v", key, first)
		}
	}
}

func TestApolloTracingDisabledByDefault(t *testing.T) {
	s, err := schema.Build(schema.Config{Query: tracingQuery{}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	e := New(s, nil)
	resp := e.Execute(context.Background(), core.Request{Query: `{ user(id: "1") { id } }`})
	if _, ok := resp.Extensions["tracing"]; ok {
		t.Fatalf("tracing should be absent without WithApolloTracing")
	}
}
