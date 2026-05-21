// Package exec contains the GraphQL lexer, parser, AST, executor, and
// introspection fast-path. It is the hot per-request execution path and
// is deliberately transport-agnostic: callers communicate with the
// executor through [core.Request] and [core.Response].
package exec

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"runtime/debug"
	"strings"

	"github.com/patrickkabwe/grx/core"
	"github.com/patrickkabwe/grx/plugin"
	"github.com/patrickkabwe/grx/schema"
)

// ErrSubscriptionOperation is returned by [Executor.Execute] when the
// document defines a subscription operation. Subscriptions must be run
// through [Executor.Subscribe] so callers receive a streaming channel.
var ErrSubscriptionOperation = errors.New("subscription operations must use the streaming Subscribe entry point")

// Executor runs GraphQL operations against a built [schema.Schema] and
// notifies the registered plugins at each lifecycle phase. It satisfies
// [core.Executor] and is safe for concurrent use.
type Executor struct {
	Schema               *schema.Schema
	Plugins              []plugin.Plugin
	disableIntrospection bool
	maxSelectionDepth    int
	maskInternalErrors   bool
	clientErrorMessage   string
	operationAuthorizer  OperationAuthorizer
	fieldAuthorizer      FieldAuthorizer
	rateLimiter          RateLimiter
	trustedDocuments     map[string]string
	rejectUnknownVars    bool
}

// New returns an [Executor] bound to schemaValue and plugins. plugins
// may be nil; an Executor with no plugins simply skips the lifecycle
// notifications.
func New(schemaValue *schema.Schema, plugins []plugin.Plugin, opts ...ExecutorOption) *Executor {
	e := &Executor{Schema: schemaValue, Plugins: plugins}
	for _, opt := range opts {
		opt(e)
	}
	return e
}

// Execute runs a query or mutation operation and returns the completed
// response. Subscription operations are rejected with
// [ErrSubscriptionOperation]; use [Executor.Subscribe] instead. Errors
// produced during plugin notifications, parsing, or field resolution are
// surfaced via the returned response.
func (e *Executor) Execute(ctx context.Context, req core.Request) (response core.Response) {
	defer func() {
		if rec := recover(); rec != nil {
			err := fmt.Errorf("panic during GraphQL execution: %v", rec)
			e.notifyError(ctx, err)
			e.notifyError(ctx, fmt.Errorf("panic stack: %s", string(debug.Stack())))
			response = e.failResponse(ctx, e.maskError(err, true))
		}
	}()

	ctx, err := e.startRequest(ctx, req)
	if err != nil {
		return e.failResponse(ctx, e.maskError(err, false))
	}
	if err := e.notifyParsing(ctx, req); err != nil {
		return e.failResponse(ctx, e.maskError(err, false))
	}

	if isIntrospectionQuery(req.Query) {
		if e.disableIntrospection {
			err := fmt.Errorf("introspection is disabled")
			e.notifyError(ctx, err)
			return e.failResponse(ctx, err)
		}
		return e.sendResponse(ctx, core.Response{Data: introspectionData(e.Schema, req)})
	}

	doc, err := parseDocumentNamed(req.Query, req.Variables, req.OperationName, e.maxSelectionDepth)
	if err != nil {
		e.notifyError(ctx, err)
		return e.failResponse(ctx, err)
	}
	if err := e.validateDocumentSecurity(ctx, req, doc); err != nil {
		e.notifyError(ctx, err)
		return e.failResponse(ctx, e.maskError(err, false))
	}

	if doc.Kind == operationSubscription {
		e.notifyError(ctx, ErrSubscriptionOperation)
		return e.failResponse(ctx, ErrSubscriptionOperation)
	}

	if err := e.notifyValidation(ctx, req); err != nil {
		return e.failResponse(ctx, err)
	}
	if err := e.notifyExecution(ctx, req); err != nil {
		return e.failResponse(ctx, err)
	}

	root, err := e.rootObject(doc.Kind)
	if err != nil {
		e.notifyError(ctx, err)
		return e.failResponse(ctx, e.maskError(err, false))
	}

	data, fieldErrors := e.executeSelectionSet(ctx, root, nil, doc.Selections, doc.Fragments, nil)
	res := core.Response{Data: data, Errors: fieldErrors}
	if len(fieldErrors) == 0 {
		res.Errors = nil
	}
	return e.sendResponse(ctx, res)
}

