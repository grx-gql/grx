---
title: API Reference
description: Generated reference for every public grx package, sourced from Go doc comments.
outline: [2, 3]
---

# API Reference

This section is generated from the Go doc comments in the grx module by
[`gomarkdoc`](https://github.com/princjef/gomarkdoc) and rebuilt on every push
to `main`. If something reads wrong here, update the doc comment on the symbol
in the source tree and regenerate.

## Packages

| Package          | What it covers                                                                                          |
| ---------------- | ------------------------------------------------------------------------------------------------------- |
| [`grx`](/reference/grx/)                         | Root package: [`NewServer`](/reference/grx/#NewServer) (`WithSchema`, [`WithMiddleware`](/reference/grx/#WithMiddleware), plugins, transports, playground paths, [`WithFieldAuthorizer`](/reference/grx/#WithFieldAuthorizer) / [`WithOperationAuthorizer`](/reference/grx/#WithOperationAuthorizer)). |
| [`core`](/reference/core/)                       | Shared types: `Request`, `Response`, `Error`, `Executor`, `Transport`, `OperationKind`. No upward imports. |
| [`schema`](/reference/schema/)                   | Code-first schema builder (`schema.Config`, `schema.Build`). Reflects user Go types into runtime metadata. |
| [`exec`](/reference/exec/)                       | Lexer, parser, AST, executor, introspection fast-path.                                                     |
| [`server`](/reference/server/)                   | `http.Handler`, GraphiQL playground, transport dispatch (auto-appends the `pkg/http` transport).           |
| [`plugin`](/reference/plugin/)                   | Plugin lifecycle interface, `plugin.Base` no-op embed.                                                     |
| [`plugin/logger`](/reference/plugin/logger/)     | Built-in `log/slog`-backed request logger.                                                                 |
| [`pkg/http`](/reference/pkg/http/)               | GraphQL-over-HTTP+JSON transport (the canonical `POST /graphql` request).                                  |
| [`pkg/sse`](/reference/pkg/sse/)                 | GraphQL over Server-Sent Events transport.                                                                 |
| [`pkg/websocket`](/reference/pkg/websocket/)     | RFC 6455 WebSockets implementing the `graphql-transport-ws` subprotocol.                                   |
| [`pkg/pubsub`](/reference/pkg/pubsub/)           | Pub/Sub interface, in-memory backend, filters, and the generic `Typed[T]` wrapper for subscription fan-out.|
| [`pkg/pubsub/redis`](/reference/pkg/pubsub/redis/) | Redis-backed `PubSub` implementation. Lives in its own Go submodule so the root grx module stays dependency-free. |

## Hosted Go documentation

For the canonical, always-up-to-date reference rendered exactly the way
the rest of the Go ecosystem renders it, see
[**pkg.go.dev**](https://pkg.go.dev/github.com/patrickkabwe/grx). The
content here is the same source comments — this site just embeds them
inside the rest of the documentation so navigation stays in one place.
