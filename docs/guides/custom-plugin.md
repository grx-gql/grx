---
title: Write a Custom Plugin
description: Implement a request-id plugin that propagates an id through the lifecycle.
outline: [2, 3]
---

Plugins are the right tool for cross-cutting concerns: logging, tracing,
metrics, request-id propagation, auth checks. This guide implements a
small **request-id** plugin to show the lifecycle in action.

## What we're building

- Generate a UUID at the start of every request.
- Attach it to the `context.Context` so resolvers can read it.
- Add it to the response `extensions` block so clients can correlate.
- Echo it on errors for log scraping.

## The plugin

```go
package extensions

import (
    "context"
    "crypto/rand"
    "encoding/hex"

    "github.com/patrickkabwe/grx/core"
    "github.com/patrickkabwe/grx/plugin"
)

type requestIDKey struct{}

func RequestIDFrom(ctx context.Context) string {
    if v, ok := ctx.Value(requestIDKey{}).(string); ok {
        return v
    }
    return ""
}

type RequestID struct{ plugin.Base }

func (RequestID) RequestStart(ctx context.Context, _ core.Request) (context.Context, error) {
    var b [16]byte
    _, _ = rand.Read(b[:])
    return context.WithValue(ctx, requestIDKey{}, hex.EncodeToString(b[:])), nil
}

func (RequestID) ResponseSend(ctx context.Context, res core.Response) error {
    if id := RequestIDFrom(ctx); id != "" {
        if res.Extensions == nil {
            res.Extensions = map[string]any{}
        }
        res.Extensions["requestId"] = id
    }
    return nil
}

func (RequestID) Error(ctx context.Context, err error) {
    if id := RequestIDFrom(ctx); id != "" {
        // Replace with your real logger.
        println("graphql error", id, err.Error())
    }
}
```

A few things worth calling out:

- The plugin embeds `plugin.Base`, so future lifecycle hooks added to the
  interface won't break the build.
- `RequestStart` is the **only** hook that may return a derived context.
  Add request-scoped values here, never on the plugin struct.
- `ResponseSend` runs once per response. Mutating `Extensions` here is
  the supported way to enrich the wire format from a plugin.

## Register it

```go
srv, _ := grx.NewServer(
    grx.WithSchema(graph.NewSchema()),
    grx.WithPlugins(
        loggerPlugin,
        extensions.RequestID{},
    ),
)
```

Plugins run in registration order. Put plugins that produce IDs **before**
plugins that consume them (e.g. the logger).

## Use it from a resolver

```go
func (Query) Whoami(ctx context.Context) (string, error) {
    return extensions.RequestIDFrom(ctx), nil
}
```

```graphql
{ whoami }
```

The response will look like:

```json
{
  "data": { "whoami": "1f1e2d3c4b5a69788796a5b4c3d2e1f0" },
  "extensions": { "requestId": "1f1e2d3c4b5a69788796a5b4c3d2e1f0" }
}
```

## Where to go next

- Hook `FieldResolveStart` to record per-field timing — but keep it cheap
  (it fires on every resolved field).
- Hook `ParsingStart` and `ValidationStart` to enforce caps on document
  size or depth.
- Use the same shape to bridge into OpenTelemetry: open a span in
  `RequestStart`, attach it to `ctx`, end it in `ResponseSend`.
