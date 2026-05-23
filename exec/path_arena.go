package exec

import (
	"context"
	"sync"
	"unsafe"
)

// pathArenaCtxKey carries a request-scratch []any reused for GraphQL paths.
// Paths passed to helpers are valid only until callers mutate or nest again;
// persistent copies are created by [core.NewFieldError].
type pathArenaCtxKey struct{}

type pathArena struct {
	buf []any
	// collector is non-nil only while an incremental-delivery request is in
	// flight; it gathers deferred/streamed work without per-field ctx lookups.
	collector *incrementalCollector
}

var pathArenaPool = sync.Pool{
	New: func() any { return &pathArena{} },
}

func withPathArena(ctx context.Context) context.Context {
	a := pathArenaPool.Get().(*pathArena)
	a.buf = a.buf[:0]
	a.collector = nil
	return context.WithValue(ctx, pathArenaCtxKey{}, a)
}

func recyclePathArena(ctx context.Context) {
	a, ok := ctx.Value(pathArenaCtxKey{}).(*pathArena)
	if !ok {
		return
	}
	pathArenaPool.Put(a)
}

func lookupPathArena(ctx context.Context) *pathArena {
	a, _ := ctx.Value(pathArenaCtxKey{}).(*pathArena)
	return a
}

// extendAppendedPath builds path+seg in the arena buffer when present and the
// path is rooted in or copied into that arena. Otherwise allocations fall back
// to appendPath.
func extendAppendedPath(a *pathArena, path []any, seg any) []any {
	if a == nil {
		return appendPath(path, seg)
	}

	need := len(path) + 1

	if need > cap(a.buf) {
		grow := need * 2
		if grow < 16 {
			grow = 16
		}
		next := make([]any, need, grow)
		if len(path) > 0 {
			copy(next, path)
		}
		next[len(path)] = seg
		a.buf = next
		return a.buf[:need]
	}

	a.buf = a.buf[:need]

	if len(path) == 0 {
		a.buf[0] = seg
		return a.buf[:need]
	}

	// Serial execution passes path prefixes that alias this buffer; extending
	// without copying overlaps is safe via in-place truncation + append.
	if len(path) <= cap(a.buf) && unsafe.SliceData(path) == unsafe.SliceData(a.buf) {
		a.buf[len(path)] = seg
		return a.buf[:need]
	}

	copy(a.buf, path)
	a.buf[len(path)] = seg
	return a.buf[:need]
}
