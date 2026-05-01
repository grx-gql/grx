---
title: Plugins
description: Lifecycle hooks for cross-cutting concerns like logging, tracing, and auth.
sidebar:
  order: 7
---

Plugins are how grx exposes the request lifecycle. They're the right place
for logging, tracing, metrics, request-scoped state, and policy decisions
that shouldn't live inside resolvers.

## The interface

```go
// plugin/plugin.go
type Plugin interface {
    RequestStart(ctx context.Context, req core.Request) (context.Context, error)
    ParsingStart(ctx context.Context, req core.Request) error
    ValidationStart(ctx context.Context, req core.Request) error
    ExecutionStart(ctx context.Context, req core.Request) error
    FieldResolveStart(ctx context.Context, field FieldContext) error
    ResponseSend(ctx context.Context, res core.Response) error
    Error(ctx context.Context, err error)
}
```

Two things make this interface comfortable to implement:

- `RequestStart` is the **only** hook that returns a derived
  `context.Context`. Add request-scoped values here.
- All hooks but `Error` may return an error. A non-nil error short-circuits
  the request — useful for auth, rate-limiting, and budget checks.

## `plugin.Base`

Embed `plugin.Base` to inherit no-op defaults for every hook. New hooks
added in future releases will keep your plugin compiling:

```go
type RequestID struct{ plugin.Base }

func (RequestID) RequestStart(ctx context.Context, _ core.Request) (context.Context, error) {
    return context.WithValue(ctx, requestIDKey{}, uuid.NewString()), nil
}
```

## Registration

Plugins run in the order they're registered. Put plugins that produce
request-scoped values (request ids, trace spans) **before** plugins that
consume them.

```go
import (
    "log/slog"
    "os"

    "github.com/patrickkabwe/grx/plugin"
    "github.com/patrickkabwe/grx/plugin/logger"
    "github.com/patrickkabwe/grx/server"
)

loggerPlugin, _ := logger.New(logger.Config{
    Logger: slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelDebug})),
})

srv, _ := server.New(server.Config{
    Schema:  graph.NewSchema(),
    Plugins: []plugin.Plugin{loggerPlugin},
})
```

## Concurrency rules

- Plugins are shared across requests. Do not store per-request state on
  the plugin struct — put it in the `context.Context` from `RequestStart`
  and read it back from later hooks.
- Hooks run on the request goroutine. If you need to do I/O, don't block
  unnecessarily; plugins are part of the latency budget.
- `FieldResolveStart` fires for every resolved field. Keep it cheap, or
  gate expensive work behind a sampling decision made in `RequestStart`.

## Built-in plugins

The [`plugin/logger`](/reference/plugin/logger/) subpackage ships a
structured logger that emits a `log/slog` event for each lifecycle hook
with operation name, payload sizes, and any error. See
[Custom Plugin](/guides/custom-plugin/) for a worked example of writing
your own.
