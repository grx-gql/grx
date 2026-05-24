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
| [`server`](/reference/server/)                   | `http.Handler`, GraphiQL playground, transport dispatch (auto-appends the [`http`](/reference/http/) transport). |
| [`plugins`](/reference/plugins/)                   | Plugin lifecycle interface, `plugins.Base` no-op embed.                                                     |
| [`plugins/logger`](/reference/plugins/logger/)     | Built-in `log/slog`-backed request logger.                                                                 |
| [`middlewares`](/reference/middlewares/)           | HTTP middleware helpers such as request ID propagation.                                                     |
| [`http`](/reference/http/)                         | GraphQL-over-HTTP+JSON transport (the canonical `POST /graphql` request).                                    |
| [`sse`](/reference/sse/)                           | GraphQL over Server-Sent Events transport.                                                                  |
| [`websocket`](/reference/websocket/)               | RFC 6455 WebSockets implementing the `graphql-transport-ws` subprotocol.                                  |
| [`memory-pubsub`](/reference/memory-pubsub/)       | Pub/Sub interface (`package pubsub`): in-memory backend, filters, and the generic `Typed[T]` wrapper.      |
| [`redis-pubsub`](/reference/redis-pubsub/)         | Redis-backed `PubSub` implementation. Separate Go submodule so the root grx module stays dependency-free.  |

## Hosted Go documentation

For the canonical, always-up-to-date reference rendered exactly the way
the rest of the Go ecosystem renders it, see
[**pkg.go.dev**](https://pkg.go.dev/github.com/grx-gql/grx). The
content here is the same source comments  -  this site just embeds them
inside the rest of the documentation so navigation stays in one place.
