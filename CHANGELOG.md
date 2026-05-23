# Changelog

All notable changes to `grx` are documented in this file.

The format is based on [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- `plugin/logger`: `graphql.response.send` and `graphql.error` include
  `graphql.response.time` (wall-clock elapsed since this plugin's `RequestStart`,
  formatted with `ms` or `s` suffix) when hooks run on the derived request context
  produced by the executor.

- Executor and server limits for abusive documents: `exec.WithMaxSelectionCount`,
  `exec.WithMaxAliasCount`, `exec.WithMaxRootFieldCount`, plus matching
  `server.Config` fields (`MaxSelectionCount`, `MaxAliasCount`,
  `MaxRootFieldCount`). Zero disables each limit.
- Optional parse cache for variable-free requests: `exec.WithDocumentCache(limit)`
  and `server.Config.DocumentCacheSize` (LRU eviction by count; requests with a
  non-empty variable map bypass the cache because defaults are applied during
  parsing today).
- Lexer LRU: `exec.WithLexerCache(limit)` shares token streams keyed by normalized
  query text so transports that probe `Executor.OperationKind` before `Execute` only
  lex once. `server.Config.LexerCacheSize` sets capacity explicitly; when it stays
  zero but `DocumentCacheSize` is positive, the lexer cache uses the same limit.
- SSE transport tuning: `sse.Config.MaxActiveSubscriptions` limits concurrent
  streams per `*sse.Transport`; `sse.New(sse.Config{...})` is optional — zero
  preserves previous behaviour. Over-limit requests receive `429` with a JSON
  GraphQL error body.
- WebSocket per-connection cap: `websocket.Config.MaxSubscriptions` rejects
  additional `subscribe` operations once the limit is reached (error payload plus
  `complete` for that id).
- GraphQL HTTP hardening: `OPTIONS /graphql` with `Allow`, per-request
  `RequestTimeout`, optional `DisableIntrospection`, `MaxHTTPRequestBytes` and
  response gzip on the default transport, `GET` rejection of mutation and
  subscription operations (`405`), `POST` rejection of subscriptions (`405`),
  panic-safe HTTP execution, and `RequestID` middleware with
  `core.RequestIDFromContext`.
- `grx` options: `WithRequestTimeout`, `WithDisableIntrospection`,
  `WithMaxHTTPRequestBytes`, `WithResponseGzip`, `WithPersistedQueries`,
  `WithSchemaSDLPath`, and `RequestID`.
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
- Execution: field aliases, `@skip` / `@include`, named fragment spreads and
  inline fragments, parse-time selection depth limits via
  `exec.WithMaxSelectionDepth`, and stricter non-null completion in the executor.
- `schema.PrintSDL` plus optional `server.Config.SchemaSDLPath` (GET) for a
  minimal SDL export of the built type registry.
- Apollo-style HTTP batching: `POST` with a JSON array of requests returns a
  JSON array of GraphQL responses; automatic persisted queries via
  `server.Config.PersistedQueries` / `pkg/http.Config.PersistedQueries` and
  `extensions.persistedQuery.sha256Hash`.
- `core.GraphQLBody.Extensions` for persisted-query metadata on the wire.

### Changed

#### Breaking API

- `pkg/sse.New` now accepts optional configuration: use `sse.New()` for the
  previous zero-value transport, or `sse.New(sse.Config{...})` when limits are
  needed.
- `grx.NewServer` no longer takes a positional schema argument — use
  `grx.WithSchema(schema.Config)` in the option list.

#### Validation and parsing

- Parser records every variable reference while building values; a missing entry
  in the request variable map no longer fails at parse time — undefined or
  unused variables are rejected during validation instead.
- Validation records variable use sites during parse and enforces that every
  `$name` in the document is declared on the selected operation and that every
  declared variable is used; sibling selections with the same response key must
  not conflict on field name or arguments.
- Built-in executable directives validate required / typed arguments where
  implemented (`@skip` / `@include` `Boolean!` `if`, optional checks for
  `@defer` / `@stream`).
- `schema.Field` gains `ArgsByName` for O(1) argument metadata lookup during
  validation (populated by `schema.Build`).

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
