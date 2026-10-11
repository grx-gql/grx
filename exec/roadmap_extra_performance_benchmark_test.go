package exec

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/schema"
)

// This harness is identical on the pre-audit and audit refs. Introspection
// explicitly uses the baseline executable option: its default shortcut ignored
// exact selections and is not a semantically equivalent comparison.
type roadmapPerfNode struct {
	ID     string
	Count  int
	Labels []string
}

type roadmapPerfQuery struct{}

func (roadmapPerfQuery) Node(context.Context) (*roadmapPerfNode, error) {
	return &roadmapPerfNode{ID: "node", Count: 7, Labels: []string{"a", "b"}}, nil
}
func (roadmapPerfQuery) Ping(context.Context) (string, error) { return "pong", nil }

type roadmapPerfIntrospectionCase struct{ name, query string }

func roadmapPerfIntrospectionCases() []roadmapPerfIntrospectionCase {
	return []roadmapPerfIntrospectionCase{
		{"schema_root", `{ __schema { queryType { name } } }`},
		{"schema_type_names", `{ __schema { types { kind name } } }`},
		{"named_type", `{ __type(name:"roadmapPerfNode") { kind name } }`},
		{"named_type_fields", `{ __type(name:"roadmapPerfNode") { name fields { name type { kind name ofType { kind name } } } } }`},
		{"root_and_named_fragment", `{ __schema { queryType { name } } __type(name:"roadmapPerfNode") { ...Fields } } fragment Fields on __Type { name fields { name type { kind name ofType { kind name } } } }`},
	}
}

func roadmapPerfIntrospectionExecutor(tb testing.TB, cacheSize int) *Executor {
	tb.Helper()
	compiled, err := schema.Build(schema.Config{Query: roadmapPerfQuery{}})
	if err != nil {
		tb.Fatal(err)
	}
	return New(compiled, nil, WithExecutableIntrospection(), WithDocumentCache(cacheSize))
}

func BenchmarkRoadmapIntrospection(b *testing.B) {
	for _, scenario := range roadmapPerfIntrospectionCases() {
		for _, cacheSize := range []int{0, 1} {
			b.Run(fmt.Sprintf("%s/document_cache_%d", scenario.name, cacheSize), func(b *testing.B) {
				executor := roadmapPerfIntrospectionExecutor(b, cacheSize)
				request := core.Request{Query: scenario.query}
				ctx := context.Background()
				roadmapPerfResponse(b, executor.Execute(ctx, request))
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					if response := executor.Execute(ctx, request); len(response.Errors) != 0 {
						b.Fatal(response.Errors)
					}
				}
			})
		}
	}
}

type roadmapPerfCacheCase struct {
	name, field string
	nested      bool
}

func roadmapPerfCacheCases() []roadmapPerfCacheCase {
	return []roadmapPerfCacheCase{
		{name: "no_arguments", field: "value"},
		{name: "scalar_argument", field: "value(id:7)"},
		{name: "object_argument", field: `value(options:{prefix:"x",count:7,tags:["a","b"]})`},
		{name: "pointer_source", field: "value(id:7)", nested: true},
	}
}

func roadmapPerfCacheExecutor(tb testing.TB, scenario roadmapPerfCacheCase, cacheEnabled bool) (*Executor, core.Request, *int) {
	tb.Helper()
	compiled, err := ParseSDL(`input Options { prefix:String count:Int tags:[String] } type Query { value(id:Int,options:Options):Int node:Node } type Node { value(id:Int,options:Options):Int }`)
	if err != nil {
		tb.Fatal(err)
	}
	calls := new(int)
	resolver := func(context.Context, schema.ResolveParams) (any, error) { *calls++; return 7, nil }
	compiled.Query.Fields["value"].Resolver = resolver
	compiled.Types["Node"].(*schema.Object).Fields["value"].Resolver = resolver
	node := &roadmapPerfNode{Count: 7}
	compiled.Query.Fields["node"].Resolver = func(context.Context, schema.ResolveParams) (any, error) { return node, nil }
	var query strings.Builder
	query.WriteString("{")
	if scenario.nested {
		query.WriteString("node {")
	}
	for index := 0; index < 16; index++ {
		fmt.Fprintf(&query, " a%d:%s", index, scenario.field)
	}
	if scenario.nested {
		query.WriteString(" }")
	}
	query.WriteString(" }")
	options := []ExecutorOption{WithDocumentCache(1)}
	if cacheEnabled {
		options = append(options, WithResolverCache())
	}
	return New(compiled, nil, options...), core.Request{Query: query.String()}, calls
}

