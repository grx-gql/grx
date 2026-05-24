---
title: What grx is
description: grx in plain language - a Go GraphQL server built from structs, with HTTP, realtime transports, hooks, and production controls.
outline: deep
---

# What grx is

**grx** is a **GraphQL server and runtime for Go**. You describe your API **in code**: exported structs and methods become GraphQL types, fields, and resolvers (**`schema.Config`** feeding **`grx.WithSchema`**). The library turns that bundle into an executable schema, runs **`query`**, **`mutation`**, and optional **`subscription`** operations, and returns spec-shaped JSON - all behind a normal **`net/http.Handler`** so Chi, Gin, Echo, Fiber, **ServeMux**, or bare **`ListenAndServe`** can host it equally.

::: tip Dependencies  
Runtime dependencies are deliberately small (see **`go.mod` in the repo**). You integrate databases, caches, Auth0, OTLP exporters, etc. - grx wires the GraphQL mechanics and transports.

:::

---

## Who it is for

- Teams shipping **production GraphQL backends in Go** who want **reflection-driven schema authoring** instead of parallel SDL/codegen upkeep.
- Services that sometimes need **realtime pushes** (**WebSocket**, **SSE**, optional **pub/sub**) without swapping frameworks.
- Engineers who prefer **readable extension points**: [**plugins**](/reference/plugin/), authorizers, and custom [**transports**](/concepts/transports) - not a sprawling plugin ecosystem.

---

## What you get (feature surface)

These are concrete capabilities surfaced by the codebase and documented here - not marketing bullets.

### Schema and execution

| Capability | Meaning |
| --- | --- |
| **Code-first schema** | Structs **`+ gql struct tags`** model object types, arguments, descriptions, deprecation - [**Define your schema**](/concepts/schema-basics). |
| **Roots** | Required **`Query`**, optional **`Mutation`** and **`Subscription`** via **`schema.Config`**. |
| **Executor** | Lex → parse → validate → execute; targets [**GraphQL October 2021**](https://spec.graphql.org/October2021/) behaviour - parity gaps on [Roadmap](/roadmap). |
| **Structured errors** | Field errors with **`path`** and locations; masking for clients via [**Security**](/guides/production-security). |

### Networking and ergonomics

| Capability | Meaning |
| --- | --- |
| **GraphQL over HTTP** | Default handler accepts JSON bodies on your configured path - [**Transports**](/concepts/transports). |
| **Optional GraphiQL** | **`WithPlaygroundPath`** serves the bundled sandbox for dev - turn it off in production ([**Introspection**](/guides/introspection)). |
| **Router-neutral** | Everything is **`http.Handler`** - mount anywhere ([**Get started**](/getting-started/)). |
| **CORS wiring** | Middleware helper (**[`grx.Cors`](https://pkg.go.dev/github.com/grx-gql/grx#Cors)**) for browsers and WebSocket origins. |
| **Gzip responses** | Optional **`WithResponseGzip`** for JSON payloads. |

### Realtime graph

| Capability | Meaning |
| --- | --- |
| **`Subscription`** resolvers | Go methods returning **`<-chan YourType`**; runtime streams payloads per event. |
| **WebSocket + SSE transports** | First-class [**subscriptions**](/guides/subscriptions) doc + optional **`memory-pubsub`** (Redis adapter for multi-node fan-out cases). |

### Safety and operations

| Capability | Meaning |
| --- | --- |
| **Toggle introspection** | **`WithDisableIntrospection`** rejects `__schema` / `__type`. |
| **Request bounds** | Timeouts (**`WithRequestTimeout`**), **`MaxHTTPRequestBytes`**, variable caps, selection/alias/root limits - [**Limits**](/guides/request-limits). |
| **APQ / trust lists** | **`WithPersistedQueries`**, **`TrustedDocuments`**, **`RequirePersistedQuery`** (**`server.Config`**) patterns. |
| **Auth hooks** | Operation + **field-level authorizers** and HTTP middleware layering - [**Authentication & authorization**](/guides/auth). |

### Observability & extension

| Capability | Meaning |
| --- | --- |
| **Lifecycle plugins** | Parse / validation / resolve hooks (**[plugin package](/reference/plugin/)**, [**concepts**](/concepts/plugins)). |
| **Custom transports** | Add protocols beside the default HTTP pipeline - [**guide**](/guides/custom-transport). |

---

## How it differs from hand-rolling GraphQL.js-style stacks

Conceptually identical moving parts (**schema + resolvers + transport**) - grx concentrates them in Go types and one handler tree rather than layering Node + SDL + codegen. Use it when idiomatic structs, low ceremony, and **`net/http`** integration outweigh a JavaScript-heavy toolchain.

---

## Next steps

1. [**Run your first server**](/getting-started/) (**`go.mod`**, **`graph` package**, **`grx.NewServer`**).  
2. [**Define your schema in Go**](/concepts/schema-basics) for tagging and composition patterns.  
3. Bookmark **[Security](/guides/production-security)**, **[Introspection](/guides/introspection)**, **[Limits](/guides/request-limits)** (or **[Production hardening](/concepts/graphql-security-production)**) before shipping beyond localhost.  

Numbers versus other servers: [**Benchmarks**](/benchmarks). Package-level API browsing: [**Reference**](/reference/).
