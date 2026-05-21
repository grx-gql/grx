---
title: Migrate to grx
description: Pick the right migration guide based on which Go GraphQL library you're moving away from.
outline: [2, 3]
---

This section walks through migrating an existing Go GraphQL service to
grx. Pick the guide that matches the library you're leaving:

| Coming from                                  | Style                                              | Guide                                                       |
| -------------------------------------------- | -------------------------------------------------- | ----------------------------------------------------------- |
| `github.com/graphql-go/graphql`              | Code-first, builder API (`graphql.NewObject`)      | [From graphql-go](/guides/migrate/from-graphql-go)         |
| `github.com/graph-gophers/graphql-go`        | Schema-first SDL string + per-type resolver structs | [From graph-gophers](/guides/migrate/from-graph-gophers)   |

Both guides assume you're already comfortable with the source library
and skip GraphQL fundamentals. They focus on the *delta* — what
disappears, what shows up new, and how to keep your existing schema
shape intact while you switch executors.

## What you gain

- **Performance.** On the [bundled benchmarks](/benchmarks), grx is
  **~25× faster** with **22× fewer allocations** than `graphql-go` on
  simple queries, and **~3.7× faster** with **~2.8× fewer allocations**
  than `graph-gophers/graphql-go` on nested queries.
- **Zero runtime dependencies.** grx is a single Go module that imports
  only the standard library. The whole runtime is auditable, vendorable,
  and easy to upgrade.
- **First-class subscriptions.** WebSocket (`graphql-transport-ws`) and
  Server-Sent Events transports ship in the box. See
  [Add Subscriptions](/guides/subscriptions).
- **Plugin lifecycle hooks** for logging, tracing, auth, and request-id
  propagation without reaching into the executor. See
  [Write a Custom Plugin](/guides/custom-plugin).

## What's missing today

grx does not yet implement the full October 2021 spec. Before
committing to a migration, scan the [Roadmap](/roadmap) for any feature
your existing schema relies on. The most likely blockers today:

- **Enum, interface, and union types** are not supported yet
  ([Type System](/roadmap#type-system)).
- **Custom scalars** can't be registered yet — only the built-in
  `String`, `Int`, `Float`, `Boolean`, `ID` scalars are available.
- **Field aliases**, **fragments**, and **`@skip` / `@include`** are not
  yet honoured by the executor ([Execution](/roadmap#execution)).
- **Default argument values** and **descriptions / deprecation
  metadata** aren't carried through.

If your schema uses any of those, file an issue so the priority is
visible — or stay on your current library until the relevant checklist
items are complete.

## Recommended order

For both source libraries, the lowest-risk migration order is:

1. **Read the [Schema Mapping](/concepts/schema-mapping) and
   [Resolvers](/concepts/resolvers) concept pages** to internalise
   grx's struct-tag conventions.
2. **Stand up a parallel grx server in a sibling package** that defines
   the same schema using the patterns from the migration guide. Don't
   delete anything yet.
3. **Run both executors against the same fixture** to confirm responses
   match field-for-field. Steal `TestImplementationsAgree` from
   [`benchmark/bench_test.go`](https://github.com/patrickkabwe/grx/blob/main/benchmark/bench_test.go)
   as a template.
4. **Switch your HTTP handler** to `grx.NewServer` and remove the old
   library from your `go.mod`.
5. **Move plugins last.** `RequestStart` is the hook you want for
   anything that used to live in middleware around `graphql.Do` or the
   `graph-gophers` HTTP wrapper.
