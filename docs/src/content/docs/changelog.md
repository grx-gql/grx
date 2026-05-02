---
title: Changelog
description: All notable changes to grx, in reverse chronological order.
sidebar:
  order: 5
editUrl: https://github.com/patrickkabwe/grx/edit/main/CHANGELOG.md
tableOfContents:
  minHeadingLevel: 2
  maxHeadingLevel: 3
---

> This page is mirrored from
> [`CHANGELOG.md`](https://github.com/patrickkabwe/grx/blob/main/CHANGELOG.md)
> at the repository root. Edit that file, not this one.


All notable changes to `grx` are documented in this file.

The format is based on [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Public documentation site built with [Astro Starlight](https://starlight.astro.build/)
  and published to GitHub Pages from `docs/`.
- API reference auto-generated from Go doc comments via
  [`gomarkdoc`](https://github.com/princjef/gomarkdoc) (`make docs-api`).
- `Makefile` with targets for build, test, and the full docs lifecycle.

### Server &amp; Transports

- HTTP server (`server.New`) with `POST /graphql` JSON handler and an
  embedded GraphiQL playground.
- `core.Transport` interface for pluggable protocol handlers, registered
  in order via `server.Config.Transports`.
- WebSocket transport (`pkg/websocket`) implementing RFC 6455 framing
  and the `graphql-transport-ws` subprotocol — the modern protocol used
  by the `graphql-ws` library and Apollo Client v3.5+. The legacy
  `subscriptions-transport-ws` protocol is intentionally not supported.
- Server-Sent Events transport (`pkg/sse`) for one-way subscription
  streams over `text/event-stream`.
- Pub/Sub primitive (`pkg/pubsub`) for cross-resolver fan-out: a small
  `PubSub` interface, an in-process `Memory` backend, server-side
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
