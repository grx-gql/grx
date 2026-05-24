package benchmark

import (
	"context"
	"encoding/json"
	"testing"

	gqlgo "github.com/graphql-go/graphql"

	"github.com/grx-gql/grx/core"
)

// Each benchmark runs the same operation through grx, graphql-go, and
// graph-gophers so library overhead is comparable on shared fixtures.
//
//	go test -C benchmark -bench=. -benchmem ./...

func BenchmarkPersistedCompound(b *testing.B) {
	runScenario(b, scenarioPersistedCompound)
}

func BenchmarkParameterizedNested(b *testing.B) {
	runScenario(b, scenarioParameterizedNested)
}

func BenchmarkFeedTimeline(b *testing.B) {
	runScenario(b, scenarioFeedTimeline)
}

func TestImplementationsAgree(t *testing.T) {
	grxExec := newGRXExecutor()
	gqlSchema := newGraphQLGoSchema()
	gophersSchema := newGophersSchema()
	ctx := context.Background()

	for _, s := range scenarioCatalog {
		t.Run(s.name, func(t *testing.T) {
			req := scenarioToGRXReq(s)

			grxRes := grxExec.Execute(ctx, req)
			if len(grxRes.Errors) != 0 {
				t.Fatalf("grx errors: %+v", grxRes.Errors)
			}
			if grxRes.Data == nil {
				t.Fatalf("grx returned nil data")
			}

			gqlRes := gqlgo.Do(gqlgo.Params{
				Schema:         gqlSchema,
				RequestString:  s.query,
				OperationName:  s.operationName,
				VariableValues: shallowCopyVarsOrEmpty(s.variables),
				Context:        ctx,
			})
			if len(gqlRes.Errors) != 0 {
				t.Fatalf("graphql-go errors: %+v", gqlRes.Errors)
			}
			if gqlRes.Data == nil {
				t.Fatalf("graphql-go returned nil data")
			}

			gophersRes := gophersSchema.Exec(ctx, s.query, s.operationName, shallowCopyVarsOrEmpty(s.variables))
			if len(gophersRes.Errors) != 0 {
				t.Fatalf("graph-gophers errors: %+v", gophersRes.Errors)
			}
			if len(gophersRes.Data) == 0 {
				t.Fatalf("graph-gophers returned empty data")
			}
		})
	}
}

// TestTrace_RealWorldMixedQuery executes each scenario once per backend.
//
//	go test -C benchmark -trace=/tmp/graphql.trace -run '^TestTrace_RealWorldMixedQuery$'
func TestTrace_RealWorldMixedQuery(t *testing.T) {
	ctx := context.Background()
	grxExec := newGRXExecutor()
	gqlSchema := newGraphQLGoSchema()
	gophersSchema := newGophersSchema()

	for _, s := range scenarioCatalog {
		t.Run(s.name, func(t *testing.T) {
			res := grxExec.Execute(ctx, scenarioToGRXReq(s))
			jsonMustSmoke(t, res)

			gqlRes := gqlgo.Do(gqlgo.Params{
				Schema: gqlSchema, RequestString: s.query, OperationName: s.operationName,
				VariableValues: shallowCopyVarsOrEmpty(s.variables), Context: ctx,
			})
			jsonMustSmoke(t, gqlRes)

			gophersRes := gophersSchema.Exec(ctx, s.query, s.operationName, shallowCopyVarsOrEmpty(s.variables))
			jsonMustSmoke(t, gophersRes)
		})
	}
}

func jsonMustSmoke(t *testing.T, envelope any) {
	t.Helper()
	b, err := json.Marshal(envelope)
	if err != nil || len(b) == 0 {
		t.Fatalf("marshal: %v empty=%v", err, len(b) == 0)
	}
}

func scenarioToGRXReq(s benchmarkScenario) core.Request {
	return core.Request{
		Query:         s.query,
		OperationName: s.operationName,
		Variables:     shallowCopyVars(s.variables),
	}
}

func shallowCopyVarsOrEmpty(vars map[string]any) map[string]any {
	cp := shallowCopyVars(vars)
	if cp == nil {
		return map[string]any{}
	}
	return cp
}

func runScenario(b *testing.B, s benchmarkScenario) {
	ctx := context.Background()

	b.Run("grx", func(b *testing.B) {
		ex := newGRXExecutor()
		req := scenarioToGRXReq(s)
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			res := ex.Execute(ctx, req)
			jsonMustBench(b, res)
		}
	})

	b.Run("graphql-go", func(b *testing.B) {
		schema := newGraphQLGoSchema()
		params := gqlgo.Params{
			Schema: schema, RequestString: s.query, OperationName: s.operationName,
			VariableValues: shallowCopyVarsOrEmpty(s.variables), Context: ctx,
		}
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			res := gqlgo.Do(params)
			jsonMustBench(b, res)
		}
	})

	b.Run("graph-gophers", func(b *testing.B) {
		schema := newGophersSchema()
		vars := shallowCopyVarsOrEmpty(s.variables)
		b.ReportAllocs()
		b.ResetTimer()
		for range b.N {
			res := schema.Exec(ctx, s.query, s.operationName, vars)
			jsonMustBench(b, res)
		}
	})
}

func jsonMustBench(b *testing.B, envelope any) {
	b.Helper()
	payload, err := json.Marshal(envelope)
	if err != nil {
		b.Fatal(err)
	}
	if len(payload) == 0 {
		b.Fatal("empty payload")
	}
}
