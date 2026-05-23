// Package exec contains the GraphQL lexer, parser, AST, executor, and
// introspection fast-path. It is the hot per-request execution path and
// is deliberately transport-agnostic: callers communicate with the
// executor through [core.Request] and [core.Response].
//
// Resolver runs are ordered: sibling fields execute sequentially within each
// selection set (deterministic semantics for side-effecting code).
package exec

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"reflect"
	"runtime/debug"
	"strconv"
	"strings"
	"sync"

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
	maxSelectionCount    int
	maxAliasCount        int
	maxRootFieldCount    int
	maskInternalErrors   bool
	clientErrorMessage   string
	operationAuthorizer  OperationAuthorizer
	fieldAuthorizer      FieldAuthorizer
	rateLimiter          RateLimiter
	trustedDocuments     map[string]string
	rejectUnknownVars    bool
	documentCache        map[string]documentBundle
	documentCacheOrder   []string
	documentCacheLimit   int
	documentCacheMu      sync.RWMutex

	lexTokenCache map[string][]token
	lexCacheOrder []string
	lexCacheLimit int
	lexMu         sync.RWMutex
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

	preparedCtx, root, doc, short := e.prepareExecution(ctx, req)
	ctx = preparedCtx
	if short != nil {
		return *short
	}

	ctx = withPathArena(ctx)
	defer recyclePathArena(ctx)

	data, fieldErrors := e.executeSelectionSet(ctx, root, nil, doc.Selections, doc.Fragments, nil)
	res := core.Response{Errors: fieldErrors}
	if data == nil && len(fieldErrors) > 0 {
		res.DataNull = true
	} else {
		res.Data = data
	}
	if len(fieldErrors) == 0 {
		res.Errors = nil
	}
	return e.sendResponse(ctx, res)
}

// prepareExecution runs the shared query/mutation preamble: request start,
// parsing, introspection short-circuit, parsing/validation/security checks, and
// root-object resolution. It returns the (possibly updated) context plus the
// root object and selected operation. When a non-nil *core.Response is
// returned, the caller must return it directly — it represents an introspection
// result or a request-level failure that ends the operation early.
func (e *Executor) prepareExecution(ctx context.Context, req core.Request) (context.Context, *schema.Object, document, *core.Response) {
	ctx, err := e.startRequest(ctx, req)
	if err != nil {
		return e.shortResponse(ctx, e.maskError(err, false))
	}
	if err := e.notifyParsing(ctx, req); err != nil {
		return e.shortResponse(ctx, e.maskError(err, false))
	}

	if isIntrospectionQuery(req.Query) {
		if e.disableIntrospection {
			err := fmt.Errorf("introspection is disabled")
			e.notifyError(ctx, err)
			return e.shortResponse(ctx, err)
		}
		res := e.sendResponse(ctx, core.Response{Data: introspectionData(e.Schema, req)})
		return ctx, nil, document{}, &res
	}

	bundle, err := e.parseBundle(req)
	if err != nil {
		e.notifyError(ctx, err)
		return e.shortResponse(ctx, err)
	}
	doc, err := selectOperation(bundle, req.OperationName)
	if err != nil {
		e.notifyError(ctx, err)
		return e.shortResponse(ctx, err)
	}
	if verrs := ValidateDocument(e.Schema, bundle, doc); len(verrs) > 0 {
		res := e.validationFailResponse(ctx, verrs)
		return ctx, nil, document{}, &res
	}
	if err := e.validateDocumentSecurity(ctx, req, doc); err != nil {
		e.notifyError(ctx, err)
		return e.shortResponse(ctx, e.maskError(err, false))
	}

	if doc.Kind == operationSubscription {
		e.notifyError(ctx, ErrSubscriptionOperation)
		return e.shortResponse(ctx, ErrSubscriptionOperation)
	}

	if err := e.notifyValidation(ctx, req); err != nil {
		return e.shortResponse(ctx, err)
	}
	if err := e.notifyExecution(ctx, req); err != nil {
		return e.shortResponse(ctx, err)
	}

	root, err := e.rootObject(doc.Kind)
	if err != nil {
		e.notifyError(ctx, err)
		return e.shortResponse(ctx, e.maskError(err, false))
	}
	return ctx, root, doc, nil
}

func (e *Executor) shortResponse(ctx context.Context, err error) (context.Context, *schema.Object, document, *core.Response) {
	res := e.failResponse(ctx, err)
	return ctx, nil, document{}, &res
}

