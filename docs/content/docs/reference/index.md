---
title: API Reference
description: Generated reference for every public grx package, sourced from Go doc comments.
---

Use this section when you already know which package or symbol you need. It
lists the public Go API and links each package to its detailed reference page.
For explanations and complete tasks, start with [Run your first
server](/getting-started/) or choose a topic from [Why grx](/concepts/why-grx).

The package pages are generated from Go doc comments. If a symbol description
is incorrect, update the comment in the source tree and regenerate the
reference pages.

## Packages

| Package          | What it covers                                                                                          |
| ---------------- | ------------------------------------------------------------------------------------------------------- |
| [`grx`](/reference/grx/)                         | Root package: [`NewServer`](/reference/grx/#NewServer) (`WithSchema`, plugins, transports, playground paths, [`WithFieldAuthorizer`](/reference/grx/#WithFieldAuthorizer) / [`WithOperationAuthorizer`](/reference/grx/#WithOperationAuthorizer)). |
| [`core`](/reference/core/)                       | Shared types: `Request`, `Response`, `Error`, `Executor`, `Transport`, `OperationKind`. No upward imports. |
| [`schema`](/reference/schema/)                   | Code-first schema builder (`schema.Config`, `schema.Build`). Reflects user Go types into runtime metadata. |
| [`exec`](/reference/exec/)                       | Lexer, parser, AST, executor, introspection fast-path.                                                     |
| [`server`](/reference/server/)                   | `http.Handler`, GraphiQL playground, transport dispatch (auto-appends the [`http`](/reference/http/) transport). |
| [`plugins`](/reference/plugins/)                   | Plugin lifecycle interface, `plugins.Base` no-op embed.                                                     |
| [`plugins/logger`](/reference/plugins/logger/)     | Built-in `log/slog`-backed request logger.                                                                 |
| [`http`](/reference/http/)                         | GraphQL-over-HTTP+JSON transport (the canonical `POST /graphql` request).                                    |
| [`sse`](/reference/sse/)                           | GraphQL over Server-Sent Events transport.                                                                  |
| [`websocket`](/reference/websocket/)               | RFC 6455 WebSockets implementing the `graphql-transport-ws` subprotocol.                                  |
| [`pubsub`](/reference/pubsub/)                     | Backend-neutral byte-oriented pub/sub contract.                                                            |
| [`memory`](/reference/memory/)                     | Process-local `PubSub` implementation.                                                                     |
| [`redis`](/reference/redis/)                       | Redis-backed `PubSub` implementation in a separate Go submodule.                                           |

## Hosted Go documentation

For the canonical, always-up-to-date reference rendered exactly the way
the rest of the Go ecosystem renders it, see
[**pkg.go.dev**](https://pkg.go.dev/github.com/grx-gql/grx). The
content here comes from the same source comments. This site places those
comments inside the rest of the grx documentation so navigation stays in one
place.
