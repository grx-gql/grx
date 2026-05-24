# Changelog

All notable changes to `grx` are documented in this file.

The format is based on [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Published versions use section titles that match
[release-please](https://github.com/googleapis/release-please) (emoji-prefixed
headings such as **`### ✨ Added`**, **`### 🐛 Fixed`**, **`### 📚 Documentation`**, … —
see **`release-please-config.json`**). This file **`CHANGELOG.md`** is the changelog; browse it **[on GitHub](https://github.com/grx-gql/grx/blob/main/CHANGELOG.md)** (the docs site no longer duplicates it).

## [0.4.0] - unpublished

### 💥 Breaking Changes

- **Import paths**: Transports and helpers previously under **`pkg/`** now resolve at the repository root—for example **`github.com/grx-gql/grx/http`**, **`…/sse`**, **`…/websocket`**, **`…/client`**, **`…/cors`**, **`…/memory-pubsub`** (`package pubsub`), and nested **`github.com/grx-gql/grx/redis-pubsub`**. Releases of the Redis submodule are tagged **`redis-pubsub/v*`** instead of **`pkg/pubsub/redis/v*`**.
- `sse.New` now accepts optional configuration: use `sse.New()` for the
  previous zero-value transport, or `sse.New(sse.Config{...})` when limits are
  needed.
- `grx.NewServer` no longer takes a positional schema argument — use
  `grx.WithSchema(schema.Config)` in the option list.

### ✨ Added

- `plugins/logger`: `graphql.response.send` and `graphql.error` include
  `graphql.response.time` (wall-clock elapsed since this plugin's `RequestStart`,
  formatted with `ms` or `s` suffix) when hooks run on the derived request context
  produced by the executor.
- `plugins.Plugin` lifecycle hooks (`RequestStart`, `ParsingStart`,
  `ValidationStart`, `ExecutionStart`, `FieldResolveStart`, `ResponseSend`,
  `Error`) with `plugins.Base` partial-implementation helper.
- Structured `plugins/logger` on `log/slog`.
- Executor and server limits for abusive documents:
  `exec.WithMaxSelectionCount`, `exec.WithMaxAliasCount`,
  `exec.WithMaxRootFieldCount`, matching `server.Config` (`MaxSelectionCount`,
  `MaxAliasCount`, `MaxRootFieldCount`); zero disables each limit.
- Optional parse cache for variable-free queries: `exec.WithDocumentCache(limit)` /
  `server.Config.DocumentCacheSize` (LRU by count); requests with variables bypass
  the cache because defaults are applied during parsing.
- Lexer LRU (`exec.WithLexerCache` / `server.Config.LexerCacheSize`) sharing token
  streams keyed by normalized query text.
- SSE `sse.Config.MaxActiveSubscriptions` limiting concurrent streams;
  over-limit replies return `429` with a GraphQL-shaped JSON body.
- WebSocket `websocket.Config.MaxSubscriptions` enforcing a per-connection cap.
- GraphQL-over-HTTP hardening: `OPTIONS /graphql` with `Allow`, per-request
  `RequestTimeout`, optional introspection toggle, gzip responses, sane request
  size caps, rejecting mutations/subscriptions on `GET`, rejecting subscriptions on
  `POST`, panic-safe handlers, plus `core.RequestID` plumbing.
- `WithRequestTimeout`, `WithDisableIntrospection`, `WithMaxHTTPRequestBytes`,
  `WithResponseGzip`, `WithPersistedQueries`, `WithSchemaSDLPath`, and related
  `RequestID` entry points on **`grx` options**.
- `grx.NewServer(opts…)` composing `schema.Config`, plugins, transports, GraphQL and
  subscription paths, forwarding to **`server.Config`**.
- Separate subscription URL routing when `SubscriptionPath` ≠ `GraphQLPath`.
- Field aliases; `@skip` / `@include`; named fragment spreads & inline fragments;
  `exec.WithMaxSelectionDepth`; stricter non-null completion during execution.
- `schema.PrintSDL` and optional `SchemaSDLPath` HTTP export of the compiled graph.
- HTTP JSON arrays for Apollo-style batches; persisted queries via persisted-query
  maps on `server`/`http` configs and `extensions.persistedQuery.sha256Hash`;
  wire `extensions` surfaced as `core.GraphQLBody.Extensions`.
- Parser-backed variable bookkeeping; deferred undefined/unused-variable errors to the
  validation phase; validation requires declared variables to be defined and used;
  merge-compatible sibling selections cannot disagree on backing field metadata.
- Built-in executable directives validated for required / typed arguments
  (`@skip` / `@include` `Boolean! if`, provisional `@defer` / `@stream` checks).
- `schema.Field.ArgsByName` for O(1) argument lookups from `schema.Build`.
- **`memory-pubsub`** façade (package **`pubsub`**) plus in-process `Memory` backend, predicates, and the
  `Typed[T]` helper (JSON codecs by default).
- Redis backend shipped as **`github.com/grx-gql/grx/redis-pubsub`**.
- RFC 6455 websocket transport exposing `graphql-transport-ws`; SSE subscriptions over
  `text/event-stream`; bundled GraphiQL + `POST /graphql` playground server.
- `core.Transport` registries registered through `server.Config`/option helpers.
- Code-first schemas via `schema.Build`, lexing/parsing/exec stack, roadmap-linked
  feature depth, introspection shortcuts for tooling.
- Comparative benchmarks & `TestImplementationsAgree` guarded inside the sibling
  `benchmark/` module.

### 📚 Documentation

- VitePress manual + GitHub Pages pipeline from `docs/`.
- Markdown API reference scaffolding via [`gomarkdoc`](https://github.com/princjef/gomarkdoc).

### 🧹 Chores

- Makefile targets covering build/tests/docs ergonomics plus release automation hooks.
- GitHub Actions **`ci`** (tests/coverage); **`Docs`** builds the VitePress site; **`Release`** runs release‑please and tag workflows.

[0.4.0]: https://github.com/grx-gql/grx/compare/v0.3.0...HEAD
