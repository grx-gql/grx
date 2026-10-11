package exec

import (
	"context"
	"reflect"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/schema"
)

// HasIncrementalDirectives reports whether the named operation in req uses
// @defer or @stream, so a transport can decide whether to negotiate an
// incremental-delivery (multipart/mixed) response before calling
// [Executor.ExecuteIncremental].
func (e *Executor) HasIncrementalDirectives(req core.Request) bool {
	bundle, err := e.parseBundle(req)
	if err != nil {
		return false
	}
	doc, err := selectOperation(bundle, req.OperationName)
	if err != nil {
		return false
	}
	return selectionsUseIncremental(doc.Selections, doc.Fragments, map[string]bool{})
}

func selectionsUseIncremental(selections []selection, fragments map[string]*fragmentDef, seen map[string]bool) bool {
	for _, s := range selections {
		if active, _ := deferDirective(s.Directives); active {
			return true
		}
		if active, _, _ := streamDirective(s.Directives); active {
			return true
		}
		if s.isFragmentSpread() {
			if seen[s.FragmentSpread] {
				continue
			}
			seen[s.FragmentSpread] = true
			if fd := fragments[s.FragmentSpread]; fd != nil {
				if selectionsUseIncremental(fd.Selections, fragments, seen) {
					return true
				}
			}
			continue
		}
		if selectionsUseIncremental(s.Selections, fragments, seen) {
			return true
		}
	}
	return false
}

// incrementalCollector accumulates deferred fragments and streamed list items
// discovered while executing the initial payload. Work is drained breadth-first
// so nested @defer/@stream inside a patch is itself delivered incrementally.
type incrementalCollector struct {
	work []incrementalWork
}

type incrementalWork struct {
	object     *schema.Object
	source     any
	selections []selection
	fragments  map[string]*fragmentDef
	path       []any
	label      string

	// stream item work
	stream   bool
	itemType schema.Type
	itemVal  any
}

func (c *incrementalCollector) addDefer(object *schema.Object, source any, selections []selection, fragments map[string]*fragmentDef, path []any, label string) {
	c.work = append(c.work, incrementalWork{
		object:     object,
		source:     source,
		selections: selections,
		fragments:  fragments,
		path:       path,
		label:      label,
	})
}

func (c *incrementalCollector) addStreamItem(itemType schema.Type, item any, selections []selection, fragments map[string]*fragmentDef, path []any, label string) {
	c.work = append(c.work, incrementalWork{
		selections: selections,
		fragments:  fragments,
		path:       path,
		label:      label,
		stream:     true,
		itemType:   itemType,
		itemVal:    item,
	})
}

func arenaCollector(a *pathArena) *incrementalCollector {
	if a == nil {
		return nil
	}
	return a.collector
}

func clonePath(path []any) []any {
	if len(path) == 0 {
		return nil
	}
	out := make([]any, len(path))
	copy(out, path)
	return out
}

// listItemType unwraps a non-null wrapper and reports the element type when t is
// a list, so @stream can complete items individually.
func listItemType(t schema.Type) (schema.Type, bool) {
	if nn, ok := t.(*schema.NonNull); ok {
		t = nn.OfType
	}
	if list, ok := t.(*schema.List); ok {
		return list.OfType, true
	}
	return nil, false
}

// completeListStreamed completes the first initialCount items of a @stream'd
// list inline and registers each remaining item with the collector for delivery
// as a subsequent incremental payload.
func (e *Executor) completeListStreamed(ctx context.Context, itemType schema.Type, value any, selections []selection, fragments map[string]*fragmentDef, path []any, initialCount int, label string, collector *incrementalCollector) (any, []core.Error) {
	arena := lookupPathArena(ctx)
	raw := reflect.ValueOf(value)
	if raw.Kind() == reflect.Pointer {
		raw = raw.Elem()
	}
	if raw.Kind() != reflect.Slice && raw.Kind() != reflect.Array {
		// Not a list after all; fall back to normal completion.
		return e.completeValue(ctx, &schema.List{OfType: itemType}, value, selections, fragments, path)
	}

	n := raw.Len()
	if initialCount > n {
		initialCount = n
	}

	items := make([]any, 0, initialCount)
	var errs []core.Error
	for index := 0; index < initialCount; index++ {
		itemPath := extendAppendedPath(arena, path, index)
		item, itemErrors := e.completeValue(ctx, itemType, raw.Index(index).Interface(), selections, fragments, itemPath)
		items = append(items, item)
		if len(itemErrors) > 0 {
			errs = append(errs, itemErrors...)
			if item == nil && !isNullableGraphQLType(itemType) {
				return nil, errs
			}
		}
	}

	for index := initialCount; index < n; index++ {
		collector.addStreamItem(itemType, raw.Index(index).Interface(), selections, fragments, clonePath(extendAppendedPath(arena, path, index)), label)
	}
	return items, errs
}

