package exec

import (
	"context"
	"fmt"
	"reflect"
	"strconv"
	"sync"
	"sync/atomic"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/loader"
	"github.com/grx-gql/grx/schema"
)

// Prepared values let batching reuse ordinary completion, including null
// propagation, scalar serialization, field hooks, and incremental collection.
type batchExecution struct {
	fields   map[string]*resolvedField
	order    []string
	abstract map[string]batchAbstract
	objects  []batchObject
	pending  []batchPending
}

type batchAbstract struct {
	object *schema.Object
	err    error
}
type batchObject struct {
	object     *schema.Object
	source     any
	selections []selection
	fragments  map[string]*fragmentDef
	path       []any
}
type batchPending struct {
	result    *resolvedField
	selected  *selection
	fragments map[string]*fragmentDef
}

func appendBatchPath(buf []byte, path []any) []byte {
	for _, part := range path {
		switch value := part.(type) {
		case string:
			buf = strconv.AppendQuote(buf, value)
		case int:
			buf = strconv.AppendInt(buf, int64(value), 10)
			buf = append(buf, '/')
		default:
			panic(fmt.Sprintf("unexpected GraphQL response path segment %T", part))
		}
	}
	return buf
}

func batchPathKey(path []any) string {
	var scratch [128]byte
	return string(appendBatchPath(scratch[:0], path))
}

func batchFieldKey(path []any, key string) string {
	var scratch [128]byte
	return string(strconv.AppendQuote(appendBatchPath(scratch[:0], path), key))
}

func newBatchExecution() *batchExecution {
	return &batchExecution{fields: make(map[string]*resolvedField), abstract: make(map[string]batchAbstract)}
}

func (e *Executor) executeWithLoaders(ctx context.Context, root *schema.Object, doc document) (*core.OrderedObject, []core.Error) {
	if doc.Kind == operationMutation {
		return e.executeBatchedMutation(ctx, root, doc)
	}
	scoped, err := loader.NewScope(ctx, e.loaders)
	if err != nil {
		return nil, []core.Error{newFieldError(err.Error(), nil, core.Location{})}
	}
	if e.resolverCacheEnabled && e.queryConcurrency > 1 {
		lookupPathArena(ctx).parallelCache = &parallelResolverCache{entries: make(map[string]*parallelResolverEntry)}
	}
	return e.executeBatchSelection(scoped, root, nil, doc.Selections, doc.Fragments, nil)
}

func (e *Executor) executeBatchedMutation(ctx context.Context, root *schema.Object, doc document) (*core.OrderedObject, []core.Error) {
	flat, _, errs := e.prepareSelections(lookupPathArena(ctx), root, doc.Selections, doc.Fragments)
	data := core.NewOrderedObject(len(flat))
	for _, selected := range flat {
		scoped, err := loader.NewScope(ctx, e.loaders)
		if err != nil {
			return nil, append(errs, newFieldError(err.Error(), nil, selected.Location))
		}
		value, fieldErrors := e.executeBatchSelection(scoped, root, nil, []selection{selected}, doc.Fragments, nil)
		errs = append(errs, fieldErrors...)
		if value == nil {
			return nil, errs
		}
		for _, field := range value.Fields() {
			data.Set(field.Name, field.Value)
		}
	}
	return finalizeSelectionObject(data, errs)
}

func (e *Executor) executeBatchSelection(ctx context.Context, object *schema.Object, source any, selections []selection, fragments map[string]*fragmentDef, path []any) (*core.OrderedObject, []core.Error) {
	batch := newBatchExecution()
	batch.objects = append(batch.objects, batchObject{object: object, source: source, selections: selections, fragments: fragments, path: clonePath(path)})
	lookupPathArena(ctx).batch = batch
	defer e.finishBatch(ctx, batch)
	if err := e.prepareBatch(ctx, batch); err != nil {
		return nil, []core.Error{newFieldError(err.Error(), path, core.Location{})}
	}
	return e.executeSelectionSet(ctx, object, source, selections, fragments, path)
}

