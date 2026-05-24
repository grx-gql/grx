---
name: graphql-grx
description: >-
  Guides AI assistants when implementing or refactoring GraphQL servers and
  resolvers using the grx Go library (`github.com/grx-gql/grx`): struct-based
  schema with `gql` tags, `grx.NewServer` / `server.New`, transports (HTTP JSON,
  WebSocket graphql-transport-ws, SSE), built-in directives, authorizers,
  plugins, persisted queries, and subscriptions with pub/sub. Use when the user mentions grx, GraphQL-in-Go
  with structs, pkg.go.dev/github.com/grx-gql/grx, or code that imports
  github.com/grx-gql/grx.
---

# graphql-grx (app development skill)

Audience: engineers building applications **with** **`grx`**, not necessarily contributing to `grx` itself. For hacking the **repository**, read [AGENT.md](https://github.com/grx-gql/grx/blob/main/AGENT.md).

## Canonical references

- **Pkg docs:** [pkg.go.dev/github.com/grx-gql/grx](https://pkg.go.dev/github.com/grx-gql/grx)
- **`client`:** outbound GraphQL-over-HTTP helper (`github.com/grx-gql/grx/client`) when calling another graph from Go or **`httptest` integration checks** (**[Testing guide](https://grx-gql.github.io/grx/guides/testing)**).
- **Docs site:** [grx-gql.github.io/grx](https://grx-gql.github.io/grx/) (guides for auth, uploads, persisted queries, subscriptions, security, deployment…)
- **Runnable patterns:** Clone the repo and mirror `examples/basic`, `examples/auth`, `examples/subscriptions`, `examples/file-upload`, etc.

## Mental model

- **Code-first:** exported structs/methods on root types become GraphQL types and fields (`schema.Config` feeds `grx.WithSchema`).
- **`grx.Server` is `http.Handler`**: mount behind Chi, Gin, Echo, Fiber, `ServeMux`, or `net/http`.
- **`server.New(server.Config{})`** exposes advanced knobs (**`MaskInternalErrors`**, selection limits, lexer cache…) when **`grx.NewServer`** helpers are insufficient (**[`Server`](https://pkg.go.dev/github.com/grx-gql/grx/server#Server)**).
- Built-ins enumerated in **`__schema.directives`**: **`skip`**, **`include`**, **`defer`**, **`stream`**, **`deprecated`**, **`specifiedBy`**, **`oneOf`** (incremental **`@defer/@stream`** needs **`multipart/mixed`** (**[guide](https://grx-gql.github.io/grx/guides/schema-directives)**)).

## Schema & resolvers

- Return **`schema.Config`** `{ Query:, Mutation:, Subscription?: }`; Query is required.
- Resolver shapes: **`(ctx context.Context, args Args) (*T, error)`** for query/mutation; subscriptions return **`(<-chan *T, error)`** where appropriate.
- Use **`gql:"fieldName"`** / **`gql:"fieldName,nonNull"`** on struct tags; method names expose lowercased field names (**`Users`** → **`users`** unless tagged).
- Split large graphs by embedding per-entity `XQuery`, `XMutation`, `XSubscription` into root structs (avoid conflicting method names on the same composite root).

## Transports & server options

- Default **GraphQL-over-HTTP POST JSON** ships via **`http`** (**`github.com/grx-gql/grx/http`**, auto-appended). Use **`grx.WithSubscriptionPath`**, **`grx.WithTransports`**, **`websocket.New`**, and **`sse.New`** from **`websocket`** / **`sse`** for realtime (**`graphql-transport-ws`**, not legacy Apollo **`subscriptions-transport-ws`**).
- **WebSocket hardening:** set **`CheckOrigin`**, timeouts, **`MaxMessageSize`**, and **`OnConnect`** for JWT/session hydration—see **[Subscriptions guide](https://grx-gql.github.io/grx/guides/subscriptions.html)** (**Securing subscriptions**).

## Authorization, plugins, observability

- **`grx.WithOperationAuthorizer`** / **`grx.WithFieldAuthorizer`** (or equivalents on **`server.Config`**) gate parsed operations and fields; HTTP middleware attaches identity before GraphQL executes.
- **Plugins** consume lifecycle hooks (**`plugin`** package); embed **`plugin.Base`** for partial implementations.
- Prefer **[Security](https://grx-gql.github.io/grx/guides/production-security.html)**, **[Auth](https://grx-gql.github.io/grx/guides/auth.html)**, **[Limits](https://grx-gql.github.io/grx/guides/request-limits.html)** when production-hardening.

## Things to avoid (common hallucinations)

- Do not invent SDL-first workflows unless the user deliberately layers SDL on top—the runtime contract is structs + **`gql` tags** plus **`schema.Config`**.
- Do not substitute Apollo Server / Node patterns for Go package layout; **`grx`** wires **`exec`**, **`schema`**, **`server`**, and small root-level transport packages (**`http`**, **`sse`**, **`websocket`**, **`memory-pubsub`**, **`redis-pubsub`**)—app code usually imports **`github.com/grx-gql/grx`** plus optional siblings.
- Do not claim **`graphql-ws`** (legacy Apollo subprotocol)—only **`graphql-transport-ws`** is supported for WebSockets.

## Validate changes

Before finishing a substantive edit in the user’s **`go.mod`** project:

```bash
go test ./...
go vet ./...
gofmt -w .
```

For HTTP-level **`grx`** integration tests against a real handler, mirror **`github.com/grx-gql/grx/client`** under **`httptest.Server`** (**[Testing with the HTTP client](https://grx-gql.github.io/grx/guides/testing)** on the docs site).

(From the **`grx` repo** itself contributors also run **`make test`** / **`make test-race`**; see **`AGENT.md`**.)
