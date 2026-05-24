# AGENT.md

Operational guide for **AI coding agents** working **in this `grx` repository** (the library source). Read it in full before making changes here.

> **Consumers of the library:** If someone builds an application with **`go get github.com/grx-gql/grx`**, point them at the reusable Cursor skill at **`.cursor/skills/graphql-grx/`** (copy or symlink the folder into **`~/.cursor/skills/graphql-grx`**). That skill summarizes **public `grx` API** patterns; **AGENT.md** is for contributors changing **this** repository only.

The authoritative feature scope and roadmap live in **`README.md`** and **`ROADMAP.md`**; **this file** describes **how** to change the codebase without breaking layering or performance budgets.

## Project Summary

`grx` is a Go GraphQL server/runtime targeting [GraphQL October 2021](https://spec.graphql.org/October2021/) parity with predictable execution and `net/http` integration.

- **Language:** Go **`1.25+`** (see root **`go.mod`**).
- **Module path:** `github.com/grx-gql/grx`
- **Dependencies:** deliberately small surface; **`go.sum`** reflects what is shipped—avoid new external packages unless there is explicit maintainer consensus and parity justification.

## Repository Layout

| Path | Purpose |
| --- | --- |
| `core/` | Shared transport-agnostic types: `Request`, `Response`, `Error`, the `Executor` interface, the `Transport` interface, and HTTP helpers (`GraphQLBody`, `DecodeGraphQLBody`, `WriteJSON`, `HeaderContains`). Anything imported by both `server` and `exec` belongs here. |
| `http/` | `core.Transport` implementation of the canonical GraphQL-over-HTTP+JSON `POST /graphql` wire format. The package name is `http`; consumers that also import `net/http` alias it (for example `grxhttp`). |
| `sse/` | `core.Transport` implementation of the GraphQL over Server-Sent Events protocol. |
| `websocket/` | RFC 6455 WebSocket framing (`conn.go`) plus a `core.Transport` implementation of the `graphql-transport-ws` subprotocol (`dispatcher.go`). Production knobs (init timeout, read/write deadlines, max message size, origin allowlist, `OnConnect` auth hook, server-side ping interval) live on `websocket.Config`. The legacy Apollo `subscriptions-transport-ws` (`graphql-ws`) subprotocol is intentionally not supported. |
| `client/` | Outbound GraphQL-over-HTTP client (`POST` JSON) backed by `net/http`; useful from services and `httptest` integration tests. |
| `cors/` | Lightweight CORS `Middleware` helpers used by docs/examples. |
| `memory-pubsub/` | `pubsub.PubSub` interface and in-memory backend used by subscription resolvers for cross-resolver fan-out. Imported as **`github.com/grx-gql/grx/memory-pubsub`** (`package pubsub`). |
| `redis-pubsub/` | Nested Go module (**`github.com/grx-gql/grx/redis-pubsub`**) providing a Redis-backed `PubSub` without pulling Redis into the root `go.mod`. |
| `schema/` | Schema builder. Reflects user Go types (root `Query`, `Mutation`, input structs, output structs) into the internal type metadata used by the executor. |
| `exec/` | GraphQL lexer, parser, AST, executor, and introspection fast-path. The hot execution path lives here. |
| `server/` | HTTP entry point, GraphiQL playground, transport dispatch (`Config.Transports`). Implements `http.Handler`. The default HTTP+JSON transport from `http` is appended automatically to the chain so plain `POST /graphql` keeps working out of the box. |
| `plugins/` | Plugin lifecycle interface (`RequestStart`, `ParsingStart`, `ValidationStart`, `ExecutionStart`, `FieldResolveStart`, `ResponseSend`, `Error`) and built-in plugins like `Logger`. Embed `plugins.Base` for partial implementations. |
| `middlewares/` | HTTP middleware helpers such as request ID propagation. Middleware runs before transports decode GraphQL requests. |
| `examples/basic/` | End-to-end example wiring resolvers, schema, plugin, transports, and server. Mirror this structure when adding new examples. |

Human-facing documentation: **`https://grx-gql.github.io/grx/`**

## Build, Run, Test

Run from the repo root. Preferred shortcuts:

```bash
make build
make test           # root module + redis-pubsub submodule
make test-race      # before risky concurrency changes
make vet
make fmt            # gofmt -w repository tree
```

Run the example server:

```bash
go run ./examples/basic
# GraphiQL: http://localhost:4000/
# GraphQL endpoint: http://localhost:4000/graphql
```

Use **`go vet`** and **`gofmt`** (**`make fmt`**) on touched Go files before finishing.

```bash
go build ./...
go test ./...
go test -race ./...
```

## Architectural Conventions

- **Single module, internal packages by responsibility.** Do not introduce a new top-level package without a clear, distinct responsibility. Prefer extending `core`, `schema`, `exec`, `server`, `plugins`, or `middlewares`, or the small sibling transport/pubsub folders at the repo root.
- **`core` has no upward imports.** It must not import `schema`, `exec`, `server`, `plugins`, or `middlewares`. Other packages depend on `core`, never the reverse. Transports (`http`, `sse`, `websocket`) may import `core` but follow the same upward-import ban.
- **Every network protocol — including HTTP+JSON — implements `core.Transport`** in its own top-level folder (for example `http/`, `sse/`, `websocket/`). `server` wires transports via `Config.Transports` and appends the default `http.Transport` chain at the end. `server` itself only owns playground, favicon, and request routing; it never parses GraphQL payloads inline.
- **`server` depends on `core`, `http`, `exec`, `schema`, `plugins`** (and pulls in `websocket` / `sse` only when examples or callers register those transports). Keep GraphQL execution out of transports; transports talk to `core.Executor` only.
- **`exec` owns parsing, validation, execution, introspection.** It must remain transport-agnostic.
- **`schema` is the only package allowed to use reflection-heavy schema introspection** for building metadata. Keep reflection out of the per-request hot path; precompute and cache in `schema` so `exec` can stay allocation-aware.
- **Plugins receive `context.Context` and may return a derived context from `RequestStart`.** Use `plugins.Base` to provide no-op defaults when implementing only a subset.

## Resolver and Schema Patterns

Follow the `examples/basic` shape:

- The schema wiring type is **`schema.Config`** **`{ Query, Mutation, Subscription }`** passed through **`grx.WithSchema`** (or **`server.Config.Schema`**). **`Query`** is required; **`Mutation`** and **`Subscription`** are optional.
- `Query`, `Mutation`, and `Subscription` are plain user-defined structs. Their exported methods are the GraphQL fields on the corresponding root type. Method names are lowercased for the GraphQL field name (`User` → `user`).
- Resolver method signatures are `func(ctx context.Context, args TArgs) (*TResult, error)` for queries and mutations, and `func(ctx context.Context, args TArgs) (<-chan *TResult, error)` for subscriptions. `ctx` and `args` are both optional in that order; omit either when the resolver does not need it.
- Argument structs are plain Go structs; field names map to GraphQL argument names. Use the `gql:"name,nonNull"` tag to override the field name or mark non-null.
- Output object types are plain Go structs; nested objects work the same way.
- Do not introduce a separate `Resolvers` middleman struct; the methods on the root type *are* the resolvers.

### Scaling to multiple entities

`schema/builder.go` enumerates root methods via `reflect.Type.NumMethod()`, which includes promoted methods from embedded fields. Use this to keep one file per entity:

- For each entity `X`, define `XQuery`, `XMutation`, and (if needed) `XSubscription` structs with the entity's resolver methods. Co-locate them with the entity's data and input types in `x.go`.
- `Query`, `Mutation`, and `Subscription` in `schema.go` embed those per-entity structs. Adding a new entity is one new file plus one embedded field per applicable root.
- Per-entity dependencies (services, loaders, repositories) live as fields on the per-entity resolver struct, not on the root type. This keeps the root types pure composition.
- Method names across embedded structs must not collide on the same root; reflection silently drops ambiguous methods. Treat collisions as a design smell and rename one side.

When introducing new GraphQL features that need new resolver shapes, update both `schema/builder.go` and `examples/basic` together so the example continues to compile and demonstrate the feature.

## Adding or Modifying Features

1. Find the matching item in the `README.md` checklist. If it is unchecked, you are implementing it; check it off in the same change once it is genuinely supported and tested.
2. If the feature does not appear in `README.md`, either it does not belong (push back) or the checklist needs a new entry. Add the entry first.
3. Touch the smallest set of packages possible. Cross-package changes should be motivated by the layering rules above, not convenience.
4. Add tests next to the code (`*_test.go` in the same package). For server-level behavior, add HTTP-level tests under `server/` similar to the existing `query_test.go`, `mutation_test.go`, `typename_test.go`, `subscription_test.go`.
5. Run `go test -race ./...` before declaring done.

## Performance Rules (non-negotiable)

These are restated from `README.md` because they constrain implementation choices:

- Keep hot execution paths allocation-aware. Avoid per-field map allocation where a precomputed slice or struct would do.
- Prefer precomputed schema metadata over repeated reflection in `exec`.
- Parse and validate once where possible; design APIs so prepared operations can be cached later even if caching is not implemented yet.
- Avoid global mutable state. Wire dependencies through `Config` structs.
- Keep resolver invocation predictable and type-safe. Reflection at resolve time should be minimal and bounded.
- Benchmark broad execution changes before claiming production readiness.

## Error and Response Conventions

- Use `core.Error` for any error surfaced in a GraphQL response. Populate `Path` for field execution errors.
- Request-level failures (invalid JSON, missing query) are emitted with no `data` field and an HTTP 4xx status. See [`http.Transport.Serve`](https://pkg.go.dev/github.com/grx-gql/grx/http#Transport.Serve).
- Field-level errors must allow partial `data`. Do not abort the whole response for a single resolver error unless non-null bubbling requires it (non-null bubbling is currently unimplemented; see the checklist).
- Do not leak Go internals into `Error.Message`. Wrap or sanitize where appropriate; full error masking is on the security checklist but baseline hygiene applies now.

## HTTP Transport Conventions

- The handler is a plain `http.Handler` returned by `grx.NewServer` or `server.New`. Do not add framework dependencies.
- The GraphiQL playground is served from CDN-hosted assets via an inlined HTML template. Keep it self-contained; no asset bundling.
- `/favicon.ico` must continue to return 204 to avoid noisy logs.
- New transports (SSE, WebSocket, custom HTTP variants) implement **`core.Transport`** beside the existing transports (not inside **`core/`**). Wire them in by appending to **`server.Config.Transports`**; the server consults transports **in order** and appends the built-in **`http`** transport **last**, so **`POST /graphql`** always has a handler. To override default HTTP behaviour, register your own **`POST`**-matching transport **before** the others.
- Default behavior must remain backward compatible: with no `Transports` configured, the server still answers `POST /graphql`, the playground, and `/favicon.ico` — that `POST` is routed through the auto-appended **`http`** transport rather than bespoke handler logic in `server`.

## Plugin Conventions

- Always embed `plugins.Base` in new plugins to remain forward-compatible with new lifecycle hooks.
- `RequestStart` is the only hook that may return a new `context.Context`. Other hooks return only `error`.
- Plugins must be safe for concurrent use across requests. Per-request state goes in `context.Context` via typed keys, not on the plugin struct.
- Errors returned from lifecycle hooks should short-circuit the request; ensure new hooks document this contract.

## Testing Conventions

- Table-driven tests where there are multiple inputs/expected outputs.
- HTTP tests use `httptest.NewServer` against a real `server.Server`. See `server/query_test.go` for the canonical pattern.
- Parser/executor unit tests live in `exec/` next to the code under test.
- Avoid time-sensitive tests; if needed, inject a clock through config.
- Race-free: anything touching the executor or plugins must pass `go test -race ./...`.

## Things Agents Frequently Get Wrong Here

- Do not import `exec` from `core` or `schema` to "share types" — define the shared type in `core` instead.
- Do not add reflection to `exec`. If you need type information at execution time, expose precomputed metadata from `schema`.
- Do not turn the GraphiQL HTML into a separate file or asset pipeline; it is intentionally inlined.
- Do not change `server.Config` field semantics without updating `examples/basic` and any tests that construct it.
- Do not mark a `README.md` checklist item complete unless it is fully implemented, tested, and reachable from the public API. Partial support stays unchecked.
- Avoid introducing third-party modules; keep the dependency graph as thin as **`go.mod`** already allows (`go mod why` helps review impact). Larger additions require maintainer-visible justification tied to roadmap items.

## GitHub (issues and CI)

- Issue forms live under `.github/ISSUE_TEMPLATE/`. **`make validate-issue-templates`** checks their YAML locally (Ruby stdlib).
- High-level parity gaps are tracked as section issues created from `ROADMAP.md` using `python3 scripts/gh_roadmap_tracking_issues.py` (dry-run) or `... --apply` with the GitHub CLI authenticated. The wrapper `scripts/gh-roadmap-tracking-issues.sh` forwards the same flags.

## When in Doubt

- Consult `README.md` for scope and the `Implementation Plan` ordering. The plan is intentionally phased: lexer/parser → AST → validation → execution correctness → type system → introspection → HTTP → subscriptions/incremental → data loading → observability/security → hardening. Avoid leapfrogging phases for unchecked items unless the user explicitly requests it.
- Match existing style in the package you are editing rather than introducing new patterns.
- Prefer extending tests over adding new test files when an existing file already covers the area.
