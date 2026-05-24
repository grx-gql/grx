---
title: Benchmarks
description: grx vs graphql-go vs graph-gophers on shared production-shaped fixtures.
outline: [2, 3]
---

# Benchmarks

The sibling [`benchmark/`](https://github.com/grx-gql/grx/tree/main/benchmark)
module runs **parse → validate → execute → JSON** for **`grx`**, **`graphql-go/graphql`** (`v0.8.1`),
and **`graph-gophers/graphql-go`** (`v1.5.0`) on identical operations and resolver data (`replace` points
`grx` at the workspace root).

**Apps only call [`Executor.Execute`](https://pkg.go.dev/github.com/grx-gql/grx/exec#Executor.Execute).**
There is no separate prepared/slow execution API.

## Running

```bash
make benchmark
go test -C benchmark -bench=. -benchmem ./...
```

## Scenarios (current harness)

| Benchmark | What it exercises |
| --------- | ---------------- |
| `BenchmarkPersistedCompound` | Named op, fragment spread, aliases, variables: roster list + highlighted post + viewer fields |
| `BenchmarkParameterizedNested` | Single parameterized root field with nested post → author selection |
| `BenchmarkFeedTimeline` | List of posts with nested `author` per row |

Full documents and variables: [`benchmark/scenarios.go`](https://github.com/grx-gql/grx/tree/main/benchmark/scenarios.go).

Production **grx** executes **sibling selections sequentially** within each selection set (predictable resolver order).
`graph-gophers` is built with **`MaxParallelism(1)`** in the harness so numbers stay comparable to serial engines.

## Latest results (representative run)

Captured **2026-05-21** on **Apple M1 Pro**, **darwin/arm64**, **Go 1.25.0**, with:

```bash
go test -C benchmark -bench=. -benchmem -benchtime=2s -count=3 ./...
```

Values are the **arithmetic mean** of the three `count` runs (`ns/op`, `B/op`, `allocs/op`). Rounded for display; reproducing on your hardware is expected to differ.

### Wall time & heap (mean)

| Scenario | Implementation | Time / op | Bytes / op | Allocs / op |
| -------- | -------------- | --------- | ---------- | ----------- |
| **PersistedCompound** | grx | **66.8 µs** | 81,026 | 670 |
| | graphql-go | 275.8 µs | 231,659 | 3,806 |
| | graph-gophers | 101.3 µs | 56,825 | 1,250 |
| **ParameterizedNested** | grx | **9.49 µs** | 9,251 | 74 |
| | graphql-go | 120.2 µs | 88,659 | 1,486 |
| | graph-gophers | 18.56 µs | 14,319 | 181 |
| **FeedTimeline** | grx | **52.5 µs** | 64,069 | 645 |
| | graphql-go | 236.0 µs | 235,505 | 3,436 |
| | graph-gophers | 102.0 µs | 67,190 | 1,509 |

### Relative time (mean ÷ grx mean)

| Scenario | graphql-go | graph-gophers |
| -------- | ---------: | ------------: |
| PersistedCompound | **4.13×** | **1.52×** |
| ParameterizedNested | **12.7×** | **1.96×** |
| FeedTimeline | **4.49×** | **1.94×** |

### Relative allocations (allocs/op ÷ grx)

| Scenario | graphql-go | graph-gophers |
| -------- | ---------: | ------------: |
| PersistedCompound | **5.68×** | **1.87×** |
| ParameterizedNested | **20.1×** | **2.45×** |
| FeedTimeline | **5.33×** | **2.34×** |

Re-check with [`benchstat`](https://pkg.go.dev/golang.org/x/perf/cmd/benchstat) once you have exported text output; the means above are computed outside that tool for readability.

_Migrating from older docs:_ scenarios used to be named `BenchmarkSimpleQuery` / `BenchmarkNestedQuery` / `BenchmarkListQuery`. Those are superseded by **PersistedCompound**, **ParameterizedNested**, and **FeedTimeline** (`benchmark/`). Refresh the figures in this page whenever you bump Go, the comparison libraries, or `grx`.

## Interpreting these numbers versus production throughput

The benchmark loop measures **steady-state executor cost on this machine**:

- **`ns/op`** is how long **one isolated iteration** takes (typically one OS thread spinning the benchmark loop) - **not** the same thing as inverse server QPS once you stack HTTP framing, middleware, pooling, contention, profiling, tracing, TLS, geo latency, databases, caches, saturation, retries, timeouts, serialization at the edges, fleet size and autoscaling, **and concurrent client load**.
- The harness uses **deterministic in-memory fixtures** (“zero-I/O” resolvers returning shared pointers). In real deployments, **`p99` is usually bounded by backends** (RPC, PostgreSQL, Redis, entitlement checks) - the GraphQL engine is often a **small fraction** of wall time unless you are intentionally CPU-heavy on trivial data.
- `go test -bench` is a **controlled micro-environment**. Treat results as answering: “for this query shape with **no datastore**, how expensive is parsing + validation + execution + encoding relative to alternatives?” - then **reproduce on your workloads** (`go test`/Netflix/load tests, `-trace`, CPU profiles).

A **rough heuristic** engineers sometimes misuse: if you pretend one core were 100 % saturated doing only GraphQL-shaped work shown here, \(\text{near upper bound ops/s} \approx 10^{9} / (\text{ns/op})\) - that still ignores everything above **and ignores that production uses many cores unevenly**.

**Summary:** benchmarks are comparisons of **relative GraphQL-runtime overhead**, not a promise of headline **production HTTP RPS** or revenue-grade SLO rates.

## Why grx often measures faster than `graphql-go/graphql` and `graph-gophers/graphql-go`

Libraries differ in runtime shape; “faster microbench” is not universal at every workload. Below is why **these** workloads tend to skew toward grx - they match how the sibling module is deliberately built (`benchmark/` parity schemas, in-memory resolves).

### 1. **Less material in grx’s hot path**

Root `go.mod` for `grx-gql/grx` is **stdlib‑centric on the executor path**: no heavyweight third‑party stacks between lexing → execution → response encoding. Comparable servers often carry richer runtime layers (builders, adaptor trees, concurrency helpers) - which show up more as **instructions + allocations even when correctness is unchanged**.

### 2. **`graphql-go/graphql` allocates and dispatches broadly**

Classic `graphql.NewObject` / `graphql.Field` setups pay for **explicit schema objects**, **widely routed `Resolve` closures**, **`map`-shaped intermediates**, and pervasive interface dispatch. That buys flexibility; it routinely costs extra **heap traffic and branches** versus a schema that was **compiled ahead of execution** once your types are fixed.

### 3. **`graph-gophers/graphql-go` indirection plus wrapper types**

SDL-first ergonomics commonly mean **thin resolver façade types** translating between GraphQL-facing methods and backing models. Listing resolvers tends to manufacture **arrays of adaptor structs**. The benchmark pins `MaxParallelism(1)` because the upstream package may otherwise launch **per-field concurrency** unrelated to datastore parallelism - skewing totals against single-thread‑style engines.

### 4. **grx binds schema from Go structs up front**

`schema.Build` materialises fields, coercion tables, resolver wiring, and introspection artefacts **during startup or cacheable compilation**, reducing **per‑request scaffolding** versus repeatedly walking generic builder graphs or façade layers.

### 5. **Response construction targets GraphQL-shaped output**

Rather than bouncing through generic recursive maps everywhere, responses lean on **`core.OrderedObject`** semantics so serialization can track **explicit field ordering** aligned with selections with less reshaping churn.

### 6. **Execution-time allocation hygiene**

Mechanisms such as pooled **scratch buffers for transient GraphQL paths** during resolution (paired with deterministic copies where errors persist) shave **tiny slice/header allocations** across deep trees and lists - they matter most exactly when backends are intentionally cheap, as here.

---

None of these points replace profiling: **`go tool pprof`**, tracer comparison, **`benchstat` across `-count=` runs**, plus **replay of real persisted operations** (`benchmark/scenarios.go` style) alongside real data sources - that is where production-critical rates are defended.