// OperationKind parses req and reports the kind of the selected operation.
// It performs no plugin notifications and runs no resolvers, so transports
// can call it cheaply to decide whether to dispatch a request to Execute
// or Subscribe.
func (e *Executor) OperationKind(req core.Request) (core.OperationKind, error) {
	doc, err := parseDocumentNamed(req.Query, req.Variables, req.OperationName, e.maxSelectionDepth)
	if err != nil {
		return "", err
	}
	switch doc.Kind {
	case operationQuery:
		return core.OperationQuery, nil
	case operationMutation:
		return core.OperationMutation, nil
	case operationSubscription:
		return core.OperationSubscription, nil
	default:
		return "", fmt.Errorf("unknown operation kind %q", doc.Kind)
	}
}

// Subscribe parses a subscription operation, invokes the source-stream resolver,
// and returns a channel of GraphQL responses that close when the source stream
// closes or the supplied context is cancelled.
func (e *Executor) Subscribe(ctx context.Context, req core.Request) (responses <-chan core.Response, err error) {
	defer func() {
		if rec := recover(); rec != nil {
			raw := fmt.Errorf("panic during GraphQL subscription: %v", rec)
			e.notifyError(ctx, raw)
			e.notifyError(ctx, fmt.Errorf("panic stack: %s", string(debug.Stack())))
			err = e.maskError(raw, true)
		}
	}()

	ctx, err = e.startRequest(ctx, req)
	if err != nil {
		return nil, e.maskError(err, false)
	}
	if err := e.notifyParsing(ctx, req); err != nil {
		return nil, e.maskError(err, false)
	}

	doc, err := parseDocumentNamed(req.Query, req.Variables, req.OperationName, e.maxSelectionDepth)
	if err != nil {
		e.notifyError(ctx, err)
		return nil, err
	}
	if err := e.validateDocumentSecurity(ctx, req, doc); err != nil {
		e.notifyError(ctx, err)
		return nil, e.maskError(err, false)
	}
	if doc.Kind != operationSubscription {
		err := fmt.Errorf("Subscribe requires a subscription operation, got %s", doc.Kind)
		e.notifyError(ctx, err)
		return nil, e.maskError(err, false)
	}
	if err := e.notifyValidation(ctx, req); err != nil {
		return nil, err
	}

	root, err := e.rootObject(doc.Kind)
	if err != nil {
		return nil, err
	}

	flat, flatErrs := e.flattenSelections(root, doc.Selections, doc.Fragments)
	if len(flatErrs) > 0 {
		return nil, errors.New(flatErrs[0].Message)
	}
	if len(flat) != 1 || !flat[0].isField() {
		err := errors.New("subscription operations must select exactly one root field")
		e.notifyError(ctx, err)
		return nil, err
	}

	rootField := flat[0]
	field, ok := root.Fields[rootField.Name]
	if !ok {
		err := fmt.Errorf("unknown subscription field %q", rootField.Name)
		e.notifyError(ctx, err)
		return nil, err
	}

	if err := e.notifyExecution(ctx, req); err != nil {
		return nil, err
	}

	source, err := field.Resolver(ctx, schema.ResolveParams{Args: rootField.Arguments})
	if err != nil {
		e.notifyError(ctx, err)
		return nil, e.maskError(err, true)
	}
	sourceValue := reflect.ValueOf(source)
	if sourceValue.Kind() != reflect.Chan {
		err := fmt.Errorf("subscription resolver %q must return a channel, got %T", rootField.Name, source)
		e.notifyError(ctx, err)
		return nil, err
	}

	outKey := rootField.responseKey()

	out := make(chan core.Response)
	go func() {
		defer close(out)
		ctxDone := reflect.ValueOf(ctx.Done())
		cases := []reflect.SelectCase{
			{Dir: reflect.SelectRecv, Chan: ctxDone},
			{Dir: reflect.SelectRecv, Chan: sourceValue},
		}
		for {
			chosen, value, ok := reflect.Select(cases)
			if chosen == 0 {
				return
			}
			if !ok {
				return
			}

			data, fieldErrors := e.completeValue(ctx, field.Type, value.Interface(), rootField.Selections, doc.Fragments, []any{outKey})
			payload := core.NewOrderedObject(1)
			payload.Set(outKey, data)
			res := core.Response{Data: payload}
			if len(fieldErrors) > 0 {
				res.Errors = fieldErrors
			}
			res = e.sendResponse(ctx, res)

			select {
			case <-ctx.Done():
				return
			case out <- res:
			}
		}
	}()

	return out, nil
}

