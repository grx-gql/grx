---
title: Changelog
description: All notable changes to grx, in reverse chronological order.
outline: [2, 3]
---

> This page is mirrored from
> [`CHANGELOG.md`](https://github.com/patrickkabwe/grx/blob/main/CHANGELOG.md)
> at the repository root. Edit that file, not this one.


All notable changes to `grx` are documented in this file.

The format is based on [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- GraphQL HTTP hardening: `OPTIONS /graphql` with `Allow`, per-request
  `RequestTimeout`, optional `DisableIntrospection`, `MaxHTTPRequestBytes` and
  response gzip on the default transport, `GET` rejection of mutation and
  subscription operations (`405`), `POST` rejection of subscriptions (`405`),
  panic-safe HTTP execution, and `RequestID` middleware with
  `core.RequestIDFromContext`.
- `grx` options: `WithRequestTimeout`, `WithDisableIntrospection`,
  `WithMaxHTTPRequestBytes`, `WithResponseGzip`, and `RequestID`.
- GitHub Actions workflow `ci` running `go test -race ./...`.
- Public documentation site built with [VitePress](https://vitepress.dev/)
  and published to GitHub Pages from `docs/`.
- API reference auto-generated from Go doc comments via
  [`gomarkdoc`](https://github.com/princjef/gomarkdoc) (`make docs-api`).
- `Makefile` with targets for build, test, and the full docs lifecycle.
- Top-level `grx` package: `grx.NewServer(opts...)` (options-only) with
  `grx.WithSchema`, `grx.WithPlugins`, `grx.WithPlaygroundPath`,
  `grx.WithGraphQLPath`, `grx.WithSubscriptionPath`, and `grx.WithTransports`,
  forwarding to [`server.Config`](https://pkg.go.dev/github.com/patrickkabwe/grx/server#Config).
- Optional split URL: concrete `*websocket.Transport` and `*sse.Transport`
  values can be routed to `SubscriptionPath` when it differs from
  `GraphQLPath` (`POST /graphql` remains queries/mutations; empty subscription
  path preserves the conventional single-path setup).

### Changed

#### Breaking API

- `grx.NewServer` no longer takes a positional schema argument — use
  `grx.WithSchema(schema.Config)` in the option list.

### Server &amp; Transports

- HTTP server (`grx.NewServer`, backed by `server.New`) with
  `POST /graphql` JSON handler and an embedded GraphiQL playground.
- `core.Transport` interface for pluggable protocol handlers, registered
  in order via `grx.WithTransports(...)` (or `server.Config.Transports`).
- WebSocket transport (`pkg/websocket`) implementing RFC 6455 framing
  and the `graphql-transport-ws` subprotocol — the modern protocol used
  by the `graphql-ws` library and Apollo Client v3.5+. The legacy
  `subscriptions-transport-ws` protocol is intentionally not supported.
- Server-Sent Events transport (`pkg/sse`) for one-way subscription
  streams over `text/event-stream`.
- Optional pub/sub primitive (`pkg/pubsub`) for cross-resolver fan-out:
  a small `PubSub` interface, an in-process `Memory` backend, server-side
  `Filter` composition (`All`, `Any`, `TopicEquals`, `TopicHasPrefix`,
  `PayloadFunc`), and a generic `Typed[T]` wrapper with a pluggable
  `Codec` (defaults to JSON) so resolver code can `Publish`/`Subscribe`
  with strongly typed values and predicates.
- Redis-backed pub/sub backend (`pkg/pubsub/redis`) packaged as a
  separate Go module so the root `grx` module remains dependency-free.

### Schema &amp; Execution

- Code-first schema builder (`schema.Build`) that reflects user Go
  structs into runtime metadata. Reflection runs once at startup; the
  per-request hot path uses precomputed indices.
- Lexer, parser, AST, and executor (`exec`) for queries, mutations, and
  subscriptions covering the subset listed in the
  [Roadmap](https://patrickkabwe.github.io/grx/roadmap/).
- Introspection fast-path (`__schema`, `__type`) sufficient to load the
  bundled GraphiQL UI.

### Plugins

- `plugin.Plugin` lifecycle interface with hooks for `RequestStart`,
  `ParsingStart`, `ValidationStart`, `ExecutionStart`,
  `FieldResolveStart`, `ResponseSend`, and `Error`.
- `plugin.Base` no-op embedding helper for partial implementations.
- `plugin/logger` structured-logging plugin built on `log/slog`.

### Benchmarks

- Comparative micro-benchmarks against `graphql-go/graphql` and
  `graph-gophers/graphql-go` in a sibling `benchmark/` Go module so the
  comparison libraries never enter the main `go.mod`.
- `TestImplementationsAgree` sanity test that asserts every library
  returns identical, error-free responses for the benchmark queries.

[Unreleased]: https://github.com/patrickkabwe/grx/commits/main
