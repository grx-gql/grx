# grx benchmarks

Comparative micro-benchmarks for the `grx` GraphQL runtime against the two
most widely-used Go GraphQL packages:

| Library | Stars | Style |
| --- | --- | --- |
| `github.com/patrickkabwe/grx` | — | Code-first, reflected struct methods |
| `github.com/graphql-go/graphql` | 10k+ | Code-first, builder API |
| `github.com/graph-gophers/graphql-go` | 4.7k+ | Schema-first (SDL) + resolver methods |

## Why a separate Go module?

`grx` is intentionally dependency-free (see `AGENT.md` and `README.md`).
Benchmarks live in a sibling Go module so the comparison libraries above are
**never** pulled into the main `go.mod`. A `replace` directive points the
benchmark module at the working tree.

## Running

`benchmark/` is a **separate Go module**, so `./benchmark/...` from the
repo root will not work (`directory prefix benchmark does not contain main
module or its selected dependencies`). Run from inside the module, or use
`go`'s `-C` flag to switch module without changing shells:

```bash
# Option A: cd into the module
cd benchmark
go test -bench=. -benchmem ./...

# Option B: from the repo root, use -C
go test -C benchmark -bench=. -benchmem ./...
```

More invocations (run from inside `benchmark/`, or prefix each with
`-C benchmark`):

```bash
# A single scenario across all libraries
go test -bench=BenchmarkNestedQuery -benchmem ./...

# Filter by library
go test -bench=BenchmarkSimpleQuery/grx -benchmem ./...

# Longer runs for tighter numbers
go test -bench=. -benchmem -benchtime=3s -count=5 ./...
```

The first run will fetch `graphql-go/graphql` and `graph-gophers/graphql-go`.

## Scenarios

| Benchmark | Operation | Tests |
| --- | --- | --- |
| `BenchmarkSimpleQuery` | `user(id: "user_1") { id name email }` | Single object, scalar fields only |
| `BenchmarkNestedQuery` | `post(...) { id title body author { id name email } }` | Nested object resolution |
| `BenchmarkListQuery` | `users(count: 50) { id name email }` | List resolution and per-item completion |

Every iteration runs `parse → validate → execute → JSON-encode` so the
result includes serialization overhead. Schemas are constructed once outside
the timed region; only request-time work is measured.

## Sanity test

```bash
cd benchmark && go test ./...
# or
go test -C benchmark ./...
```

`TestImplementationsAgree` verifies all three libraries return non-empty,
error-free responses for each query before any benchmarking happens.

## Fairness notes

- Resolvers in every implementation pull from the same `fixture.go` data so
  the comparison measures library overhead, not data-source cost.
- `graph-gophers/graphql-go` is configured with `MaxParallelism(1)` to keep
  the executor single-threaded, matching the synchronous execution model of
  the other two and removing goroutine-scheduling noise from the numbers.
- Each library uses its idiomatic resolver style; we deliberately do not
  contort one library to mimic another.