// ExecuteIncremental runs a query or mutation that uses @defer/@stream and
// returns the initial response plus the ordered incremental payloads. The
// initial response's HasNext is set to true when at least one payload follows.
// Callers that detect no incremental directives should prefer [Executor.Execute].
func (e *Executor) ExecuteIncremental(ctx context.Context, req core.Request) (core.Response, []core.IncrementalPayload) {
	var initial core.Response
	var payloads []core.IncrementalPayload
	err := e.ExecuteIncrementalStream(ctx, req, func(response core.Response) error {
		initial = response
		return nil
	}, func(payload core.IncrementalPayload, _ bool) error {
		payloads = append(payloads, payload)
		return nil
	})
	if err != nil {
		initial.Errors = append(initial.Errors, core.NewRequestError(err))
		hasNext := false
		initial.HasNext = &hasNext
	}
	return initial, payloads
}

// ExecuteIncrementalStream emits the initial response before completing pending
// @defer/@stream work. Callbacks run synchronously while execution state is alive;
// callback errors and context cancellation stop all remaining work.
func (e *Executor) ExecuteIncrementalStream(ctx context.Context, req core.Request, emitInitial func(core.Response) error, emitPatch func(core.IncrementalPayload, bool) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	preparedCtx, root, doc, short := e.prepareExecution(ctx, req)
	ctx = preparedCtx
	if short != nil {
		return emitInitial(*short)
	}

	collector := &incrementalCollector{}
	ctx = withPathArena(ctx)
	defer recyclePathArena(ctx)
	if arena := lookupPathArena(ctx); arena != nil {
		arena.collector = collector
	}
	e.enableResolverCache(ctx, doc.Kind)

	data, fieldErrors := e.executeSelectionSet(ctx, root, nil, doc.Selections, doc.Fragments, nil)
	initial := core.Response{Errors: fieldErrors}
	if data == nil && len(fieldErrors) > 0 {
		initial.DataNull = true
	} else {
		initial.Data = data
	}
	if len(fieldErrors) == 0 {
		initial.Errors = nil
	}

	hasNext := len(collector.work) > 0
	initial.HasNext = &hasNext
	initial = e.sendResponse(ctx, initial)
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := emitInitial(initial); err != nil {
		return err
	}
	// A response hook can replace the initial response with an error response.
	if initial.HasNext == nil || !*initial.HasNext {
		return ctx.Err()
	}
	for len(collector.work) > 0 {
		if err := ctx.Err(); err != nil {
			return err
		}
		item := collector.work[0]
		collector.work[0] = incrementalWork{}
		collector.work = collector.work[1:]
		payload := e.runIncrementalWork(ctx, item)
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := emitPatch(payload, len(collector.work) > 0); err != nil {
			return err
		}
	}
	return ctx.Err()
}

func (e *Executor) runIncrementalWork(ctx context.Context, item incrementalWork) core.IncrementalPayload {
	payload := core.IncrementalPayload{Label: item.label, Path: item.path}
	if item.stream {
		value, errs := e.completeValue(ctx, item.itemType, item.itemVal, item.selections, item.fragments, item.path)
		payload.Items = []any{value}
		if value == nil {
			nullPath := item.path
			if !isNullableGraphQLType(item.itemType) {
				payload.Items = nil
				payload.ItemsNull = true
				nullPath = item.path[:len(item.path)-1]
			}
			if collector := arenaCollector(lookupPathArena(ctx)); collector != nil {
				collector.work = incrementalWorkOutsidePath(collector.work, nullPath)
			}
		}
		if len(errs) > 0 {
			payload.Errors = errs
		}
		return payload
	}
	data, errs := e.executeSelectionSet(ctx, item.object, item.source, item.selections, item.fragments, item.path)
	payload.Data = data
	if data == nil {
		if collector := arenaCollector(lookupPathArena(ctx)); collector != nil {
			collector.work = incrementalWorkOutsidePath(collector.work, item.path)
		}
	}
	if len(errs) > 0 {
		payload.Errors = errs
	}
	return payload
}

// incrementalWorkOutsidePath removes pending work below a failed response
// position without discarding independent streams or deferred fragments.
func incrementalWorkOutsidePath(work []incrementalWork, path []any) []incrementalWork {
	remaining := make([]incrementalWork, 0, len(work))
	for _, item := range work {
		if !incrementalPathContains(path, item.path) {
			remaining = append(remaining, item)
		}
	}
	return remaining
}

func incrementalPathContains(parent, child []any) bool {
	if len(child) < len(parent) {
		return false
	}
	for index, part := range parent {
		if child[index] != part {
			return false
		}
	}
	return true
}
