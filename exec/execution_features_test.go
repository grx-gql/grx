package exec

import (
	"context"
	"sync/atomic"
	"testing"

	"github.com/patrickkabwe/grx/core"
	"github.com/patrickkabwe/grx/schema"
)

type cacheUser struct {
	ID string `gql:"id,nonNull"`
}

type cacheQuery struct{ calls *int64 }

func (q cacheQuery) User(ctx context.Context, args struct {
	ID string `gql:"id,nonNull"`
}) (*cacheUser, error) {
	atomic.AddInt64(q.calls, 1)
	return &cacheUser{ID: args.ID}, nil
}

func TestResolverCacheMemoizesIdenticalCalls(t *testing.T) {
	var calls int64
	s, err := schema.Build(schema.Config{Query: cacheQuery{calls: &calls}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	// Two aliases with identical field+args are not selection-merged, so the
	// resolver would run twice without memoization.
	query := `{ a: user(id: "1") { id } b: user(id: "1") { id } }`

	e := New(s, nil)
	calls = 0
	if resp := e.Execute(context.Background(), core.Request{Query: query}); len(resp.Errors) != 0 {
		t.Fatalf("errors: %#v", resp.Errors)
	}
	if calls != 2 {
		t.Fatalf("without cache expected 2 resolver calls, got %d", calls)
	}

	eCached := New(s, nil, WithResolverCache())
	calls = 0
	if resp := eCached.Execute(context.Background(), core.Request{Query: query}); len(resp.Errors) != 0 {
		t.Fatalf("errors: %#v", resp.Errors)
	}
	if calls != 1 {
		t.Fatalf("with cache expected 1 resolver call, got %d", calls)
	}
}

type thunkQuery struct{}

func (thunkQuery) Slow(ctx context.Context) (string, error)  { return "", nil }
func (thunkQuery) Plain(ctx context.Context) (string, error) { return "", nil }

func TestDeferredResolverThunks(t *testing.T) {
	s, err := schema.Build(schema.Config{Query: thunkQuery{}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	// Override resolvers to return deferred values. The executor must invoke the
	// thunk and use its result as the field value.
	s.Query.Fields["slow"].Resolver = func(ctx context.Context, p schema.ResolveParams) (any, error) {
		return schema.Thunk(func() (any, error) { return "deferred-value", nil }), nil
	}
	s.Query.Fields["plain"].Resolver = func(ctx context.Context, p schema.ResolveParams) (any, error) {
		return func() (any, error) { return "func-thunk", nil }, nil
	}

	e := New(s, nil)
	resp := e.Execute(context.Background(), core.Request{Query: `{ slow plain }`})
	if len(resp.Errors) != 0 {
		t.Fatalf("errors: %#v", resp.Errors)
	}
	data := responseObject(t, resp.Data)
	if data["slow"] != "deferred-value" {
		t.Fatalf("slow = %v, want deferred-value", data["slow"])
	}
	if data["plain"] != "func-thunk" {
		t.Fatalf("plain = %v, want func-thunk", data["plain"])
	}
}

type animalQuery struct{}

type Dog struct {
	Name string `gql:"name,nonNull"`
}

type Cat struct {
	Name string `gql:"name,nonNull"`
}

// Pet is a union (Dog | Cat); abstract-type resolution applies to unions too.
type Pet interface{ isPet() }

func (*Dog) isPet() {}
func (*Cat) isPet() {}

func (animalQuery) Pet(ctx context.Context, args struct {
	Kind string `gql:"kind,nonNull"`
}) (Pet, error) {
	if args.Kind == "cat" {
		return &Cat{Name: "Whiskers"}, nil
	}
	return &Dog{Name: "Rex"}, nil
}

func buildAnimalSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s, err := schema.Build(schema.Config{
		Query: animalQuery{},
		Unions: []schema.UnionConfig{{
			Type:         (*Pet)(nil),
			Name:         "Pet",
			Implementors: []any{Dog{}, Cat{}},
		}},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	return s
}

func TestAbstractTypeResolverHookIsConsulted(t *testing.T) {
	s := buildAnimalSchema(t)

	var called int
	resolver := func(value any) (string, error) {
		called++
		switch value.(type) {
		case *Cat:
			return "Cat", nil
		default:
			return "Dog", nil
		}
	}
	e := New(s, nil, WithAbstractTypeResolver(resolver))

	resp := e.Execute(context.Background(), core.Request{
		Query: `{ pet(kind: "cat") { __typename ... on Cat { name } } }`,
	})
	if len(resp.Errors) != 0 {
		t.Fatalf("errors: %#v", resp.Errors)
	}
	if called == 0 {
		t.Fatal("expected abstract type resolver to be consulted")
	}
	pet := responseObject(t, responseObject(t, resp.Data)["pet"])
	if pet["__typename"] != "Cat" {
		t.Fatalf("__typename = %v, want Cat", pet["__typename"])
	}
	if pet["name"] != "Whiskers" {
		t.Fatalf("name = %v, want Whiskers", pet["name"])
	}
}

func TestAbstractTypeResolverEmptyFallsBack(t *testing.T) {
	s := buildAnimalSchema(t)
	// Returning "" must fall back to default reflection-based resolution.
	e := New(s, nil, WithAbstractTypeResolver(func(any) (string, error) { return "", nil }))
	resp := e.Execute(context.Background(), core.Request{
		Query: `{ pet(kind: "dog") { __typename } }`,
	})
	if len(resp.Errors) != 0 {
		t.Fatalf("errors: %#v", resp.Errors)
	}
	pet := responseObject(t, responseObject(t, resp.Data)["pet"])
	if pet["__typename"] != "Dog" {
		t.Fatalf("__typename = %v, want Dog (fallback)", pet["__typename"])
	}
}
