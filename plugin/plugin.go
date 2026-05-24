// Package plugin defines the lifecycle hook interface that observers and
// middleware implement to participate in GraphQL request processing. A
// plugin is consulted at each phase of a request (parsing, validation,
// execution, response, errors). Embed [Base] when implementing only a
// subset of the hooks.
package plugin

import (
	"context"

	"github.com/grx-gql/grx/core"
)

// RequestContext carries per-request data passed to plugin hooks that
// operate at the request granularity.
type RequestContext struct {
	Request core.Request
}

// FieldContext describes a single field resolution inside a selection
// set. Path is the response path (using GraphQL response-path semantics)
// and FieldName is the GraphQL field name being resolved. ParentType and
// ReturnType carry the GraphQL type names, enabling tracing exporters
// (OpenTelemetry, Apollo tracing) to label spans precisely.
type FieldContext struct {
	Path       []string
	FieldName  string
	ParentType string
	ReturnType string
}

// FieldResolveEnder is an optional companion to [Plugin]. Plugins that
// implement it receive a callback after each field finishes resolving (with the
// resolver error, if any), enabling field-level tracing and metrics without a
// breaking change to the core Plugin interface. The executor invokes it only
// for plugins that implement it.
type FieldResolveEnder interface {
	FieldResolveEnd(ctx context.Context, field FieldContext, err error)
}

// Plugin is the lifecycle interface an observer or middleware
// implements. Hooks are invoked in registration order; returning a
// non-nil error from any hook (other than RequestStart) aborts the
// current request and surfaces the error to the client. RequestStart may
// additionally return a derived context that becomes the parent context
// for every subsequent hook of the same request.
//
// Implementations should be safe for concurrent use because a single
// Plugin instance is shared across all in-flight requests.
type Plugin interface {
	RequestStart(ctx context.Context, req core.Request) (context.Context, error)
	ParsingStart(ctx context.Context, req core.Request) error
	ValidationStart(ctx context.Context, req core.Request) error
	ExecutionStart(ctx context.Context, req core.Request) error
	FieldResolveStart(ctx context.Context, field FieldContext) error
	ResponseSend(ctx context.Context, res core.Response) error
	Error(ctx context.Context, err error)
}

// Base is a no-op implementation of [Plugin]. Embed it in a custom
// plugin to get default implementations of every hook and only override
// the ones you care about.
type Base struct{}

// RequestStart returns ctx unchanged.
func (Base) RequestStart(ctx context.Context, req core.Request) (context.Context, error) {
	return ctx, nil
}

// ParsingStart is a no-op.
func (Base) ParsingStart(ctx context.Context, req core.Request) error {
	return nil
}

// ValidationStart is a no-op.
func (Base) ValidationStart(ctx context.Context, req core.Request) error {
	return nil
}

// ExecutionStart is a no-op.
func (Base) ExecutionStart(ctx context.Context, req core.Request) error {
	return nil
}

// FieldResolveStart is a no-op.
func (Base) FieldResolveStart(ctx context.Context, field FieldContext) error {
	return nil
}

// ResponseSend is a no-op.
func (Base) ResponseSend(ctx context.Context, res core.Response) error {
	return nil
}

// Error is a no-op.
func (Base) Error(ctx context.Context, err error) {}