func (e *Executor) sendResponse(ctx context.Context, res core.Response) core.Response {
	res = core.AttachRequestIDExtension(res, core.RequestIDFromContext(ctx))
	for _, hook := range e.Plugins {
		if err := hook.ResponseSend(ctx, res); err != nil {
			e.notifyError(ctx, err)
			return e.failResponse(ctx, e.maskError(err, false))
		}
	}
	return res
}

func (e *Executor) rootObject(kind operationKind) (*schema.Object, error) {
	switch kind {
	case operationQuery:
		if e.Schema.Query == nil {
			return nil, fmt.Errorf("schema has no query root")
		}
		return e.Schema.Query, nil
	case operationMutation:
		if e.Schema.Mutation == nil {
			return nil, fmt.Errorf("schema has no mutation root")
		}
		return e.Schema.Mutation, nil
	case operationSubscription:
		if e.Schema.Subscription == nil {
			return nil, fmt.Errorf("schema has no subscription root")
		}
		return e.Schema.Subscription, nil
	default:
		return nil, fmt.Errorf("unsupported operation kind %q", kind)
	}
}

func (e *Executor) executeSelectionSet(ctx context.Context, object *schema.Object, source any, selections []selection, fragments map[string]*fragmentDef, path []any) (*core.OrderedObject, []core.Error) {
	flat, flatErrs := e.flattenSelections(object, selections, fragments)
	errors := append([]core.Error{}, flatErrs...)

	data := core.NewOrderedObject(len(flat))
	for _, selected := range flat {
		if err := ctx.Err(); err != nil {
			errors = append(errors, newFieldError(err.Error(), path, core.Location{}))
			return data, errors
		}

		key := selected.responseKey()
		if selected.Name == "__typename" {
			data.Set(key, object.Name())
			continue
		}

		skip, include, derr := evalSkipInclude(selected.Directives)
		if derr != nil {
			errors = append(errors, newFieldError(derr.Error(), appendPath(path, key), selected.Location))
			continue
		}
		if skip || !include {
			continue
		}

		field, ok := object.Fields[selected.Name]
		if !ok {
			errors = append(errors, newFieldError(fmt.Sprintf("unknown field %q on %s", selected.Name, object.Name()), appendPath(path, key), selected.Location))
			continue
		}

		fieldPath := appendPath(path, key)
		if e.fieldAuthorizer != nil {
			err := e.fieldAuthorizer(ctx, FieldAuthorizationContext{
				ParentType: object.Name(),
				FieldName:  selected.Name,
				Path:       pathStrings(fieldPath),
			})
			if err != nil {
				e.notifyError(ctx, err)
				errors = append(errors, newFieldError(e.maskError(err, false).Error(), fieldPath, selected.Location))
				continue
			}
		}

		blocked := false
		for _, hook := range e.Plugins {
			if err := hook.FieldResolveStart(ctx, plugin.FieldContext{Path: pathStrings(fieldPath), FieldName: selected.Name}); err != nil {
				e.notifyError(ctx, err)
				errors = append(errors, newFieldError(e.maskError(err, false).Error(), fieldPath, selected.Location))
				blocked = true
				break
			}
		}
		if blocked {
			continue
		}

		value, err := field.Resolver(ctx, schema.ResolveParams{Source: source, Args: selected.Arguments})
		if err != nil {
			e.notifyError(ctx, err)
			errors = append(errors, newFieldError(e.maskError(err, true).Error(), fieldPath, selected.Location))
			continue
		}

		resolved, nestedErrors := e.completeValue(ctx, field.Type, value, selected.Selections, fragments, fieldPath)
		data.Set(key, resolved)
		errors = append(errors, nestedErrors...)
	}

	return data, errors
}

