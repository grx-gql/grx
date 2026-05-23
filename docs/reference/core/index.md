---
title: core
description: API reference for the core package, generated from Go doc comments.
outline: [2, 4]
lastUpdated: false
---

# core

```go
```

Package core defines the transport\-agnostic types shared by the server, executor, and transport implementations. It deliberately has no upward imports so that schema, exec, server, plugin, and core sub\-packages can all depend on it without creating an import cycle.

## Index

- [func HeaderContains\(values \[\]string, needle string\) bool](<#HeaderContains>)
- [func WriteJSON\(w http.ResponseWriter, status int, value any\)](<#WriteJSON>)
- [type Error](<#Error>)
- [type Executor](<#Executor>)
- [type GraphQLBody](<#GraphQLBody>)
  - [func DecodeGraphQLBody\(r \*http.Request\) \(GraphQLBody, error\)](<#DecodeGraphQLBody>)
- [type OperationKind](<#OperationKind>)
- [type Request](<#Request>)
- [type Response](<#Response>)
- [type Transport](<#Transport>)


<a name="HeaderContains"></a>
## func HeaderContains

```go
func HeaderContains(values []string, needle string) bool
```

HeaderContains reports whether any of values contains needle as a case\-insensitive, comma\-separated token. Useful for parsing request headers like Connection or Accept.

<a name="WriteJSON"></a>
## func WriteJSON

```go
func WriteJSON(w http.ResponseWriter, status int, value any)
```

WriteJSON writes value as a JSON response with the given status. It is shared by transports that need to surface request\-level errors before any streaming has started.

<a name="Error"></a>
## type Error

Error is a single GraphQL error entry. Path identifies the field that produced the error \(using GraphQL response\-path semantics\) and Meta carries optional implementation\-defined metadata.

```go
type Error struct {
    Message string         `json:"message"`
    Path    []string       `json:"path,omitempty"`
    Meta    map[string]any `json:"meta,omitempty"`
}
```

<a name="Executor"></a>
## type Executor

Executor is the contract every transport uses to run GraphQL operations. Implementations must be safe for concurrent use by multiple goroutines.

```go
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
```

<a name="GraphQLBody"></a>
## type GraphQLBody

GraphQLBody is the canonical wire shape for an HTTP\-borne GraphQL request. Transports decode payloads into this struct before invoking the Executor.

```go
type GraphQLBody struct {
    Query         string         `json:"query"`
    OperationName string         `json:"operationName"`
    Variables     map[string]any `json:"variables"`
}
```

<a name="DecodeGraphQLBody"></a>
### func DecodeGraphQLBody

```go
func DecodeGraphQLBody(r *http.Request) (GraphQLBody, error)
```

DecodeGraphQLBody parses a JSON body into a GraphQLBody and returns a human\-readable error suitable for surfacing as a request\-level GraphQL error.

<a name="OperationKind"></a>
## type OperationKind

OperationKind enumerates the three GraphQL executable operation kinds. It is returned by \[Executor.OperationKind\] so transports can route a request to \[Executor.Execute\] or \[Executor.Subscribe\] without re\-implementing GraphQL parsing.

```go
type OperationKind string
```

<a name="OperationQuery"></a>Operation kinds defined by the GraphQL October 2021 spec §2.3.

```go
const (
    OperationQuery        OperationKind = "query"
    OperationMutation     OperationKind = "mutation"
    OperationSubscription OperationKind = "subscription"
)
```

<a name="Request"></a>
## type Request

Request is the executor\-facing representation of a single GraphQL operation. Transports decode their wire format into this struct before invoking \[Executor.Execute\] or \[Executor.Subscribe\].

```go
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
```

<a name="Response"></a>
## type Response

Response is the canonical GraphQL response envelope. Either Data, Errors, or both may be set. The omitempty JSON tags ensure that absent fields are not serialised, matching the GraphQL spec.

```go
type Response struct {
    Data   any     `json:"data,omitempty"`
    Errors []Error `json:"errors,omitempty"`
}
```

<a name="Transport"></a>
## type Transport

Transport plugs a GraphQL protocol \(WebSocket subscriptions, SSE, HTTP\+JSON, batching, ...\) into the server. The server iterates registered transports in order; the first one whose Match returns true takes ownership of the response.

```go
type Transport interface {
    // Match reports whether this transport wants to handle the given request.
    // It must be cheap and side-effect free.
    Match(r *http.Request) bool

    // Serve writes the protocol-specific response. It is only called after
    // Match returned true.
    Serve(w http.ResponseWriter, r *http.Request, executor Executor)
}
```

Generated by [gomarkdoc](<https://github.com/princjef/gomarkdoc>)
