package exec

import (
	"strings"
	"testing"

	"github.com/grx-gql/grx/schema"
)

func TestResolverCacheNoArgumentKeyAllocations(t *testing.T) {
	field := &schema.Field{Name: "value"}
	allocations := testing.AllocsPerRun(100, func() {
		key, ok := resolverCacheKey(field, nil, nil)
		if !ok || key == "" {
			t.Fatal("cache key unavailable")
		}
	})
	if allocations > 1 {
		t.Fatalf("nil-argument root key allocated %.0f times; want at most 1 final key allocation", allocations)
	}
}

func TestResolverCacheKeyKeepsContainerAndIdentityDistinctions(t *testing.T) {
	field := &schema.Field{Name: "value"}
	emptyField := &schema.Field{Name: "value"}
	source := &struct{ ID int }{ID: 1}
	otherSource := &struct{ ID int }{ID: 1}
	var nilMap map[string]any
	var nilList []any
	cases := []struct {
		field  *schema.Field
		source any
		args   map[string]any
	}{
		{field, nil, nil},
		{field, nil, map[string]any{}},
		{emptyField, nil, nil},
		{field, source, nil},
		{field, otherSource, nil},
		{field, nil, map[string]any{"v": nilMap}},
		{field, nil, map[string]any{"v": map[string]any{}}},
		{field, nil, map[string]any{"v": nilList}},
		{field, nil, map[string]any{"v": []any{}}},
	}
	seen := make(map[string]bool, len(cases))
	for index, tc := range cases {
		key, ok := resolverCacheKey(tc.field, tc.source, tc.args)
		if !ok || seen[key] {
			t.Fatalf("case %d reused a distinct key or could not encode", index)
		}
		seen[key] = true
	}
}

func TestResolverCacheKeyRetainsLargeArgumentSuffixAndMapOrder(t *testing.T) {
	field := &schema.Field{Name: "value"}
	prefix := strings.Repeat("payload", 100)
	first, ok := resolverCacheKey(field, nil, map[string]any{"a": prefix + "first", "b": 2})
	if !ok {
		t.Fatal("large key unavailable")
	}
	same := map[string]any{"b": 2}
	same["a"] = prefix + "first"
	repeated, ok := resolverCacheKey(field, nil, same)
	if !ok || first != repeated {
		t.Fatal("equivalent arguments produced different keys")
	}
	different, ok := resolverCacheKey(field, nil, map[string]any{"a": prefix + "second", "b": 2})
	if !ok || first == different {
		t.Fatal("large argument suffix was lost")
	}
}