func (e *Executor) completeValue(ctx context.Context, fieldType schema.Type, value any, selections []selection, fragments map[string]*fragmentDef, path []any) (any, []core.Error) {
	if err := ctx.Err(); err != nil {
		return nil, []core.Error{newFieldError(err.Error(), path, core.Location{})}
	}
	if value == nil {
		if fieldType.Kind() == schema.NonNullKind {
			return nil, []core.Error{newFieldError("non-null field resolved to null", path, core.Location{})}
		}
		return nil, nil
	}

	switch typed := fieldType.(type) {
	case *schema.NonNull:
		inner, errs := e.completeValue(ctx, typed.OfType, value, selections, fragments, path)
		if len(errs) > 0 || inner == nil {
			if len(errs) == 0 {
				errs = []core.Error{newFieldError("non-null field resolved to null", path, core.Location{})}
			}
			return nil, errs
		}
		return inner, nil
	case *schema.List:
		return e.completeList(ctx, typed.OfType, value, selections, fragments, path)
	case *schema.Object:
		return e.executeSelectionSet(ctx, typed, value, selections, fragments, path)
	case *schema.Interface:
		objectType, err := typed.Resolve(value)
		if err != nil {
			e.notifyError(ctx, err)
			return nil, []core.Error{newFieldError(e.maskError(err, true).Error(), path, core.Location{})}
		}
		return e.executeSelectionSet(ctx, objectType, value, selections, fragments, path)
	case *schema.Union:
		objectType, err := typed.Resolve(value)
		if err != nil {
			e.notifyError(ctx, err)
			return nil, []core.Error{newFieldError(e.maskError(err, true).Error(), path, core.Location{})}
		}
		return e.executeSelectionSet(ctx, objectType, value, selections, fragments, path)
	case *schema.Enum:
		serialized, err := typed.Serialize(value)
		if err != nil {
			e.notifyError(ctx, err)
			return nil, []core.Error{newFieldError(e.maskError(err, true).Error(), path, core.Location{})}
		}
		return serialized, nil
	case *schema.Scalar:
		serialized, err := typed.Serialize(value)
		if err != nil {
			e.notifyError(ctx, err)
			return nil, []core.Error{newFieldError(e.maskError(err, true).Error(), path, core.Location{})}
		}
		return serialized, nil
	default:
		return value, nil
	}
}

func (e *Executor) completeList(ctx context.Context, itemType schema.Type, value any, selections []selection, fragments map[string]*fragmentDef, path []any) ([]any, []core.Error) {
	raw := reflect.ValueOf(value)
	if raw.Kind() == reflect.Pointer {
		raw = raw.Elem()
	}
	if raw.Kind() != reflect.Slice && raw.Kind() != reflect.Array {
		err := fmt.Errorf("expected list value, got %T", value)
		e.notifyError(ctx, err)
		return nil, []core.Error{newFieldError(e.maskError(err, true).Error(), path, core.Location{})}
	}

	items := make([]any, 0, raw.Len())
	errors := []core.Error{}
	for index := 0; index < raw.Len(); index++ {
		if err := ctx.Err(); err != nil {
			return items, append(errors, newFieldError(err.Error(), path, core.Location{}))
		}
		itemPath := appendPath(path, index)
		item, itemErrors := e.completeValue(ctx, itemType, raw.Index(index).Interface(), selections, fragments, itemPath)
		items = append(items, item)
		errors = append(errors, itemErrors...)
	}

	return items, errors
}

func (e *Executor) startRequest(ctx context.Context, req core.Request) (context.Context, error) {
	var err error
	for _, hook := range e.Plugins {
		ctx, err = hook.RequestStart(ctx, req)
		if err != nil {
			e.notifyError(ctx, err)
			return ctx, err
		}
	}
	return ctx, nil
}

