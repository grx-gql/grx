---
title: Write a Custom Transport
description: Implement core.Transport to support a wire format the built-ins don't cover.
outline: [2, 3]
---

If you need a wire format the built-in HTTP+JSON, WebSocket, or SSE
transports don't cover, you implement `core.Transport`. This guide
implements `GET /graphql?query=…` (a small but useful piece of the
[GraphQL-over-HTTP](https://github.com/graphql/graphql-over-http) spec)
to show the shape.

## The interface, in two methods

```go
type Transport interface {
    Match(r *http.Request) bool
    Serve(w http.ResponseWriter, r *http.Request, executor Executor)
}
```

- `Match` is called on every request. Make it cheap and side-effect free.
- `Serve` owns the response from the moment `Match` returns true.

## The transport

```go
package httpget

import (
    "encoding/json"
    "net/http"

    "github.com/patrickkabwe/grx/core"
)

type Transport struct{}

func New() Transport { return Transport{} }

func (Transport) Match(r *http.Request) bool {
    if r.Method != http.MethodGet || r.URL.Path != "/graphql" {
        return false
    }
    return r.URL.Query().Get("query") != ""
}

func (t Transport) Serve(w http.ResponseWriter, r *http.Request, executor core.Executor) {
    q := r.URL.Query()

    body := core.GraphQLBody{
        Query:         q.Get("query"),
        OperationName: q.Get("operationName"),
    }
    if raw := q.Get("variables"); raw != "" {
        if err := json.Unmarshal([]byte(raw), &body.Variables); err != nil {
            core.WriteJSON(w, http.StatusBadRequest, core.Response{
                Errors: []core.Error{{Message: "invalid variables JSON: " + err.Error()}},
            })
            return
        }
    }

    res := executor.Execute(r.Context(), core.Request{
        Query:         body.Query,
        OperationName: body.OperationName,
        Variables:     body.Variables,
    })

    core.WriteJSON(w, http.StatusOK, res)
}
```

What this gets you:

- A correctly-decoded `GET /graphql?query=…&variables=%7B%22id%22%3A1%7D`
  request.
- Response routed through the same executor as the JSON `POST` handler,
  so plugins, validation, and field error semantics are identical.

## Register it

The server consults transports in **order**. Register your custom
transport in the position you want it considered:

```go
srv, _ := grx.NewServer(
    grx.WithSchema(graph.NewSchema()),
    grx.WithTransports(httpget.New()),
)
```

`grx.NewServer` always appends the built-in `pkg/http` transport at the
end of the chain, so the canonical `POST /graphql` JSON request keeps
working without any explicit registration. To override that default
behaviour, register a `pkg/http`-style POST-matching transport ahead of
the others.

If you also want WebSocket and SSE, list them in the order you want them
considered — the first `Match` wins:

```go
grx.WithTransports(
    websocket.New(),
    sse.New(),
    httpget.New(),
)
```

## Rules of thumb

- **One transport, one wire format.** If you find yourself branching on
  request shape inside `Serve`, you probably want a second transport.
- **Don't reach into `schema` or `exec`.** Transports talk to the
  `core.Executor` only — that's what keeps the layering honest.
- **Surface request-level errors with `core.WriteJSON`** before any
  streaming has started. Once you've started writing a stream, you owe
  the protocol-specific termination message.
- **Honour `r.Context()`.** It's already cancelled when the client
  disconnects; pass it through to the executor and to any goroutines you
  spawn.
