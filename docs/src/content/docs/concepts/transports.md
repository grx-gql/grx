---
title: Transports
description: How protocol handlers attach to the server.
sidebar:
  order: 5
---

A **transport** is a protocol implementation that sits in front of the
executor: HTTP+JSON, WebSocket, SSE, and anything else that can carry
GraphQL traffic. They share a tiny interface so the server doesn't need to
know what's on the wire — every byte that crosses the network goes through
a transport, including the canonical `POST /graphql` JSON request.

## The interface

```go
// core/transport.go
type Transport interface {
    Match(r *http.Request) bool
    Serve(w http.ResponseWriter, r *http.Request, executor Executor)
}
```

- `Match` is consulted on every incoming request. It must be **cheap and
  side-effect free** — typically a header or path check.
- `Serve` is only called after `Match` returned true. The transport owns
  the response from there; nothing else writes to `w`.

The server iterates registered transports **in order** and the first one
to match wins. The default `pkg/http` HTTP+JSON transport is appended to
the chain automatically, so a plain `POST /graphql` request always has a
handler even when `Transports` is empty.

## Built-in transports

| Package           | Protocol                                                                                                                          |
| ----------------- | --------------------------------------------------------------------------------------------------------------------------------- |
| `pkg/http`        | The canonical GraphQL-over-HTTP+JSON `POST /graphql` request. Appended automatically by `server.New`; register it explicitly only when you need it to run before another transport that also matches POST. |
| `pkg/sse`         | GraphQL over Server-Sent Events. Activates when the client sends `Accept: text/event-stream` to `/graphql`.                       |
| `pkg/websocket`   | RFC 6455 WebSockets implementing the `graphql-transport-ws` subprotocol (the modern `graphql-ws` library protocol).               |

Default behaviour with no transports configured: the server still answers
`POST /graphql` (via the auto-appended `pkg/http` transport), the
playground, and `/favicon.ico`.

## Registration

```go
import (
    "github.com/patrickkabwe/grx/core"
    grxhttp "github.com/patrickkabwe/grx/pkg/http"
    "github.com/patrickkabwe/grx/pkg/sse"
    "github.com/patrickkabwe/grx/pkg/websocket"
    "github.com/patrickkabwe/grx/server"
)

srv, _ := server.New(server.Config{
    Schema: graph.NewSchema(),
    Transports: []core.Transport{
        websocket.New(websocket.Config{
            ConnectionInitTimeout: 3 * time.Second,
            ReadIdleTimeout:       60 * time.Second,
            WriteTimeout:          10 * time.Second,
            MaxMessageSize:        1 << 20,
            CheckOrigin:           originAllowlist,
            OnConnect:             authorize,
            PingInterval:          25 * time.Second,
        }),
        sse.New(),
        // Optional: register grxhttp.New() yourself when you want to
        // place it ahead of another POST-matching transport. server.New
        // appends one automatically when you don't.
        grxhttp.New(),
    },
})
```

The `pkg/http` package is named `http`, which collides with the standard
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
| `CheckOrigin`             | Origin allowlist hook. Default is "same-origin only"; production needs a real allowlist. |
| `OnConnect`               | Auth hook. Receives the init payload, returns the per-connection context and ack data.  |
| `PingInterval`            | If non-zero, the server emits application-level pings on this cadence.                   |

The framer enforces RFC 6455 strictly: client masking, fragmentation,
control-frame size, reserved bits, UTF-8 validation, reserved opcodes,
and continuation ordering all map to the spec-defined close codes.

## SSE configuration

`sse.New()` is intentionally minimal. It accepts both `POST` requests
with a JSON body and `GET` requests with `query=`/`variables=`/`operationName=`
parameters, then streams `event: next` / `event: complete` records back.

## When to write a custom transport

If you need a wire format the built-in transports don't cover —
GraphQL-over-HTTP `application/graphql-response+json`, multipart for file
uploads, gRPC, your own pub/sub bridge — you write a transport. See
[Custom Transport](/guides/custom-transport/) for a worked example.
