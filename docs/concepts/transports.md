---
title: HTTP, WebSocket, and SSE
description: How transports attach to the server and why order matters when more than one matches a request.
outline: [2, 3]
---

# HTTP, WebSocket, and SSE

A **transport** is a protocol implementation that sits in front of the
executor: HTTP+JSON, WebSocket, SSE, and anything else that can carry
GraphQL traffic. They share a tiny interface so the server doesn't need to
know what's on the wire  -  every byte that crosses the network goes through
a transport, including the canonical `POST /graphql` JSON request.

## The interface

```go
type Transport interface {
    Match(r *http.Request) bool
    Serve(w http.ResponseWriter, r *http.Request, executor Executor)
}
```

- `Match` is consulted on every incoming request. It must be **cheap and
  side-effect free**  -  typically a header or path check.
- `Serve` is only called after `Match` returned true. The transport owns
  the response from there; nothing else writes to `w`.

The server iterates registered transports **in order** and the first one
to match wins. The default `http` HTTP+JSON transport is appended to
the chain automatically, so a plain `POST /graphql` request always has a
handler even when `Transports` is empty.

## Built-in transports

| Package           | Protocol                                                                                                                          |
| ----------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| `http`        | The canonical GraphQL-over-HTTP+JSON `POST /graphql` request. Appended automatically by `grx.NewServer`; register it explicitly only when you need it to run before another transport that also matches POST. |
| `sse`         | GraphQL over Server-Sent Events. Activates when the client sends `Accept: text/event-stream` to `/graphql`.                       |
| `websocket`   | RFC 6455 WebSockets implementing the `graphql-transport-ws` subprotocol (the modern `graphql-ws` library protocol).               |

Default behaviour with no transports configured: the server still answers
`POST /graphql` (via the auto-appended `http` transport), the
playground, and `/favicon.ico`.

## Splitting query vs subscription paths

If you set [`server.Config.SubscriptionPath`](https://pkg.go.dev/github.com/grx-gql/grx/server#Config)
(or `grx.WithSubscriptionPath`), and it differs from the GraphQL POST path
(default `/graphql`), **only** concrete `*websocket.WebSocketTransport` and `*sse.Transport`
values from your `Transports` slice are moved to the subscription path.
The default `http` transport stays on the GraphQL path.

Many production setups use a **single URL** for both `POST` queries and
`graphql-transport-ws` (GraphiQL expects that by default). Splitting paths
is optional - use it when your gateway or operations team wants HTTP and
long-lived streams explicitly separated.

## Registration

```go
import (
    "github.com/grx-gql/grx"
    grxhttp "github.com/grx-gql/grx/http"
    "github.com/grx-gql/grx/sse"
    "github.com/grx-gql/grx/websocket"
)

srv, _ := grx.NewServer(
    grx.WithSchema(graph.NewSchema()),
    grx.WithTransports(
        websocket.New(websocket.Config{
            ConnectionInitTimeout: 3 * time.Second,
            ReadIdleTimeout:       60 * time.Second,
            WriteTimeout:          10 * time.Second,
            MaxMessageSize:        1 << 20,
            CheckOrigin:           originAllowlist,
            OnConnect:             authorize,
            PingInterval:          25 * time.Second,
        }),
        sse.New(sse.Config{
            MaxActiveSubscriptions: 256,
        }),
        // Optional: register grxhttp.New() yourself when you want to
        // place it ahead of another POST-matching transport.
        // grx.NewServer appends one automatically when you don't.
        grxhttp.New(),
    ),
)
```

The `http` package is named `http`, which collides with the standard
library's `net/http`. Import it under an alias (we use `grxhttp` in the
docs) when you need both packages in the same file.

## WebSocket configuration

The `websocket.Config` struct exposes the production knobs you'd expect
from a long-lived connection:

| Field                     | Purpose                                                                                  |
| ------------------------- | ---------------------------------------------------------------------------------------- |
| `ConnectionInitTimeout`   | Maximum time to wait for the client's `connection_init` message.                         |
| `ReadIdleTimeout`         | Closes connections that haven't sent any frame in this window.                           |
| `WriteTimeout`            | Per-frame write deadline; protects the server from slow consumers.                       |
| `MaxMessageSize`          | Hard limit on a single message; oversize frames cause a `1009` close.                    |
| `CheckOrigin`             | Origin allowlist hook. Nil accepts every origin; production needs a real allowlist. |
| `OnConnect`               | Auth hook. Receives the init payload, returns the per-connection context and ack data.  |
| `PingInterval`            | If non-zero, the server emits application-level pings on this cadence.                   |
| `MaxSubscriptions`        | Cap active `subscribe` operations per WebSocket; zero means unlimited. When exceeded, the server sends an error and `complete` for that id. |

The framer enforces RFC 6455 strictly: client masking, fragmentation,
control-frame size, reserved bits, UTF-8 validation, reserved opcodes,
and continuation ordering all map to the spec-defined close codes.

## SSE configuration

`sse.New()` builds a transport with default limits (none). Pass
`sse.Config{MaxActiveSubscriptions: N}` when you need a hard cap on concurrent
SSE streams for that transport value: over-limit handshakes receive `429` with a
JSON GraphQL error body before the `text/event-stream` response begins.

The handler accepts both `POST` requests with a JSON body and `GET` requests
with `query=`/`variables=`/`operationName=` parameters, then streams
`event: next` / `event: complete` records back.

## When to write a custom transport

If you need a wire format the built-in transports don't cover  - 
GraphQL-over-HTTP `application/graphql-response+json`, multipart for file
uploads, gRPC, your own pub/sub bridge  -  you write a transport. See
[Custom Transport](/guides/custom-transport) for a worked example.
