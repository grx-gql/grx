// Package core defines the transport-agnostic types shared by the server,
// executor, and transport implementations. It deliberately has no upward
// imports so that schema, exec, server, plugin, and core sub-packages can
// all depend on it without creating an import cycle.
package core

import "context"

// Request is the executor-facing representation of a single GraphQL
// operation. Transports decode their wire format into this struct before
// invoking [Executor.Execute] or [Executor.Subscribe].
type Request struct {
	// Query is the raw GraphQL document. It may contain a single
	// operation or multiple operations disambiguated by OperationName.
	Query string
	// OperationName selects an operation when Query defines more than
	// one. It is the empty string when the document contains exactly one
	// operation.
	OperationName string
	// Variables holds the JSON-decoded values referenced by `$name`
	// variables in Query. A nil map is equivalent to an empty map.
	Variables map[string]any
}

// Response is the canonical GraphQL response envelope. Either Data,
// Errors, or both may be set. The omitempty JSON tags ensure that absent
// fields are not serialised, matching the GraphQL spec.
type Response struct {
	Data   any     `json:"data,omitempty"`
	Errors []Error `json:"errors,omitempty"`
}

// Error is a single GraphQL error entry. Path identifies the field that
// produced the error (using GraphQL response-path semantics) and Meta
// carries optional implementation-defined metadata.
type Error struct {
	Message string         `json:"message"`
	Path    []string       `json:"path,omitempty"`
	Meta    map[string]any `json:"meta,omitempty"`
}

// OperationKind enumerates the three GraphQL executable operation kinds.
// It is returned by [Executor.OperationKind] so transports can route a
// request to [Executor.Execute] or [Executor.Subscribe] without
// re-implementing GraphQL parsing.
type OperationKind string

// Operation kinds defined by the GraphQL October 2021 spec §2.3.
const (
	OperationQuery        OperationKind = "query"
	OperationMutation     OperationKind = "mutation"
	OperationSubscription OperationKind = "subscription"
)

// Executor is the contract every transport uses to run GraphQL operations.
// Implementations must be safe for concurrent use by multiple goroutines.
type Executor interface {
	// Execute runs a Query or Mutation operation and returns the
	// completed response. It must not be used for Subscription
	// operations; use Subscribe instead.
	Execute(ctx context.Context, req Request) Response

	// Subscribe runs a Subscription operation and returns a channel of
	// responses. The channel is closed when the source stream ends or
	// ctx is cancelled. An error is returned only when the subscription
	// could not be started; per-event errors are surfaced inside the
	// emitted Response values.
	Subscribe(ctx context.Context, req Request) (<-chan Response, error)

	// OperationKind reports whether the operation selected by req (using
	// req.OperationName when the document defines several) is a query,
	// mutation, or subscription. It returns an error when the document
	// fails to parse or the named operation does not exist. Transports
	// rely on it to dispatch a single GraphQL-over-WS "subscribe" frame
	// to either Execute or Subscribe based on the actual operation kind.
	OperationKind(req Request) (OperationKind, error)
}
