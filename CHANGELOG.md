# Changelog

All notable changes to `grx` are documented in this file.

The format is based on [Keep a Changelog 1.1.0](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

Published versions use section titles that match
[release-please](https://github.com/googleapis/release-please) (emoji-prefixed
headings such as **`### ✨ Added`**, **`### 🐛 Fixed`**, **`### 📚 Documentation`**, … —
see **`release-please-config.json`**). This file **`CHANGELOG.md`** is the changelog; browse it **[on GitHub](https://github.com/grx-gql/grx/blob/main/CHANGELOG.md)** (the docs site no longer duplicates it).

## [0.4.1](https://github.com/grx-gql/grx/compare/v0.4.0...v0.4.1) (2026-05-24)


### 🧹 Chores

* **ci:** drop tag-verify job from Release workflow ([c379db7](https://github.com/grx-gql/grx/commit/c379db78fdb111370d1d7890c6a5d12b97897420))
* **ci:** drop tag-verify job from Release workflow ([e733c7f](https://github.com/grx-gql/grx/commit/e733c7f62806b92447e239bbad075f032eecf857))

## [0.4.0](https://github.com/grx-gql/grx/compare/v0.3.0...v0.4.0) (2026-05-24)


### ⚠ BREAKING CHANGES

* flatten module layout and migrate import paths under grx-gql/grx

### ✨ Added

* add runtime observability options ([21729c3](https://github.com/grx-gql/grx/commit/21729c35221d37f19a54361c304bcb74def95a67))
* add schema coordinate resolution ([89c6f4c](https://github.com/grx-gql/grx/commit/89c6f4ced16f5a0bdeae4089b30cb5770aee0149))
* expand websocket transport controls ([9b19a21](https://github.com/grx-gql/grx/commit/9b19a21237d765ccd17cdfbc4a1962d2d2f5709a))


### 🐛 Fixed

* **ci:** gh workflows ([a87aa63](https://github.com/grx-gql/grx/commit/a87aa63b409662947e9014270a61829a7d7a9dfa))
* **ci:** gh workflows ([73b5701](https://github.com/grx-gql/grx/commit/73b5701be234fecdc9bfc6077682bb9cfe9ee6ed))
* complete graphql validation parity ([61cbcf2](https://github.com/grx-gql/grx/commit/61cbcf28e6e7befb50c000cb5734529afed6ae4a))
* **docs:** drop dead docs/changelog link from changelog pages ([08944bd](https://github.com/grx-gql/grx/commit/08944bd47a173d69ae9d0a7edafd8ba862f4fa20))
* release workflow ([b8745c7](https://github.com/grx-gql/grx/commit/b8745c7787e9898ee6c91eef510bff59323fb2a2))
* release workflow ([78818bb](https://github.com/grx-gql/grx/commit/78818bb7c8ad8b77e57560b75436fc848d804c1e))
* test coverage ([7c590d8](https://github.com/grx-gql/grx/commit/7c590d890e5229e0bc93df5fed46a8fedb9a8343))
* test coverage ([7e6fea1](https://github.com/grx-gql/grx/commit/7e6fea1f087e753b9ff1a62bce740e52af6798ae))


### 📚 Documentation

* link changelog from GitHub, remove mirrored page ([2641eeb](https://github.com/grx-gql/grx/commit/2641eeb17491efb7c877f3929bb6497d8c249cc1))
* link changelog from GitHub, remove mirrored page ([d89e1da](https://github.com/grx-gql/grx/commit/d89e1da8a4d290dcd4bd79d3ccc4ad2fd979be37))
* sync roadmap parity status ([aef2a37](https://github.com/grx-gql/grx/commit/aef2a37e0c3d822ee5f5913f53415a33e13652f9))


### 🧹 Chores

* add library coverage target ([fbbe1d8](https://github.com/grx-gql/grx/commit/fbbe1d8598817121dd8e93fd4b17d886a76615b4))
* **changelog:** resync docs/changelog.md via sync-changelog script ([ff53f58](https://github.com/grx-gql/grx/commit/ff53f581a1e2b932b31308b7a26f726e5608e93e))
* **ci:** clean up workflow ([b344f0c](https://github.com/grx-gql/grx/commit/b344f0c9d1ac04a509a057cbb9fbff21ea63bdb2))
* **ci:** clean up workflow ([37a2aed](https://github.com/grx-gql/grx/commit/37a2aedd8355da3f166c1bd6f747cb7a31f3cc21))
* **ci:** simplify release workflow ([56d479b](https://github.com/grx-gql/grx/commit/56d479bd79a443a820329001ecd3a6cd501f5b01))
* flatten module layout and migrate import paths under grx-gql/grx ([c1233c1](https://github.com/grx-gql/grx/commit/c1233c1357d39519a43ef2054027dce6d1cb8b17))

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
