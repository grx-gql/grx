# grx

`grx` is a fast, dependency-free GraphQL server and runtime for Go. The project is focused on predictable execution, clear extension points, and production-oriented transport support without unnecessary abstraction.

## Quick Start

Requirements:

- Go `1.22+`
- Bun for the docs site only

Run the basic example server:

```bash
go run ./examples/basic
```

The example starts a server on `http://localhost:4000` and serves the GraphQL playground at `http://localhost:4000/`.

Useful local commands:

```bash
make build
make test
make vet
make fmt
```

## Documentation

Full documentation is available at:

- https://patrickkabwe.github.io/grx/

## Contributing

Contributions should stay small, focused, and directly related to the problem being solved.

Contribution rules for this repository:

- Keep changes minimal.
- Prefer existing logic over introducing new patterns.
- Preserve performance characteristics and use the fast path where possible.
- Use strict typing and simple single-purpose functions.
- Do not revert unrelated changes.
- Run relevant validation before opening a change, at minimum `make test` when code changes affect runtime behavior.

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
- https://patrickkabwe.github.io/grx/roadmap/
