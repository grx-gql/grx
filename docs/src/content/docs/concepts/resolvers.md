---
title: Resolvers
description: Resolver method signatures, context propagation, and error semantics.
sidebar:
  order: 3
---

A resolver in grx is just a method on a root struct. The signature follows
a small set of rules, both arguments and the result are typed Go values, and
the GraphQL response is built from what you return.

## Signatures

For queries and mutations:

```go
func (T) FieldName(ctx context.Context, args TArgs) (*TResult, error)
```

For subscriptions:

```go
func (T) FieldName(ctx context.Context, args TArgs) (<-chan *TResult, error)
```

Both `ctx` and `args` are **optional**, in that order. Omit either when the
resolver does not need it:

```go
func (Query) Hello() (string, error)                       // no ctx, no args
func (Query) Now(ctx context.Context) (time.Time, error)   // ctx only
func (Query) User(args UserArgs) (*User, error)            // args only
```

## Result types

- Queries and mutations return any type that maps to a GraphQL output type
  (object struct, scalar, list of either, pointer for nullability).
- Subscriptions return a **receive-only channel**. The element type
  determines the GraphQL output type. The executor consumes the channel and
  dispatches each value to subscribers.

`nil` and the zero value are distinguished only when the field is nullable
(typically `*T` returns) — for `*T` results, returning `nil, nil` produces
a `null` field; for required (`T`) results, the zero value is sent as-is.

## Context

`ctx` is the per-request context. It carries:

- The `http.Request` deadline and any cancellation triggered by the client
  disconnecting.
- Any values added by plugins in `RequestStart`.
- For subscriptions, cancellation when the WebSocket or SSE connection is
  closed by either side.

Always propagate `ctx` into downstream calls (database, RPC, channel sends)
so the cleanup signal reaches them. The `examples/basic` user subscription
shows the pattern:

```go
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
return nil, &core.Error{
    Message: "user not found",
    Path:    []any{"user"},
}
```

:::caution[Don't leak internals]
Wrap or sanitize errors before returning them. Stack traces and database
errors often contain information that should not be sent to clients. The
[security checklist](https://github.com/patrickkabwe/grx#security) tracks
the upcoming first-class error masking story; baseline hygiene applies now.
:::

## Concurrency expectations

- Resolvers can be called from multiple goroutines simultaneously across
  requests. Per-request state belongs in `ctx`, never on the resolver
  struct.
- Per-entity resolver structs may hold long-lived dependencies (a database
  pool, an RPC client) — treat those as immutable after construction.
- Subscriptions own their goroutine: spawn workers from the resolver, but
  always honour `ctx.Done()` to release them.
