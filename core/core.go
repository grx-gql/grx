// Package core defines the transport-agnostic types shared by the server,
// executor, and transport implementations. It deliberately has no upward
// imports so that schema, exec, server, plugin, and core sub-packages can
// all depend on it without creating an import cycle.
package core

import (
	"context"
)

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
// Errors, or both may be set. [Response.MarshalJSON] serialises absent data
// (request/validation failures) without a top-level key, partial or full null
// data with "data":null when [Response.DataNull] is true.
type Response struct {
	Data        any                  `json:"data,omitempty"`
	DataNull    bool                 `json:"-"`
	Errors      []Error              `json:"errors,omitempty"`
	Incremental []IncrementalPayload `json:"incremental,omitempty"`
	HasNext     *bool                `json:"hasNext,omitempty"`
	Extensions  map[string]any       `json:"extensions,omitempty"`
}

// Error is a single GraphQL error entry. Path identifies the field that
// produced the error (using GraphQL response-path semantics) and Meta
// carries optional implementation-defined metadata.
type Error struct {
	Message    string         `json:"message"`
	Locations  []Location     `json:"locations,omitempty"`
	Path       []any          `json:"path,omitempty"`
	Extensions map[string]any `json:"extensions,omitempty"`
}

// Location points at a concrete source position inside the GraphQL
// document. Both fields are 1-based per the GraphQL spec.
type Location struct {
	Line   int `json:"line"`
	Column int `json:"column"`
}

// IncrementalPayload is a single GraphQL incremental delivery patch.
// It is shared across HTTP, SSE, and WebSocket transports.
type IncrementalPayload struct {
	Label      string         `json:"label,omitempty"`
	Path       []any          `json:"path,omitempty"`
	Data       any            `json:"data,omitempty"`
	Items      []any          `json:"items,omitempty"`
	Errors     []Error        `json:"errors,omitempty"`
	Extensions map[string]any `json:"extensions,omitempty"`
}

// OrderedObject preserves GraphQL field order when serialised to JSON.
// Use it for response payloads instead of plain maps when the wire order
// must follow the query selection order.
type OrderedObject struct {
	fields []OrderedField
}

// OrderedField is one field/value pair inside an [OrderedObject].
type OrderedField struct {
	Name  string
	Value any
}

// NewOrderedObject allocates an object sized for capacity fields.
func NewOrderedObject(capacity int) *OrderedObject {
	return &OrderedObject{fields: make([]OrderedField, 0, capacity)}
}

// Set assigns or appends a field/value pair preserving selection order.
// Duplicates overwrite the earliest matching name.
func (o *OrderedObject) Set(name string, value any) {
	n := len(o.fields)
	if n > 0 && o.fields[n-1].Name == name {
		o.fields[n-1].Value = value
		return
	}
	for index := range o.fields {
		if o.fields[index].Name == name {
			o.fields[index].Value = value
			return
		}
	}
	o.fields = append(o.fields, OrderedField{Name: name, Value: value})
}

// Fields returns the object's ordered field/value pairs.
func (o *OrderedObject) Fields() []OrderedField {
	return o.fields
}

// Map returns a shallow unordered copy keyed by field name.
func (o *OrderedObject) Map() map[string]any {
	result := make(map[string]any, len(o.fields))
	for _, field := range o.fields {
		result[field.Name] = field.Value
	}
	return result
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

// IncrementalExecutor is an optional capability implemented by executors that
// support GraphQL incremental delivery (@defer / @stream). Transports type-assert
// the Executor to this interface to negotiate a multipart/mixed response.
type IncrementalExecutor interface {
	// HasIncrementalDirectives reports whether the operation selected by req
	// uses @defer or @stream, so the transport can decide whether to stream an
	// incremental-delivery response.
	HasIncrementalDirectives(req Request) bool

	// ExecuteIncremental runs the operation and returns the initial response
	// (with HasNext set) plus the ordered incremental payloads that follow.
	ExecuteIncremental(ctx context.Context, req Request) (Response, []IncrementalPayload)
}
