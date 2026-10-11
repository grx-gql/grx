package exec

import (
	"context"
	"testing"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/schema"
)

func BenchmarkSelectionExecution(b *testing.B) {
	compiled, err := schema.Build(schema.Config{Query: orderedListQuery{}})
	if err != nil {
		b.Fatal(err)
	}
	for _, scenario := range []struct {
		name  string
		query string
	}{
		{name: "typename", query: `{ __typename }`},
		{name: "root_fragment", query: `{ ...Root } fragment Root on Query { __typename }`},
		{name: "list_fields", query: `{ items { id name } }`},
		{name: "list_fragment", query: `{ items { ...Row } } fragment Row on orderedListRow { id name }`},
	} {
		b.Run(scenario.name, func(b *testing.B) {
			executor := New(compiled, nil, WithDocumentCache(1))
			request := core.Request{Query: scenario.query}
			ctx := context.Background()
			if response := executor.Execute(ctx, request); len(response.Errors) != 0 {
				b.Fatal(response.Errors)
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