// OperationKind parses req and reports the kind of the selected operation.
// It performs no plugin notifications and runs no resolvers, so transports
// can call it cheaply to decide whether to dispatch a request to Execute
// or Subscribe. With [WithLexerCache], tokenisation is LRU-cached per
// normalised query so a transport that checks OperationKind before Execute can
// share one lexical pass across both steps.
func (e *Executor) OperationKind(req core.Request) (core.OperationKind, error) {
	source := normalizeSource(req.Query)
	tokens, err := e.sharedLexNormalized(source)
	if err != nil {
		return "", err
	}
	return operationKindFromTokens(source, tokens, req.OperationName)
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

	bundle, err := e.parseBundle(req)
	if err != nil {
		e.notifyError(ctx, err)
		return nil, err
	}
	doc, err := selectOperation(bundle, req.OperationName)
	if err != nil {
		e.notifyError(ctx, err)
		return nil, err
	}
	if verrs := ValidateDocument(e.Schema, bundle, doc); len(verrs) > 0 {
		return nil, verrs[0]
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

	flat, _, flatErrs := e.flattenSelections(false, root, doc.Selections, doc.Fragments)
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

func (e *Executor) parseBundle(req core.Request) (documentBundle, error) {
	if e.documentCacheLimit <= 0 || len(req.Variables) > 0 {
		return parseDocumentBundleWithLexer(e, req.Query, req.Variables, e.maxSelectionDepth)
	}
	normQuery := normalizeSource(req.Query)
	key := fmt.Sprintf("%d:%s", e.maxSelectionDepth, normQuery)
	e.documentCacheMu.RLock()
	if cached, ok := e.documentCache[key]; ok {
		e.documentCacheMu.RUnlock()
		return cached, nil
	}
	e.documentCacheMu.RUnlock()

	bundle, err := parseDocumentBundleWithLexer(e, req.Query, nil, e.maxSelectionDepth)
	if err != nil {
		return documentBundle{}, err
	}

	e.documentCacheMu.Lock()
	defer e.documentCacheMu.Unlock()
	if e.documentCache == nil {
		e.documentCache = make(map[string]documentBundle, e.documentCacheLimit)
	}
	if _, exists := e.documentCache[key]; !exists {
		if len(e.documentCacheOrder) >= e.documentCacheLimit {
			oldest := e.documentCacheOrder[0]
			e.documentCacheOrder = e.documentCacheOrder[1:]
			delete(e.documentCache, oldest)
		}
		e.documentCache[key] = bundle
		e.documentCacheOrder = append(e.documentCacheOrder, key)
	}
	return bundle, nil
}

func (e *Executor) sharedLexNormalized(source string) ([]token, error) {
	if e.lexCacheLimit <= 0 {
		return lexNormalizedSource(source)
	}

	e.lexMu.RLock()
	if tokens, ok := e.lexTokenCache[source]; ok {
		e.lexMu.RUnlock()
		return tokens, nil
	}
	e.lexMu.RUnlock()

	tokens, err := lexNormalizedSource(source)
	if err != nil {
		return nil, err
	}

	e.lexMu.Lock()
	defer e.lexMu.Unlock()

	if cached, dup := e.lexTokenCache[source]; dup {
		return cached, nil
	}
	if e.lexTokenCache == nil {
		e.lexTokenCache = make(map[string][]token, e.lexCacheLimit)
	}
	if len(e.lexCacheOrder) >= e.lexCacheLimit {
		evictKey := e.lexCacheOrder[0]
		e.lexCacheOrder = e.lexCacheOrder[1:]
		delete(e.lexTokenCache, evictKey)
	}
	e.lexTokenCache[source] = tokens
	e.lexCacheOrder = append(e.lexCacheOrder, source)
	return tokens, nil
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

func finalizeSelectionObject(data *core.OrderedObject, errors []core.Error) (*core.OrderedObject, []core.Error) {
	if len(errors) > 0 && len(data.Fields()) == 0 {
		return nil, errors
	}
	return data, errors
}

func isNullableGraphQLType(t schema.Type) bool {
	_, isNN := t.(*schema.NonNull)
	return !isNN
}

func (e *Executor) executeSelectionSet(ctx context.Context, object *schema.Object, source any, selections []selection, fragments map[string]*fragmentDef, path []any) (*core.OrderedObject, []core.Error) {
	arena := lookupPathArena(ctx)
	collector := (*incrementalCollector)(nil)
	if arena != nil {
		collector = arena.collector
	}

	flat, defers, flatErrs := e.flattenSelections(collector != nil, object, selections, fragments)
	var errors []core.Error
	if len(flatErrs) > 0 {
		errors = append(errors, flatErrs...)
	}

	if collector != nil {
		for _, d := range defers {
			collector.addDefer(object, source, d.selections, fragments, clonePath(path), d.label)
		}
	}

	data := core.NewOrderedObject(len(flat))
	for _, selected := range flat {
		key, value, fieldErrors, wire := e.resolveSelectedField(ctx, object, source, selected, fragments, path, arena)
		if len(fieldErrors) > 0 {
			errors = append(errors, fieldErrors...)
		}
		if !wire {
			continue
		}
		data.Set(key, value)
	}
	return finalizeSelectionObject(data, errors)
}

func (e *Executor) resolveSelectedField(ctx context.Context, object *schema.Object, source any, selected selection, fragments map[string]*fragmentDef, path []any, arena *pathArena) (string, any, []core.Error, bool) {
	if err := ctx.Err(); err != nil {
		return "", nil, []core.Error{newFieldError(err.Error(), path, core.Location{})}, false
	}

	key := selected.responseKey()
	if selected.Name == "__typename" {
		return key, object.Name(), nil, true
	}

	field, ok := object.Fields[selected.Name]
	if !ok {
		return key, nil, []core.Error{newFieldError(
			fmt.Sprintf(`Cannot query field "%s" on type "%s".`, selected.Name, object.Name()),
			extendAppendedPath(arena, path, key), selected.Location)}, false
	}

	fieldPath := extendAppendedPath(arena, path, key)

	needPathStrings := e.fieldAuthorizer != nil || len(e.Plugins) > 0
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
			return key, nil, []core.Error{newFieldError(e.maskError(err, false).Error(), fieldPath, selected.Location)}, false
		}
	}

	for _, hook := range e.Plugins {
		if err := hook.FieldResolveStart(ctx, plugin.FieldContext{Path: pathParts, FieldName: selected.Name}); err != nil {
			e.notifyError(ctx, err)
			return key, nil, []core.Error{newFieldError(e.maskError(err, false).Error(), fieldPath, selected.Location)}, false
		}
	}

	args, err := coerceArguments(field.Args, selected.Arguments)
	if err != nil {
		e.notifyError(ctx, err)
		return key, nil, []core.Error{newFieldError(e.maskError(err, false).Error(), fieldPath, selected.Location)}, false
	}

	value, resolverErr := field.Resolver(ctx, schema.ResolveParams{Source: source, Args: args})
	var resolverMsgs []core.Error
	if resolverErr != nil {
		e.notifyError(ctx, resolverErr)
		resolverMsgs = []core.Error{newFieldError(e.maskError(resolverErr, true).Error(), fieldPath, selected.Location)}
		value = nil
	}

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
	if !streamed {
		resolved, nestedErrors = e.completeValue(ctx, field.Type, value, selected.Selections, fragments, fieldPath)
	}
	errs := append(resolverMsgs, nestedErrors...)
	var wire bool
	switch {
	case resolverErr != nil:
		wire = false // omit nullable key so root can serialize as full data:null
	case len(errs) == 0:
		wire = true
	case resolved == nil && !isNullableGraphQLType(field.Type):
		wire = false
	default:
		wire = true
	}
	return key, resolved, errs, wire
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
		coerced, err := coerceBuiltInScalar(typed.TypeName, serialized)
		if err != nil {
			e.notifyError(ctx, err)
			return nil, []core.Error{newFieldError(e.maskError(err, true).Error(), path, core.Location{})}
		}
		return coerced, nil
	default:
		coerced, err := coerceBuiltInScalarOutput(value)
		if err != nil {
			e.notifyError(ctx, err)
			return nil, []core.Error{newFieldError(e.maskError(err, true).Error(), path, core.Location{})}
		}
		return coerced, nil
	}
}

