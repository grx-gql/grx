package exec

import (
	"github.com/grx-gql/grx/schema"
	"math"
	"testing"
)

func TestResolverCachePreservesFloatBits(t *testing.T) {
	field := &schema.Field{Name: "custom"}
	for _, values := range [][]any{{math.Float64frombits(0x7ff8000000000001), math.Float64frombits(0x7ff8000000000002)}, {math.Float32frombits(0x7fc00001), math.Float32frombits(0x7fc00002)}, {float64(0), math.Copysign(0, -1)}} {
		first, ok := resolverCacheKey(field, nil, map[string]any{"v": values[0]})
		if !ok {
			t.Fatal("float key unavailable")
		}
		second, ok := resolverCacheKey(field, nil, map[string]any{"v": values[1]})
		if !ok || first == second {
			t.Fatalf("distinct float representations share cache: %x %x", first, second)
		}
	}
}