func (e *Executor) completeBatchValue(ctx context.Context, fieldType schema.Type, value any, selections []selection, fragments map[string]*fragmentDef, path []any) (any, []core.Error) {
	batch := newBatchExecution()
	lookupPathArena(ctx).batch = batch
	defer e.finishBatch(ctx, batch)
	e.collectBatchValue(ctx, batch, fieldType, value, selections, fragments, clonePath(path))
	if err := e.prepareBatch(ctx, batch); err != nil {
		return nil, []core.Error{newFieldError(err.Error(), path, core.Location{})}
	}
	return e.completeValue(ctx, fieldType, value, selections, fragments, path)
}

func (e *Executor) prepareBatch(ctx context.Context, batch *batchExecution) error {
	for len(batch.objects) > 0 || len(batch.pending) > 0 {
		for len(batch.objects) > 0 {
			objects := batch.objects
			batch.objects = nil
			for _, object := range objects {
				arena := lookupPathArena(ctx)
				flat, _, _ := e.prepareSelections(arena, object.object, object.selections, object.fragments)
				results := e.prepareBatchFields(ctx, object, flat)
				for i := range results {
					result := &results[i]
					selected := flat[i]
					key := batchFieldKey(object.path, selected.responseKey())
					batch.fields[key] = result
					batch.order = append(batch.order, key)
					if result.field == nil || result.resolverError != nil {
						continue
					}
					if _, pending := result.value.(*loader.Pending); pending {
						batch.pending = append(batch.pending, batchPending{result: result, selected: &flat[i], fragments: object.fragments})
					} else {
						e.collectBatchField(ctx, batch, result, selected, object.fragments)
					}
				}
			}
		}
		if len(batch.pending) == 0 {
			continue
		}
		if err := loader.Dispatch(ctx); err != nil {
			return err
		}
		pending := batch.pending
		batch.pending = nil
		for _, work := range pending {
			result := work.result
			value, err := loader.Read(result.value.(*loader.Pending))
			if _, nested := value.(*loader.Pending); nested && err == nil {
				err = fmt.Errorf("loader result at %v must be a concrete value", result.path)
			}
			result.value, result.resolverError = value, err
			if err != nil {
				e.notifyError(ctx, err)
				result.value = nil
				result.errors = []core.Error{newFieldError(e.maskError(err, true).Error(), result.path, work.selected.Location)}
			}
			if err == nil {
				e.collectBatchField(ctx, batch, result, *work.selected, work.fragments)
			}
		}
	}
	return nil
}

func (e *Executor) prepareBatchFields(ctx context.Context, object batchObject, flat []selection) []resolvedField {
	results := make([]resolvedField, len(flat))
	if len(object.path) != 0 || len(flat) <= 1 || e.queryConcurrency <= 1 {
		for i, selected := range flat {
			results[i] = e.prepareBatchField(ctx, object, selected)
		}
		return results
	}
	arena := lookupPathArena(ctx)
	var next atomic.Int64
	var workers sync.WaitGroup
	run := func() {
		workerCtx := withPathArena(ctx)
		defer recyclePathArena(workerCtx)
		workerArena := lookupPathArena(workerCtx)
		workerArena.parallelCache, workerArena.apollo = arena.parallelCache, arena.apollo
		for i := int(next.Add(1) - 1); i < len(flat); i = int(next.Add(1) - 1) {
			results[i] = e.prepareBatchField(workerCtx, object, flat[i])
		}
	}
	for i := 1; i < min(e.queryConcurrency, len(flat)); i++ {
		workers.Go(run)
	}
	run()
	workers.Wait()
	return results
}

func (e *Executor) prepareBatchField(ctx context.Context, object batchObject, selected selection) (result resolvedField) {
	defer func() {
		if recovered := recover(); recovered != nil {
			err := fmt.Errorf("panic during GraphQL field resolution: %v", recovered)
			e.notifyError(ctx, err)
			result = resolvedField{key: selected.responseKey(), wire: true, errors: []core.Error{newFieldError(e.maskError(err, true).Error(), appendPath(object.path, selected.responseKey()), selected.Location)}}
		}
	}()
	result = e.prepareSelectedField(ctx, object.object, object.source, selected, object.path, lookupPathArena(ctx))
	result.path = clonePath(result.path)
	return result
}

