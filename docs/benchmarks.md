---
title: Benchmarks
description: Comparative micro-benchmarks for grx vs graphql-go and graph-gophers/graphql-go.
outline: [2, 3]
---

These numbers come from the
[`benchmark/`](https://github.com/patrickkabwe/grx/tree/main/benchmark)
sibling Go module, which exercises the same three queries through `grx`
and the two most widely-used Go GraphQL libraries:

| Library                                  | Style                                       |
| ---------------------------------------- | ------------------------------------------- |
| `github.com/patrickkabwe/grx`            | Code-first, reflected struct methods        |
| `github.com/graphql-go/graphql`          | Code-first, builder API                     |
| `github.com/graph-gophers/graphql-go`    | Schema-first (SDL) + resolver methods       |

Each iteration runs `parse → validate → execute → JSON-encode` so the
result includes serialization overhead. Schemas are constructed once
outside the timed region; only request-time work is measured.

:::note[How to read these tables]
Lower is better for every column. Multipliers next to each cell are
relative to `grx` for that scenario. Numbers are the mean of 5 runs from
the benchmark output committed at
[`bench.txt`](https://github.com/patrickkabwe/grx/blob/main/bench.txt);
your numbers will vary by hardware and Go version.
:::

## Headline ratios

| Scenario               | grx     | graph-gophers   | graphql-go        |
| ---------------------- | ------- | --------------- | ----------------- |
| Simple query           | 1.00×   | **3.79× slower**  | **25.20× slower** |
| Nested query           | 1.00×   | **3.74× slower**  | **23.79× slower** |
| List query (50 items)  | 1.00×   | **2.18× slower**  | **3.97× slower**  |

Allocation counts on the same scenarios:

| Scenario               | grx       | graph-gophers       | graphql-go            |
| ---------------------- | --------- | ------------------- | --------------------- |
| Simple query           | 39 allocs | 102 allocs (2.6×)   | 857 allocs (22.0×)    |
| Nested query           | 60 allocs | 168 allocs (2.8×)   | 1,322 allocs (22.0×)  |
| List query (50 items)  | 1,117 allocs | 2,026 allocs (1.8×) | 3,816 allocs (3.4×) |

## Detailed results

### Simple query

```graphql
query { user(id: "user_1") { id name email } }
```

| Library         | Time / op   | Bytes / op  | Allocs / op |
| --------------- | ----------- | ----------- | ----------- |
| **grx**         | **2.7 µs**  | **3,329 B** | **39**      |
| graph-gophers   | 10.4 µs     | 7,519 B     | 102         |
| graphql-go      | 68.9 µs     | 50,044 B    | 857         |

### Nested query

```graphql
query { post(id: "post_1") { id title body author { id name email } } }
```

| Library         | Time / op   | Bytes / op  | Allocs / op |
| --------------- | ----------- | ----------- | ----------- |
| **grx**         | **4.2 µs**  | **4,994 B** | **60**      |
| graph-gophers   | 15.6 µs     | 13,751 B    | 168         |
| graphql-go      | 99.0 µs     | 80,094 B    | 1,322       |

### List query (50 items)

```graphql
query { users(count: 50) { id name email } }
```

| Library         | Time / op    | Bytes / op    | Allocs / op |
| --------------- | ------------ | ------------- | ----------- |
| **grx**         | **59.5 µs**  | **53,645 B**  | **1,117**   |
| graph-gophers   | 129.9 µs     | 79,555 B      | 2,026       |
| graphql-go      | 235.9 µs     | 256,284 B     | 3,816       |

## Hardware and method

- **CPU**: Apple M1 Pro (`darwin/arm64`)
- **Go module**: `go 1.22`
- **Runs per case**: 5
- **Measured**: `parse → validate → execute → encoding/json.Marshal`
- **Schema construction**: outside the timed region (`b.ResetTimer()`
  after schema build)
- **Fairness knobs**: `graph-gophers/graphql-go` is configured with
  `MaxParallelism(1)` so its executor is single-threaded, matching the
  synchronous execution model of the other two and removing
  goroutine-scheduling noise from the numbers. Each library uses its
  idiomatic resolver style — we deliberately do not contort one library
  to mimic another.

## Why grx is fast on these workloads

- **No reflection on the hot path.** The `schema` package builds
  per-field metadata at startup; `exec` uses precomputed indices and
  typed accessors at request time.
- **One pass per field.** No intermediate maps, no repeated `interface{}`
  unwrapping, no JSON-as-AST.
- **Allocation-aware completion.** Result objects are built directly into
  a single response shape; lists pre-size their backing slices.
- **No third-party runtime dependencies.** The whole executor is the Go
  standard library plus `grx` itself.

These choices show up clearly in the **allocations** column more than the
nanoseconds — `grx` does 22× fewer allocations than `graphql-go` on the
nested query, which is what the GC ultimately bills you for under load.

## Reproducing locally

`benchmark/` is its own Go module so the comparison libraries never enter
the main `go.mod`. Run from inside the module:

```bash
cd benchmark
go test -bench=. -benchmem ./...
```

Or from the repo root with `go`'s `-C` flag:

```bash
go test -C benchmark -bench=. -benchmem ./...
```

For tighter numbers:

```bash
cd benchmark
go test -bench=. -benchmem -benchtime=3s -count=5 ./...
```

A single scenario across all libraries:

```bash
go test -C benchmark -bench=BenchmarkNestedQuery -benchmem ./...
```

A single library across one scenario:

```bash
go test -C benchmark -bench=BenchmarkSimpleQuery/grx -benchmem ./...
```

## Sanity check

Before running any benchmark, run the unit test that asserts every
implementation returns identical, error-free responses for every query
— this prevents shipping a fast-but-wrong number:

```bash
cd benchmark
go test ./...
```

`TestImplementationsAgree` covers the three queries above against all
three libraries.
