package exec

import (
	"context"

	"github.com/grx-gql/grx/plugins"
	"github.com/grx-gql/grx/schema"
)

func (e *Executor) resolveSubscriptionSource(ctx context.Context, root *schema.Object, field *schema.Field, selected selection) (any, error) {
	var path []string
	if e.fieldAuthorizer != nil || len(e.Plugins) > 0 {
		path = []string{selected.responseKey()}
	}
	if e.fieldAuthorizer != nil {
		if err := e.fieldAuthorizer(ctx, FieldAuthorizationContext{
			ParentType: root.Name(), FieldName: selected.Name, Path: path,
		}); err != nil {
			e.notifyError(ctx, err)
			return nil, e.maskError(err, false)
		}
	}
	var fieldCtx plugins.FieldContext
	if len(e.Plugins) > 0 {
		fieldCtx = plugins.FieldContext{
			ParentType: root.Name(), FieldName: selected.Name,
			Path: path, ReturnType: field.Type.Name(),
		}
	}
	for _, hook := range e.Plugins {
		if err := hook.FieldResolveStart(ctx, fieldCtx); err != nil {
			e.notifyError(ctx, err)
			return nil, e.maskError(err, true)
		}
	}
	args, err := coerceArguments(field.Args, selected.Arguments)
	if err != nil {
		e.notifyError(ctx, err)
		return nil, e.maskError(err, false)
	}
	source, err := e.invokeResolver(ctx, nil, field, nil, args)
	for _, ender := range e.fieldEnders {
		ender.FieldResolveEnd(ctx, fieldCtx, err)
	}
	if err != nil {
		e.notifyError(ctx, err)
		return nil, e.maskError(err, true)
	}
	return source, nil
}