func (e *Executor) notifyParsing(ctx context.Context, req core.Request) error {
	for _, hook := range e.Plugins {
		if err := hook.ParsingStart(ctx, req); err != nil {
			e.notifyError(ctx, err)
			return err
		}
	}
	return nil
}

func (e *Executor) notifyValidation(ctx context.Context, req core.Request) error {
	for _, hook := range e.Plugins {
		if err := hook.ValidationStart(ctx, req); err != nil {
			e.notifyError(ctx, err)
			return err
		}
	}
	return nil
}

func (e *Executor) notifyExecution(ctx context.Context, req core.Request) error {
	for _, hook := range e.Plugins {
		if err := hook.ExecutionStart(ctx, req); err != nil {
			e.notifyError(ctx, err)
			return err
		}
	}
	return nil
}

func (e *Executor) notifyError(ctx context.Context, err error) {
	for _, hook := range e.Plugins {
		hook.Error(ctx, err)
	}
}

func (e *Executor) validateDocumentSecurity(ctx context.Context, req core.Request, doc document) error {
	if e.rejectUnknownVars {
		if err := rejectUnknownVariables(req, doc); err != nil {
			return err
		}
	}
	if len(e.trustedDocuments) > 0 {
		if err := e.validateTrustedDocument(req.Query); err != nil {
			return err
		}
	}

	operationCtx := OperationContext{
		Request: req,
		Kind:    toCoreOperationKind(doc.Kind),
		Name:    doc.Name,
	}
	if e.rateLimiter != nil {
		if err := e.rateLimiter(ctx, operationCtx); err != nil {
			return err
		}
	}
	if e.operationAuthorizer != nil {
		if err := e.operationAuthorizer(ctx, operationCtx); err != nil {
			return err
		}
	}
	return nil
}

func (e *Executor) validateTrustedDocument(query string) error {
	sum := sha256.Sum256([]byte(query))
	hash := hex.EncodeToString(sum[:])
	trustedQuery, ok := e.trustedDocuments[hash]
	if !ok {
		return fmt.Errorf("operation is not trusted")
	}
	if trustedQuery != "" && trustedQuery != query {
		return fmt.Errorf("trusted document hash %q does not match request query", hash)
	}
	return nil
}

func rejectUnknownVariables(req core.Request, doc document) error {
	if len(req.Variables) == 0 {
		return nil
	}
	declared := make(map[string]struct{}, len(doc.Variables))
	for _, variable := range doc.Variables {
		declared[variable] = struct{}{}
	}
	for variable := range req.Variables {
		if _, ok := declared[variable]; !ok {
			return fmt.Errorf("unknown variable %q", variable)
		}
	}
	return nil
}

func toCoreOperationKind(kind operationKind) core.OperationKind {
	switch kind {
	case operationQuery:
		return core.OperationQuery
	case operationMutation:
		return core.OperationMutation
	case operationSubscription:
		return core.OperationSubscription
	default:
		return core.OperationKind(kind)
	}
}

func (e *Executor) maskError(err error, internal bool) error {
	if !internal || !e.maskInternalErrors {
		return err
	}
	message := strings.TrimSpace(e.clientErrorMessage)
	if message == "" {
		message = "internal server error"
	}
	return errors.New(message)
}

func appendPath(path []any, item any) []any {
	next := make([]any, 0, len(path)+1)
	next = append(next, path...)
	next = append(next, item)
	return next
}

func pathStrings(path []any) []string {
	result := make([]string, 0, len(path))
	for _, item := range path {
		result = append(result, fmt.Sprint(item))
	}
	return result
}

func (e *Executor) failResponse(ctx context.Context, err error) core.Response {
	return core.AttachRequestIDExtension(
		core.Response{Errors: []core.Error{core.NewRequestError(err)}},
		core.RequestIDFromContext(ctx),
	)
}

func newFieldError(message string, path []any, location core.Location) core.Error {
	return core.NewFieldError(message, path, location)
}