func (e *Executor) collectBatchField(ctx context.Context, batch *batchExecution, result *resolvedField, selected selection, fragments map[string]*fragmentDef) {
	value := result.value
	if arenaCollector(lookupPathArena(ctx)) != nil {
		if active, count, _ := streamDirective(selected.Directives); active {
			if _, ok := listItemType(result.field.Type); ok && value != nil && !isNilPointer(value) {
				raw := reflect.ValueOf(value)
				if raw.Kind() == reflect.Pointer {
					raw = raw.Elem()
				}
				if raw.Kind() == reflect.Slice || raw.Kind() == reflect.Array {
					itemType, _ := listItemType(result.field.Type)
					for i := 0; i < min(count, raw.Len()); i++ {
						e.collectBatchValue(ctx, batch, itemType, raw.Index(i).Interface(), selected.Selections, fragments, appendPath(result.path, i))
					}
					return
				}
			}
		}
	}
	e.collectBatchValue(ctx, batch, result.field.Type, value, selected.Selections, fragments, result.path)
}

// collectBatchValue retains paths owned by the batch; callers must not pass
// the mutable path arena buffer. List elements get their own path in appendPath.
func (e *Executor) collectBatchValue(ctx context.Context, batch *batchExecution, fieldType schema.Type, value any, selections []selection, fragments map[string]*fragmentDef, path []any) {
	if value == nil || isNilPointer(value) || ctx.Err() != nil {
		return
	}
	switch typed := fieldType.(type) {
	case *schema.NonNull:
		e.collectBatchValue(ctx, batch, typed.OfType, value, selections, fragments, path)
	case *schema.List:
		raw := reflect.ValueOf(value)
		if raw.Kind() == reflect.Pointer {
			raw = raw.Elem()
		}
		if raw.Kind() != reflect.Slice && raw.Kind() != reflect.Array {
			return
		}
		for i := 0; i < raw.Len(); i++ {
			e.collectBatchValue(ctx, batch, typed.OfType, raw.Index(i).Interface(), selections, fragments, appendPath(path, i))
		}
	case *schema.Object:
		batch.objects = append(batch.objects, batchObject{object: typed, source: value, selections: selections, fragments: fragments, path: path})
	case *schema.Interface:
		e.collectBatchAbstract(ctx, batch, typed.Name(), typed.Resolve, value, selections, fragments, path)
	case *schema.Union:
		e.collectBatchAbstract(ctx, batch, typed.Name(), typed.Resolve, value, selections, fragments, path)
	}
}

func (e *Executor) collectBatchAbstract(ctx context.Context, batch *batchExecution, name string, resolve func(any) (*schema.Object, error), value any, selections []selection, fragments map[string]*fragmentDef, path []any) {
	object, err := e.resolveAbstractType(name, value, resolve)
	batch.abstract[batchPathKey(path)] = batchAbstract{object: object, err: err}
	if err == nil {
		e.collectBatchValue(ctx, batch, object, value, selections, fragments, path)
	}
}

func (e *Executor) resolveCompletionObject(ctx context.Context, name string, value any, resolve func(any) (*schema.Object, error), path []any) (*schema.Object, error) {
	if arena := lookupPathArena(ctx); arena != nil && arena.batch != nil {
		if resolved, ok := arena.batch.abstract[batchPathKey(path)]; ok {
			return resolved.object, resolved.err
		}
		return nil, fmt.Errorf("missing prepared abstract type %s at response path %v", name, path)
	}
	return e.resolveAbstractType(name, value, resolve)
}

func (e *Executor) finishBatch(ctx context.Context, batch *batchExecution) {
	for _, key := range batch.order {
		if result, ok := batch.fields[key]; ok {
			e.endResolvedField(ctx, lookupPathArena(ctx), result)
		}
	}
	lookupPathArena(ctx).batch = nil
}

func (e *Executor) completeLoaderEvent(ctx context.Context, fieldType schema.Type, value any, selections []selection, fragments map[string]*fragmentDef, path []any) (any, []core.Error) {
	scoped, err := loader.NewScope(ctx, e.loaders)
	if err != nil {
		return nil, []core.Error{newFieldError(err.Error(), path, core.Location{})}
	}
	scoped = withPathArena(scoped)
	defer recyclePathArena(scoped)
	return e.completeBatchValue(scoped, fieldType, value, selections, fragments, path)
}
