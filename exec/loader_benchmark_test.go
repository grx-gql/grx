package exec

import (
	"context"
	"strconv"
	"testing"
	"time"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/loader"
	"github.com/grx-gql/grx/schema"
)

func BenchmarkLoaderList(b *testing.B) {
	for _, latency := range []time.Duration{0, time.Millisecond} {
		b.Run(latency.String(), func(b *testing.B) {
			for _, mode := range []string{"individual", "batched"} {
				b.Run(mode, func(b *testing.B) {
					s, err := schema.Build(schema.Config{Query: thunkQuery{}})
					if err != nil {
						b.Fatal(err)
					}
					var calls int
					l, err := loader.New("names", func(_ context.Context, keys []loader.Key) ([]loader.Result, error) {
						calls++
						time.Sleep(latency)
						results := make([]loader.Result, len(keys))
						for i, key := range keys {
							results[i].Value = string(key)
						}
						return results, nil
					}, 32)
					if err != nil {
						b.Fatal(err)
					}
					user := &schema.Object{TypeName: "User", Fields: map[string]*schema.Field{}}
					resolve := func(_ context.Context, p schema.ResolveParams) (any, error) {
						calls++
						time.Sleep(latency)
						return string(p.Source.(loader.Key)), nil
					}
					options := []ExecutorOption{WithDocumentCache(1)}
					if mode == "batched" {
						resolve = func(ctx context.Context, p schema.ResolveParams) (any, error) {
							return loader.Load(ctx, l, p.Source.(loader.Key))
						}
						options = append(options, WithLoaders([]*loader.Loader{l}))
					}
					user.Fields["name"] = &schema.Field{Name: "name", Type: s.Types["String"], Resolver: resolve}
					keys := make([]loader.Key, 32)
					for i := range keys {
						keys[i] = loader.Key(strconv.Itoa(i))
					}
					s.Types[user.Name()] = user
					s.Query.Fields["users"] = &schema.Field{Name: "users", Type: &schema.List{OfType: user}, Resolver: func(context.Context, schema.ResolveParams) (any, error) { return keys, nil }}
					e := New(s, nil, options...)
					req := core.Request{Query: `{ users { name } }`}
					if response := e.Execute(context.Background(), req); len(response.Errors) != 0 {
						b.Fatal(response.Errors)
					}
					calls = 0
					b.ReportAllocs()
					b.ResetTimer()
					for b.Loop() {
						if response := e.Execute(context.Background(), req); len(response.Errors) != 0 {
							b.Fatal(response.Errors)
						}
					}
					b.ReportMetric(float64(calls)/float64(b.N), "calls/op")
				})
			}
		})
	}
}
