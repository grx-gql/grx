package exec

import (
	"context"
	"fmt"
	"time"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/plugins"
	"github.com/grx-gql/grx/schema"
)

type resolvedField struct {
	key           string
	field         *schema.Field
	value         any
	errors        []core.Error
	wire          bool
	path          []any
	fieldContext  plugins.FieldContext
	start         time.Time
	resolverError error
}

func (e *Executor) prepareSelectedField(ctx context.Context, object *schema.Object, source any, selected selection, path []any, arena *pathArena) resolvedField {
	if err := ctx.Err(); err != nil {
		return resolvedField{errors: []core.Error{newFieldError(err.Error(), path, core.Location{})}}
	}

	key := selected.responseKey()
	if selected.Name == "__typename" {
		return resolvedField{key: key, value: object.Name(), wire: true}
	}

	field, ok := object.Fields[selected.Name]
	if !ok {
		return resolvedField{key: key, errors: []core.Error{newFieldError(
			fmt.Sprintf(`Cannot query field "%s" on type "%s".`, selected.Name, object.Name()),
			extendAppendedPath(arena, path, key), selected.Location)}}
	}

	fieldPath := extendAppendedPath(arena, path, key)

	traceFields := len(e.fieldEnders) > 0 || e.apolloTracing
	needFieldContext := len(e.Plugins) > 0 || traceFields
	needPathStrings := e.fieldAuthorizer != nil || needFieldContext
	var pathParts []string
	if needPathStrings {
		pathParts = pathStrings(fieldPath)
	}

	if e.fieldAuthorizer != nil {
		err := e.fieldAuthorizer(ctx, FieldAuthorizationContext{
			ParentType: object.Name(),
			FieldName:  selected.Name,
			Path:       pathParts,
		})
		if err != nil {
			e.notifyError(ctx, err)
			return resolvedField{key: key, errors: []core.Error{newFieldError(e.maskError(err, false).Error(), fieldPath, selected.Location)}, wire: true}
		}
	}

	// fieldCtx (notably field.Type.Name(), which allocates for wrapped types)
	// is built only when an observer needs it, keeping the hot path allocation-free.
	var fieldCtx plugins.FieldContext
	if needFieldContext {
		fieldCtx = plugins.FieldContext{
			Path:       pathParts,
			FieldName:  selected.Name,
			ParentType: object.Name(),
			ReturnType: field.Type.Name(),
		}
	}
	for _, hook := range e.Plugins {
		if err := hook.FieldResolveStart(ctx, fieldCtx); err != nil {
			e.notifyError(ctx, err)
			return resolvedField{key: key, errors: []core.Error{newFieldError(e.maskError(err, true).Error(), fieldPath, selected.Location)}, wire: true}
		}
	}

	var fieldStart time.Time
	if traceFields {
		fieldStart = time.Now()
	}

	args, err := coerceArguments(field.Args, selected.Arguments)
	if err != nil {
		e.notifyError(ctx, err)
		return resolvedField{key: key, errors: []core.Error{newFieldError(e.maskError(err, false).Error(), fieldPath, selected.Location)}, wire: true}
	}

	value, resolverErr := e.invokeResolver(ctx, arena, field, source, args)
	var resolverMsgs []core.Error
	if resolverErr != nil {
		e.notifyError(ctx, resolverErr)
		resolverMsgs = []core.Error{newFieldError(e.maskError(resolverErr, true).Error(), fieldPath, selected.Location)}
		value = nil
	}

	return resolvedField{key: key, field: field, value: value, errors: resolverMsgs, wire: true, path: fieldPath, fieldContext: fieldCtx, start: fieldStart, resolverError: resolverErr}
}

func (e *Executor) completeResolvedField(ctx context.Context, selected selection, fragments map[string]*fragmentDef, arena *pathArena, result *resolvedField) (string, any, []core.Error, bool) {
	if result.field == nil {
		return result.key, result.value, result.errors, result.wire
	}
	field, value, fieldPath := result.field, result.value, result.path
	resolverErr, resolverMsgs := result.resolverError, result.errors
	var resolved any
	var nestedErrors []core.Error
	streamed := false
	if collector := arenaCollector(arena); collector != nil && resolverErr == nil && value != nil {
		if active, initialCount, label := streamDirective(selected.Directives); active {
			if itemType, ok := listItemType(field.Type); ok {
				resolved, nestedErrors = e.completeListStreamed(ctx, itemType, value, selected.Selections, fragments, fieldPath, initialCount, label, collector)
				streamed = true
			}
		}
	}
	if !streamed && resolverErr == nil {
		resolved, nestedErrors = e.completeValue(ctx, field.Type, value, selected.Selections, fragments, fieldPath)
	}
	errs := append(resolverMsgs, completionErrorLocations(nestedErrors, selected.Location)...)

	e.endResolvedField(ctx, arena, result)

	return result.key, resolved, errs, true
}

func (e *Executor) endResolvedField(ctx context.Context, arena *pathArena, result *resolvedField) {
	if result.field == nil {
		return
	}
	for _, ender := range e.fieldEnders {
		ender.FieldResolveEnd(ctx, result.fieldContext, result.resolverError)
	}
	if e.apolloTracing {
		recordApolloFieldTrace(arena, result.fieldContext, result.start, time.Now())
	}
}
