---
title: Resolver methods
description: How grx turns a GraphQL field into a Go method - arguments, return values, context, errors, and subscriptions.
outline: [2, 3]
---

# Resolver methods

In GraphQL terms, a **resolver** is the function that runs when a client asks for a field. In grx, that function is almost always a **Go method**: either a method on your `Query` / `Mutation` / `Subscription` struct (top-level fields), or - under the hood - property reads on structs you return (nested object fields are read from the struct, not separate methods you write).

This page covers **methods you write yourself**: legal signatures, `context`, errors, and subscription channels.

## Signatures

For queries and mutations:

```go
package graph

import "context"

func (T) FieldName(ctx context.Context, args TArgs) (*TResult, error)
```

For subscriptions:

```go
package graph

import "context"

func (T) FieldName(ctx context.Context, args TArgs) (<-chan *TResult, error)
```

Both `ctx` and `args` are **optional**, in that order. Omit either when the
resolver does not need it:

```go
package graph

import (
	"context"
	"time"
)

func (Query) Hello() (string, error)

func (Query) Now(ctx context.Context) (time.Time, error)

func (Query) User(args UserArgs) (*User, error)
```

(`User`, `UserArgs`, and concrete return handling are declared alongside your schema.)

## Result types

- Queries and mutations return any type that maps to a GraphQL output type
  (object struct, scalar, list of either, pointer for nullability).
- Subscriptions return a **receive-only channel**. The element type
  determines the GraphQL output type. The executor consumes the channel and
  dispatches each value to subscribers.

`nil` and the zero value are distinguished only when the field is nullable
(typically `*T` returns)  -  for `*T` results, returning `nil, nil` produces
a `null` field; for required (`T`) results, the zero value is sent as-is.

## Context

`ctx` is the per-request context. It carries:

- The `http.Request` deadline and any cancellation triggered by the client
  disconnecting.
- Any values added by plugins in `RequestStart`.
- For subscriptions, cancellation when the WebSocket or SSE connection is
  closed by either side.

Always propagate `ctx` into downstream calls (database, RPC, channel sends)
so the cleanup signal reaches them. The subscriptions guide has the full
walkthrough; the pattern is always the same: honour `ctx.Done()` when sending
on the stream:

```go
package graph

import (
	"context"
	"time"
)

func (UserSubscription) UserCreated(ctx context.Context) (<-chan *User, error) {
	stream := make(chan *User)
	go func() {
		defer close(stream)
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				select {
				case <-ctx.Done():
					return
				case stream <- &User{ /* ... */ }:
				}
			}
		}
	}()
	return stream, nil
}
```

## Errors

Return a normal Go `error` to surface a field-level error in the GraphQL
response. The executor:

- Sets the affected field to `null`.
- Appends an entry to `errors[]` with `path` populated.
- Continues executing sibling fields. The rest of the response is preserved
  as `data`.

Use `core.Error` directly when you need to control the message that reaches
the client:

```go
package graph

import "github.com/grx-gql/grx/core"

func findUser() (*User, error) {
	return nil, &core.Error{
		Message: "user not found",
		Path:    []any{"user"},
	}
}
```

::: warning Don't leak internals
Wrap or sanitize errors before returning them. Stack traces and database
errors often contain information that should not be sent to clients. The
[Roadmap](/roadmap) tracks
the upcoming first-class error masking story; baseline hygiene applies now.
:::

## Concurrency expectations

- Resolvers can be called from multiple goroutines simultaneously across
  requests. Per-request state belongs in `ctx`, never on the resolver
  struct.
- Per-entity resolver structs may hold long-lived dependencies (a database
  pool, an RPC client)  -  treat those as immutable after construction.
- Subscriptions own their goroutine: spawn workers from the resolver, but
  always honour `ctx.Done()` to release them.
