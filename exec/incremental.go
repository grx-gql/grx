package exec

import (
	"context"
	"reflect"

	"github.com/patrickkabwe/grx/core"
	"github.com/patrickkabwe/grx/schema"
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
	return selectionsUseIncremental(doc.Selections, bundle.fragments, map[string]bool{})
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
	preparedCtx, root, doc, short := e.prepareExecution(ctx, req)
	ctx = preparedCtx
	if short != nil {
		return *short, nil
	}

	collector := &incrementalCollector{}
	ctx = withPathArena(ctx)
	defer recyclePathArena(ctx)
	if arena := lookupPathArena(ctx); arena != nil {
		arena.collector = collector
	}

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

	var payloads []core.IncrementalPayload
	for len(collector.work) > 0 {
		item := collector.work[0]
		collector.work = collector.work[1:]
		payloads = append(payloads, e.runIncrementalWork(ctx, item))
	}

	hasNext := len(payloads) > 0
	initial.HasNext = &hasNext
	initial = e.sendResponse(ctx, initial)
	return initial, payloads
}

func (e *Executor) runIncrementalWork(ctx context.Context, item incrementalWork) core.IncrementalPayload {
	payload := core.IncrementalPayload{Label: item.label, Path: item.path}
	if item.stream {
		value, errs := e.completeValue(ctx, item.itemType, item.itemVal, item.selections, item.fragments, item.path)
		payload.Items = []any{value}
		if len(errs) > 0 {
			payload.Errors = errs
		}
		return payload
	}
	data, errs := e.executeSelectionSet(ctx, item.object, item.source, item.selections, item.fragments, item.path)
	payload.Data = data
	if len(errs) > 0 {
		payload.Errors = errs
	}
	return payload
}
