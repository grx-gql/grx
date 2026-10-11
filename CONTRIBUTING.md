# Contributing to grx

Thanks for helping. This guide covers setting up the repository, the rules the codebase holds itself to, and how changes are reviewed and released.

## Before you start

- **Bugs** — open an issue using the bug report form, with the smallest schema and query that reproduce it.
- **Features** — check [ROADMAP.md](./ROADMAP.md) first. For anything larger than a small fix, open a feature request before writing code so the approach can be agreed.
- **Performance** — use the performance form and include benchmark numbers.

Keep each pull request small and about one thing. Don't reformat or refactor code your change doesn't need, and don't revert unrelated changes.

## Setup

You need Go 1.25 or newer (see `go.mod`), and [Bun](https://bun.sh) only if you work on the documentation site.

```bash
git clone https://github.com/grx-gql/grx.git
cd grx
make dev-setup      # go mod tidy in each module, then installs the git hooks
```

`make install-hooks` alone installs just the commit-message hook.

## Build and test

```bash
make build
make test           # root module and the pubsub/redis submodule
make test-race      # required for anything touching the executor, plugins or transports
make vet
make fmt            # gofmt -w the tree
```

Run an example server to try a change by hand:

```bash
go run ./examples/basic     # playground on http://localhost:4000/
```

Every pull request must pass `make test`, `make vet` and `gofmt`. Run `make test-race` before submitting concurrency-sensitive changes.

## Repository layout

| Path | Purpose |
| --- | --- |
| `core/` | Transport-agnostic types shared by everything: `Request`, `Response`, `Error`, and the `Executor` and `Transport` interfaces. |
| `schema/` | Builds schema metadata from Go types (the root `Query`, `Mutation` and `Subscription`, inputs and outputs). |
| `exec/` | Lexer, parser, AST, validation, executor and introspection — the hot path. |
| `server/` | The `http.Handler`: playground, routing, and dispatch to the configured transports. |
| `http/`, `sse/`, `websocket/` | `core.Transport` implementations: GraphQL over HTTP+JSON, Server-Sent Events, and `graphql-transport-ws`. |
| `client/` | A GraphQL-over-HTTP client, useful in services and tests. |
| `plugins/` | The plugin lifecycle interface and built-in plugins. |
| `pubsub/` | The backend-neutral `PubSub` interface and message contract. |
| `pubsub/memory/` | The process-local `PubSub` backend for subscriptions. |
| `pubsub/redis/` | A Redis `PubSub`, in its own Go module so Redis never enters the root `go.mod`. |
| `examples/` | Runnable examples. `examples/basic` is the reference shape for resolvers and wiring. |
| `docs/` | The VitePress documentation site published at https://grx-gql.github.io/grx/. |

## Design rules

### Layering

- `core` imports nothing from `schema`, `exec`, `server`, or `plugins`. Everything else depends on `core`, never the reverse. If two packages need a type, it belongs in `core`.
- Every network protocol, HTTP+JSON included, is a `core.Transport` in its own top-level folder. Transports talk only to `core.Executor`; GraphQL execution never happens inside one.
- `server` owns the playground, `/favicon.ico` and routing, and never parses a GraphQL payload itself. It consults `Config.Transports` in order and appends the default `http` transport last, so `POST /graphql` always has a handler.
- `exec` stays transport-agnostic.
- Don't add a top-level package without a clearly distinct responsibility; extend an existing one first.

### Performance

- Keep the execution path allocation-aware. Prefer a precomputed slice or struct to a per-field map.
- Reflection belongs in `schema`, at build time. If `exec` needs type information, expose precomputed metadata from `schema` rather than reflecting per request.
- Parse and validate once where possible, and avoid global mutable state; wire dependencies through config.
- Benchmark broad execution changes (`benchmark/`, and [the benchmarks page](https://grx-gql.github.io/grx/benchmarks)) before claiming an improvement.

### Dependencies

The root module has a single third-party dependency (`github.com/andybalholm/brotli`, for response compression), and keeping it that way is deliberate. A new dependency needs a strong reason tied to a roadmap item and the maintainer's agreement; check its impact with `go mod why`.

### Public API

- Changing the meaning of a `server.Config` field or a `grx` option means updating `examples/` and the tests that construct it in the same change.
- New resolver shapes need `schema/builder.go` and `examples/basic` updated together, so the example keeps demonstrating the feature.
- Errors surfaced in a response are `core.Error`, with `Path` set for field errors. Field errors keep partial `data`. Don't leak Go internals into error messages.
- Plugins embed `plugins.Base` to stay compatible with new hooks, keep per-request state in `context.Context` rather than on the plugin, and must be safe for concurrent use. Only `RequestStart` may return a new context.
- The GraphiQL playground is an inlined HTML template on purpose; don't turn it into separate assets.

### Tests

- Put tests next to the code, in the same package. Prefer table-driven tests, and extend an existing test file when it already covers the area.
- Server behaviour is tested over HTTP with `httptest.NewServer` against a real server — see `server/query_test.go`.
- Avoid time-sensitive tests; inject a clock through config if you need one.

## Commits and pull requests

Commit messages follow [Conventional Commits](https://www.conventionalcommits.org/) — `feat:`, `fix:`, `docs:`, `chore:`, `perf:`, `refactor:`, `test:` — and the commit-message hook checks it. Pull requests are squash-merged, so the PR title becomes the commit on `main` and must follow the same format; it decides the next version.

When a change makes a roadmap item fully supported, tested and reachable from the public API, update [ROADMAP.md](./ROADMAP.md) in the same pull request. Partial support stays open.

## Releases

Releases are tagged Go modules; there are no binaries. [release-please](https://github.com/googleapis/release-please) runs on `main` (`.github/workflows/release.yml`) and keeps a release pull request open that updates [CHANGELOG.md](./CHANGELOG.md) and the version. Merging it tags `v*` and publishes the module.

The `pubsub/redis` module is not managed by release-please. Tag it by hand as `pubsub/redis/vX.Y.Z`; the same workflow warms `proxy.golang.org` for those tags.

## License

By contributing, you agree that your contributions are licensed under the [MIT License](./LICENSE).