func (e *Executor) completeList(ctx context.Context, itemType schema.Type, value any, selections []selection, fragments map[string]*fragmentDef, path []any) (any, []core.Error) {
	arena := lookupPathArena(ctx)

	raw := reflect.ValueOf(value)
	if raw.Kind() == reflect.Pointer {
		raw = raw.Elem()
	}
	if raw.Kind() != reflect.Slice && raw.Kind() != reflect.Array {
		err := fmt.Errorf("expected list value, got %T", value)
		e.notifyError(ctx, err)
		return nil, []core.Error{newFieldError(e.maskError(err, true).Error(), path, core.Location{})}
	}

	n := raw.Len()

	if gqlObj, ok := schemaObjectLeaf(itemType); ok {
		got := raw.Type().Elem()
		want := gqlObj.ReflectType()
		if want != nil && got.Kind() == reflect.Pointer && got.AssignableTo(reflect.PointerTo(want)) {
			objs := make([]*core.OrderedObject, n)
			var errors []core.Error
			for index := 0; index < n; index++ {
				if err := ctx.Err(); err != nil {
					return objs[:index], append(errors, newFieldError(err.Error(), path, core.Location{}))
				}
				itemPath := extendAppendedPath(arena, path, index)
				item, itemErrors := e.executeSelectionSet(ctx, gqlObj, raw.Index(index).Interface(), selections, fragments, itemPath)
				objs[index] = item
				if len(itemErrors) > 0 {
					errors = append(errors, itemErrors...)
				}
			}
			return objs, errors
		}
	}

	items := make([]any, n)
	var errors []core.Error
	for index := 0; index < n; index++ {
		if err := ctx.Err(); err != nil {
			return items[:index], append(errors, newFieldError(err.Error(), path, core.Location{}))
		}
		itemPath := extendAppendedPath(arena, path, index)
		item, itemErrors := e.completeValue(ctx, itemType, raw.Index(index).Interface(), selections, fragments, itemPath)
		items[index] = item
		if len(itemErrors) > 0 {
			errors = append(errors, itemErrors...)
		}
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
	if err := e.validateDocumentLimits(doc); err != nil {
		return err
	}
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

func (e *Executor) validateDocumentLimits(doc document) error {
	if e.maxSelectionCount <= 0 && e.maxAliasCount <= 0 && e.maxRootFieldCount <= 0 {
		return nil
	}
	stats := documentLimitStats{}
	collectLimitStats(doc.Selections, &stats)
	if e.maxRootFieldCount > 0 && stats.rootFields > e.maxRootFieldCount {
		return fmt.Errorf("root field count %d exceeds limit of %d", stats.rootFields, e.maxRootFieldCount)
	}
	if e.maxSelectionCount > 0 && stats.selections > e.maxSelectionCount {
		return fmt.Errorf("selection count %d exceeds limit of %d", stats.selections, e.maxSelectionCount)
	}
	if e.maxAliasCount > 0 && stats.aliases > e.maxAliasCount {
		return fmt.Errorf("alias count %d exceeds limit of %d", stats.aliases, e.maxAliasCount)
	}
	return nil
}

type documentLimitStats struct {
	selections int
	aliases    int
	rootFields int
}

func collectLimitStats(selections []selection, stats *documentLimitStats) {
	for _, selected := range selections {
		stats.selections++
		if selected.isField() {
			stats.rootFields++
			if selected.Alias != "" {
				stats.aliases++
			}
		}
		collectNestedLimitStats(selected.Selections, stats)
	}
}

func collectNestedLimitStats(selections []selection, stats *documentLimitStats) {
	for _, selected := range selections {
		stats.selections++
		if selected.isField() && selected.Alias != "" {
			stats.aliases++
		}
		collectNestedLimitStats(selected.Selections, stats)
	}
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

func schemaObjectLeaf(t schema.Type) (*schema.Object, bool) {
	for {
		switch x := t.(type) {
		case *schema.NonNull:
			t = x.OfType
		case *schema.Object:
			return x, true
		default:
			return nil, false
		}
	}
}

func appendPath(path []any, item any) []any {
	next := make([]any, len(path)+1)
	copy(next, path)
	next[len(path)] = item
	return next
}

func pathStrings(path []any) []string {
	if len(path) == 0 {
		return nil
	}
	result := make([]string, len(path))
	for index, item := range path {
		result[index] = pathSegmentString(item)
	}
	return result
}

func pathSegmentString(segment any) string {
	switch typed := segment.(type) {
	case string:
		return typed
	case int:
		return strconv.Itoa(typed)
	case int8:
		return strconv.Itoa(int(typed))
	case int16:
		return strconv.Itoa(int(typed))
	case int32:
		return strconv.FormatInt(int64(typed), 10)
	case int64:
		return strconv.FormatInt(typed, 10)
	case uint:
		return strconv.FormatUint(uint64(typed), 10)
	case uint8:
		return strconv.FormatUint(uint64(typed), 10)
	case uint16:
		return strconv.FormatUint(uint64(typed), 10)
	case uint32:
		return strconv.FormatUint(uint64(typed), 10)
	case uint64:
		return strconv.FormatUint(typed, 10)
	default:
		return fmt.Sprint(segment)
	}
}

func (e *Executor) failResponse(ctx context.Context, err error) core.Response {
	return core.AttachRequestIDExtension(
		core.Response{Errors: []core.Error{core.NewRequestError(err)}},
		core.RequestIDFromContext(ctx),
	)
}

func (e *Executor) validationFailResponse(ctx context.Context, errs []validationError) core.Response {
	return core.AttachRequestIDExtension(validationResponse(errs), core.RequestIDFromContext(ctx))
}

func newFieldError(message string, path []any, location core.Location) core.Error {
	return core.NewFieldError(message, path, location)
}
