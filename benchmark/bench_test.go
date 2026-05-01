package benchmark

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/graphql-go/graphql"

	"github.com/patrickkabwe/grx/core"
)

// Each benchmark runs the same operation through the three implementations
// as sub-benchmarks. Every iteration goes all the way through to a JSON
// payload so library-level differences in result encoding are included.
//
// This package lives in its own Go module, so invoke `go test` from inside
// `benchmark/` (or prefix with `-C benchmark` from the repo root):
//
//   cd benchmark && go test -bench=. -benchmem ./...
//   go test -C benchmark -bench=. -benchmem ./...
//
// Filter by library:
//   go test -C benchmark -bench=BenchmarkSimpleQuery/grx -benchmem ./...

const (
	simpleQuery = `query { user(id: "user_1") { id name email } }`
	nestedQuery = `query { post(id: "post_1") { id title body author { id name email } } }`
	listQuery   = `query { users(count: 50) { id name email } }`
)

// ---------------------------------------------------------------------------
// Sanity test: every implementation returns a non-empty data payload with no
// errors for each query. Without this, a misconfigured schema could ship a
// fast-but-wrong benchmark.
// ---------------------------------------------------------------------------

func TestImplementationsAgree(t *testing.T) {
	cases := []struct{ name, query string }{
		{"simple", simpleQuery},
		{"nested", nestedQuery},
		{"list", listQuery},
	}

	grxExec := newGRXExecutor()
	gqlSchema := newGraphQLGoSchema()
	gophersSchema := newGophersSchema()
	ctx := context.Background()

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			grxRes := grxExec.Execute(ctx, core.Request{Query: tc.query})
			if len(grxRes.Errors) != 0 {
				t.Fatalf("grx errors: %+v", grxRes.Errors)
			}
			if grxRes.Data == nil {
				t.Fatalf("grx returned nil data")
			}

			gqlRes := graphql.Do(graphql.Params{Schema: gqlSchema, RequestString: tc.query, Context: ctx})
			if len(gqlRes.Errors) != 0 {
				t.Fatalf("graphql-go errors: %+v", gqlRes.Errors)
			}
			if gqlRes.Data == nil {
				t.Fatalf("graphql-go returned nil data")
			}

			gophersRes := gophersSchema.Exec(ctx, tc.query, "", nil)
			if len(gophersRes.Errors) != 0 {
				t.Fatalf("graph-gophers errors: %+v", gophersRes.Errors)
			}
			if len(gophersRes.Data) == 0 {
				t.Fatalf("graph-gophers returned empty data")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkSimpleQuery(b *testing.B) {
	runScenario(b, simpleQuery)
}

func BenchmarkNestedQuery(b *testing.B) {
	runScenario(b, nestedQuery)
}

func BenchmarkListQuery(b *testing.B) {
	runScenario(b, listQuery)
}

// runScenario fans the same query through every library as a sub-benchmark.
// Schemas are constructed once outside the timed region; the inner loop only
// measures execute + JSON encode.
func runScenario(b *testing.B, query string) {
	ctx := context.Background()

	b.Run("grx", func(b *testing.B) {
		exec := newGRXExecutor()
		req := core.Request{Query: query}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			res := exec.Execute(ctx, req)
			payload, err := json.Marshal(res)
			if err != nil {
				b.Fatal(err)
			}
			if len(payload) == 0 {
				b.Fatal("empty payload")
			}
		}
	})

	b.Run("graphql-go", func(b *testing.B) {
		schema := newGraphQLGoSchema()
		params := graphql.Params{Schema: schema, RequestString: query, Context: ctx}
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			res := graphql.Do(params)
			payload, err := json.Marshal(res)
			if err != nil {
				b.Fatal(err)
			}
			if len(payload) == 0 {
				b.Fatal("empty payload")
			}
		}
	})

	b.Run("graph-gophers", func(b *testing.B) {
		schema := newGophersSchema()
		b.ReportAllocs()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			res := schema.Exec(ctx, query, "", nil)
			payload, err := json.Marshal(res)
			if err != nil {
				b.Fatal(err)
			}
			if len(payload) == 0 {
				b.Fatal("empty payload")
			}
		}
	})
}
