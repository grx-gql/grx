package exec

import (
	"context"
	"testing"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/schema"
)

func BenchmarkIncrementalDelivery(b *testing.B) {
	s, err := schema.Build(schema.Config{Query: incrementalQuery{}})
	if err != nil {
		b.Fatal(err)
	}
	e := New(s, nil, WithDocumentCache(16))
	req := core.Request{Query: `{friends @stream(initialCount:0){id}}`}
	b.Run("buffered", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			e.ExecuteIncremental(context.Background(), req)
		}
	})
	b.Run("streamed", func(b *testing.B) {
		b.ReportAllocs()
		for b.Loop() {
			if err := e.ExecuteIncrementalStream(context.Background(), req, func(core.Response) error { return nil }, func(core.IncrementalPayload, bool) error { return nil }); err != nil {
				b.Fatal(err)
			}
		}
	})
}
