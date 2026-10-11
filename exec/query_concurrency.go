package exec

import (
	"context"
	"fmt"
	"runtime/debug"
	"sync"
	"sync/atomic"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/schema"
)

type parallelFieldResult struct {
	key    string
	value  any
	errors []core.Error
	wire   bool
}

// executeParallelQuery gives each worker independent completion scratch. Only
// completed results, tracing, and optional resolver memoization cross workers.
func (e *Executor) executeParallelQuery(ctx context.Context, root *schema.Object, doc document) (*core.OrderedObject, []core.Error) {
	arena := lookupPathArena(ctx)
	flat, _, flatErrors := e.prepareSelections(arena, root, doc.Selections, doc.Fragments)
	if len(flat) <= 1 {
		return e.executeSelectionSet(ctx, root, nil, doc.Selections, doc.Fragments, nil)
	}
	results := make([]parallelFieldResult, len(flat))
	var next atomic.Int64
	var workers sync.WaitGroup
	var cache *parallelResolverCache
	if e.resolverCacheEnabled {
		cache = &parallelResolverCache{entries: make(map[string]*parallelResolverEntry)}
	}
	run := func() {
		workerCtx := withPathArena(ctx)
		defer recyclePathArena(workerCtx)
		workerArena := lookupPathArena(workerCtx)
		workerArena.apollo = arena.apollo
		workerArena.parallelCache = cache
		for index := int(next.Add(1) - 1); index < len(flat); index = int(next.Add(1) - 1) {
			results[index] = e.resolveParallelField(workerCtx, root, flat[index], doc.Fragments, workerArena)
		}
	}
	for index := 1; index < min(e.queryConcurrency, len(flat)); index++ {
		workers.Add(1)
		go func() { defer workers.Done(); run() }()
	}
	run()
	workers.Wait()
	data := core.NewOrderedObject(len(flat))
	errors := append([]core.Error(nil), flatErrors...)
	nullRoot := false
	for index, result := range results {
		errors = append(errors, result.errors...)
		if len(result.errors) > 0 && result.value == nil {
			if field := root.Fields[flat[index].Name]; field != nil && !isNullableGraphQLType(field.Type) {
				nullRoot = true
			}
		}
		if result.wire {
			data.Set(result.key, result.value)
		}
	}
	if nullRoot {
		return nil, errors
	}
	return finalizeSelectionObject(data, errors)
}

func (e *Executor) resolveParallelField(ctx context.Context, root *schema.Object, selected selection, fragments map[string]*fragmentDef, arena *pathArena) (result parallelFieldResult) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err := fmt.Errorf("panic during GraphQL field execution: %v", recovered)
			e.notifyError(ctx, err)
			e.notifyError(ctx, fmt.Errorf("panic stack: %s", debug.Stack()))
			key := selected.responseKey()
			result = parallelFieldResult{key: key, errors: []core.Error{newFieldError(e.maskError(err, true).Error(), []any{key}, selected.Location)}, wire: true}
		}
	}()
	result.key, result.value, result.errors, result.wire = e.resolveSelectedField(ctx, root, nil, selected, fragments, nil, arena)
	return result
}

type parallelResolverEntry struct {
	ready chan struct{}
	value any
	err   error
}

type parallelResolverCache struct {
	mu      sync.Mutex
	entries map[string]*parallelResolverEntry
}

// resolve publishes one raw resolver result before child completion begins.
// Waiting never holds the mutex, so nested field completion cannot deadlock.
func (cache *parallelResolverCache) resolve(ctx context.Context, key string, resolve func() (any, error)) (any, error) {
	cache.mu.Lock()
	if entry, ok := cache.entries[key]; ok {
		cache.mu.Unlock()
		select {
		case <-entry.ready:
			return entry.value, entry.err
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	entry := &parallelResolverEntry{ready: make(chan struct{})}
	cache.entries[key] = entry
	cache.mu.Unlock()
	entry.value, entry.err = resolve()
	close(entry.ready)
	return entry.value, entry.err
}
