# grx benchmarks

Self-contained timings for **`grx`** against two common Go GraphQL libraries on the **same schemas and fixtures**:

| Package | Typical style |
| --- | --- |
| `github.com/grx-gql/grx` | Code-first resolvers (`Execute` only) |
| `github.com/graphql-go/graphql` | Code-first schema builder API |
| `github.com/graph-gophers/graphql-go` | SDL + resolver structs (`MaxParallelism(1)` for fair comparison) |

Published doc tables (means, **2026-05-21**, M1 Pro / Go 1.25, `-benchtime=2s -count=3`): **[Benchmarks](https://grx-gql.github.io/grx/benchmarks.html)** / [`docs/benchmarks.md`](/docs/benchmarks.md).

## Why a sibling module?

The root `go.mod` stays lean. Bench harness deps stay **outside** the library module via `replace` → parent tree.

## Running

```bash
make benchmark
go test -C benchmark -bench=. -benchmem ./...
go test -C benchmark -bench=BenchmarkParameterizedNested/graphql-go -benchmem ./...
```

## Scenarios & fairness

Queries live in [`scenarios.go`](./scenarios.go). Root `user` / `post` args are **`String!`** so validation matches **grx** mapping Go `string` → GraphQL `String`; output `User.id` / `Post.id` remain `ID!`.

`TestImplementationsAgree` rejects misconfigured schemas before trusting bench numbers.

## Interpreting results & production throughput

Bench output is **`ns/op` on one loop thread** against **RAM-only resolvers**. That answers “relative executor overhead,” **not** your production HTTP throughput (databases and networks usually dominate **`p99`**). For the honest translation and concrete reasons grx skews lighter than the comparison libraries **in these fixtures**, see the site page [**Benchmarks — interpretation & internals**](https://grx-gql.github.io/grx/benchmarks.html) *(source [`docs/benchmarks.md`](/docs/benchmarks.md))*.
