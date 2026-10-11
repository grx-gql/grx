package exec

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/schema"
)

func BenchmarkQueryConcurrency(b *testing.B) {
	for _, delay := range []time.Duration{0, time.Millisecond} {
		for _, limit := range []int{1, 4} {
			b.Run(delay.String()+"/workers="+strconv.Itoa(limit), func(b *testing.B) {
				compiled, err := schema.Build(schema.Config{Query: introQuery{}})
				if err != nil {
					b.Fatal(err)
				}
				compiled.Query.Fields["work"] = &schema.Field{Name: "work", Type: compiled.Types["String"], Resolver: func(context.Context, schema.ResolveParams) (any, error) {
					if delay > 0 {
						time.Sleep(delay)
					}
					return "ok", nil
				}}
				e := New(compiled, nil, WithDocumentCache(1), WithQueryConcurrency(limit))
				req := core.Request{Query: `{ a:work b:work c:work d:work }`}
				ctx := context.Background()
				if r := e.Execute(ctx, req); len(r.Errors) > 0 {
					b.Fatal(r.Errors)
				}
				b.ReportAllocs()
				b.ResetTimer()
				for b.Loop() {
					if r := e.Execute(ctx, req); len(r.Errors) > 0 {
						b.Fatal(r.Errors)
					}
				}
			})
		}
	}
}
