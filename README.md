# grx

`grx` is a fast, dependency-free GraphQL server and runtime for Go. The project is focused on predictable execution, clear extension points, and production-oriented transport support without unnecessary abstraction.

## Quick Start

Requirements:

- Go `1.25+` (see root `go.mod`)
- Bun for building the [VitePress](https://vitepress.dev/) documentation site

Run the basic example server:

```bash
go run ./examples/basic
```

The example starts a server on `http://localhost:4000` and serves the GraphQL playground at `http://localhost:4000/`.

Run the subscription example when you want pub/sub, WebSocket, and SSE wiring:

```bash
go run ./examples/subscriptions
```

For Bearer authentication, context-scoped identity, and a field authorizer, run:

```bash
go run ./examples/auth
```

Useful local commands:

```bash
make dev-setup      # go mod tidy + install-hooks (good right after clone)
# or
make mod-tidy && make install-hooks
make build
make test
make vet
make fmt
```

## Documentation

Full documentation is available at:

- https://grx-gql.github.io/grx/

### AI-assisted development

- **`AGENT.md`** at the repo root is the operational guide for **agents working inside this repository**.
- **`AGENTS.md`** is a stub for tools that look for **`AGENTS.md`**; it points **`AGENT.md`** vs **`.cursor/skills/graphql-grx/`** for **app developers** importing **`grx`**.
- **`/.cursor/skills/graphql-grx/SKILL.md`** is a reusable **Cursor skill** describing public API patterns (`schema.Config`, **`grx.NewServer`**, transports, subscriptions, hardening)—copy it to **`~/.cursor/skills/graphql-grx`** in your Go project.

See **[AI assistants](https://grx-gql.github.io/grx/guides/ai-assistants)** on the docs site for setup steps.

## Contributing

Contributions should stay small, focused, and directly related to the problem being solved.

**Releases**: use **[Conventional Commits](https://www.conventionalcommits.org/)** in the squash-merge subject (`feat:`, `fix:`, `chore:`, `docs:`, …) so **release-please** can propose the next semver and changelog. See **`RELEASING.md`**. After cloning, **`make dev-setup`** runs **`go mod tidy`** in each module and then enables hooks; use **`make install-hooks`** alone if you only need the Conventional Commit check.

Contribution rules for this repository:

- Keep changes minimal.
- Prefer existing logic over introducing new patterns.
- Preserve performance characteristics and use the fast path where possible.
- Use strict typing and simple single-purpose functions.
- Do not revert unrelated changes.
- Run relevant validation before opening a change, at minimum `make test` when code changes affect runtime behavior.

## Releases

Tagged **[Go module](https://go.dev/doc/modules/publishing)** releases (no binaries). Maintainer steps and tag conventions: **[`RELEASING.md`](./RELEASING.md)**.

## License

This repository does not currently include a root `LICENSE` file. Until a license is added, the usage terms are not explicitly defined in the repository.

## Project Status

`grx` is active and usable for a subset of GraphQL workloads, but it is not yet at full production-grade GraphQL feature parity.

Current status:

- Core query, mutation, and subscription flows are implemented.
- HTTP, SSE, and WebSocket transport support exists.
- GraphQL responses now include field and request error classification, source locations, and stable response field ordering.
- The roadmap still includes missing parity items such as broader GraphQL-over-HTTP semantics, incremental delivery execution, and additional language and execution features.

For detailed status and planned work, see:

- [ROADMAP.md](./ROADMAP.md)
- https://grx-gql.github.io/grx/roadmap/