func BenchmarkRoadmapResolverCache(b *testing.B) {
	for _, scenario := range roadmapPerfCacheCases() {
		for _, cacheEnabled := range []bool{false, true} {
			b.Run(fmt.Sprintf("%s/resolver_cache_%t", scenario.name, cacheEnabled), func(b *testing.B) {
				executor, request, calls := roadmapPerfCacheExecutor(b, scenario, cacheEnabled)
				ctx := context.Background()
				roadmapPerfResponse(b, executor.Execute(ctx, request))
				expectedCalls := 16
				if cacheEnabled {
					expectedCalls = 1
				}
				if *calls != expectedCalls {
					b.Fatalf("resolver calls=%d want%d", *calls, expectedCalls)
				}
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					if response := executor.Execute(ctx, request); len(response.Errors) != 0 {
						b.Fatal(response.Errors)
					}
				}
			})
		}
	}
}

func BenchmarkRoadmapResolverCacheKey(b *testing.B) {
	field := &schema.Field{Name: "value"}
	node := &roadmapPerfNode{Count: 7}
	for _, scenario := range []struct {
		name string
		args map[string]any
	}{
		{name: "none"},
		{name: "scalar", args: map[string]any{"id": 7}},
		{name: "object", args: map[string]any{"options": map[string]any{"prefix": "x", "count": 7, "tags": []any{"a", "b"}}}},
	} {
		for _, source := range []struct {
			name  string
			value any
		}{{name: "root"}, {name: "pointer", value: node}} {
			b.Run(scenario.name+"/"+source.name, func(b *testing.B) {
				b.ReportAllocs()
				for b.Loop() {
					if key, ok := resolverCacheKey(field, source.value, scenario.args); !ok || key == "" {
						b.Fatal("cache key unexpectedly unavailable")
					}
				}
			})
		}
	}
}

func roadmapPerfResponse(tb testing.TB, response core.Response) []byte {
	tb.Helper()
	if len(response.Errors) != 0 {
		tb.Fatal(response.Errors)
	}
	encoded, err := json.Marshal(response)
	if err != nil {
		tb.Fatal(err)
	}
	return encoded
}

// Compare these hashes between refs before using timing results. Matching
// complete serialized responses avoids comparing different introspection work.
func TestRoadmapExtraBenchmarkSemantics(t *testing.T) {
	ctx := context.Background()
	for _, scenario := range roadmapPerfIntrospectionCases() {
		executor := roadmapPerfIntrospectionExecutor(t, 1)
		encoded := roadmapPerfResponse(t, executor.Execute(ctx, core.Request{Query: scenario.query}))
		t.Logf("semantic %s %x", scenario.name, sha256.Sum256(encoded))
	}
	for _, scenario := range roadmapPerfCacheCases() {
		var uncached string
		for _, enabled := range []bool{false, true} {
			executor, request, calls := roadmapPerfCacheExecutor(t, scenario, enabled)
			encoded := roadmapPerfResponse(t, executor.Execute(ctx, request))
			expectedCalls := 16
			if enabled {
				expectedCalls = 1
			}
			if *calls != expectedCalls {
				t.Fatalf("%s cache%t calls=%d want%d", scenario.name, enabled, *calls, expectedCalls)
			}
			if !enabled {
				uncached = string(encoded)
			} else if string(encoded) != uncached {
				t.Fatalf("cache changed %s response", scenario.name)
			}
		}
		t.Logf("semantic resolver_%s %x", scenario.name, sha256.Sum256([]byte(uncached)))
	}
}
