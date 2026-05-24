package exec

import (
	"context"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/plugins"
)

// executeIntrospection resolves an introspection query through the normal
// selection-execution path (honoring the selection set and invoking field
// hooks) rather than the built-in fast path. It reuses the fast-path data
// builders to produce the underlying values, then projects the requested
// selection over them so plugins and field authorizers observe each field.
func (e *Executor) executeIntrospection(ctx context.Context, req core.Request) (*core.OrderedObject, []core.Error) {
	bundle, err := e.parseBundle(req)
	if err != nil {
		e.notifyError(ctx, err)
		return nil, []core.Error{core.NewRequestError(err)}
	}
	doc, err := selectOperation(bundle, req.OperationName)
	if err != nil {
		e.notifyError(ctx, err)
		return nil, []core.Error{core.NewRequestError(err)}
	}

	root := introspectionData(e.Schema, req)
	projected, errs := e.projectIntrospection(ctx, root, doc.Selections, doc.Fragments, nil, "Query")
	obj, _ := projected.(*core.OrderedObject)
	return obj, errs
}

// projectIntrospection projects a selection set over a pre-built introspection
// value, recursing into nested objects and lists. Field hooks run for every
// selected field so introspection participates in normal execution semantics.
func (e *Executor) projectIntrospection(ctx context.Context, value any, selections []selection, fragments map[string]*fragmentDef, path []any, typeName string) (any, []core.Error) {
	switch node := value.(type) {
	case *core.OrderedObject:
		flat := flattenIntrospectionSelections(selections, fragments)
		out := core.NewOrderedObject(len(flat))
		var errs []core.Error
		for _, sel := range flat {
			key := sel.responseKey()
			fieldPath := append(append([]any{}, path...), key)

			if hookErr := e.introspectionFieldHooks(ctx, typeName, sel.Name, fieldPath); hookErr != nil {
				errs = append(errs, *hookErr)
				continue
			}

			if sel.Name == "__typename" {
				out.Set(key, typeName)
				continue
			}
			child, ok := orderedLookup(node, sel.Name)
			if !ok {
				out.Set(key, nil)
				continue
			}
			projected, childErrs := e.projectIntrospection(ctx, child, sel.Selections, fragments, fieldPath, "")
			errs = append(errs, childErrs...)
			out.Set(key, projected)
		}
		return out, errs
	case []any:
		out := make([]any, len(node))
		var errs []core.Error
		for i, item := range node {
			itemPath := append(append([]any{}, path...), i)
			projected, itemErrs := e.projectIntrospection(ctx, item, selections, fragments, itemPath, typeName)
			out[i] = projected
			errs = append(errs, itemErrs...)
		}
		return out, errs
	default:
		// Leaf value (scalar/enum/null) — selections do not apply.
		return value, nil
	}
}

// introspectionFieldHooks runs the field authorizer and plugins FieldResolveStart
// hooks for a projected introspection field. A non-nil return is a field error
// that should replace the field.
func (e *Executor) introspectionFieldHooks(ctx context.Context, parentType, fieldName string, path []any) *core.Error {
	if e.fieldAuthorizer == nil && len(e.Plugins) == 0 {
		return nil
	}
	pathParts := pathStrings(path)
	if e.fieldAuthorizer != nil {
		if err := e.fieldAuthorizer(ctx, FieldAuthorizationContext{
			ParentType: parentType,
			FieldName:  fieldName,
			Path:       pathParts,
		}); err != nil {
			e.notifyError(ctx, err)
			fe := newFieldError(e.maskError(err, false).Error(), path, core.Location{})
			return &fe
		}
	}
	for _, hook := range e.Plugins {
		if err := hook.FieldResolveStart(ctx, plugins.FieldContext{Path: pathParts, FieldName: fieldName}); err != nil {
			e.notifyError(ctx, err)
			fe := newFieldError(e.maskError(err, false).Error(), path, core.Location{})
			return &fe
		}
	}
	return nil
}

func orderedLookup(o *core.OrderedObject, name string) (any, bool) {
	for _, f := range o.Fields() {
		if f.Name == name {
			return f.Value, true
		}
	}
	return nil, false
}

// flattenIntrospectionSelections inlines fragment spreads and inline fragments
// and applies @skip/@include. Type conditions are not re-checked because the
// underlying introspection value already has the correct shape and the document
// passed validation.
func flattenIntrospectionSelections(selections []selection, fragments map[string]*fragmentDef) []selection {
	var out []selection
	for _, sel := range selections {
		if skip, include, err := evalSkipInclude(sel.Directives); err == nil && (skip || !include) {
			continue
		}
		switch {
		case sel.isFragmentSpread():
			if fd := fragments[sel.FragmentSpread]; fd != nil {
				out = appendMergedSelections(out, flattenIntrospectionSelections(fd.Selections, fragments))
			}
		case sel.isInlineFragment():
			out = appendMergedSelections(out, flattenIntrospectionSelections(sel.Selections, fragments))
		case sel.isField():
			out = appendMergedSelection(out, sel)
		}
	}
	return out
}
