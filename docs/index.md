---
layout: home

hero:
  name: grx
  text: The fast, dependency-free Go GraphQL runtime.
  tagline: Code-first schemas, predictable execution, built-in subscriptions and pub/sub. Roughly 25× faster than graphql-go with zero third-party imports.
  image:
    src: /hero.svg
    alt: Abstract diagram of a GraphQL execution graph fanning out from a root node into nested fields.
  actions:
    - theme: brand
      text: Get Started
      link: /getting-started
    - theme: alt
      text: View on GitHub
      link: https://github.com/patrickkabwe/grx

features:
  - icon: 🚀
    title: Fast by construction
    details: On the bundled benchmarks, grx executes a simple query in roughly 2.7 µs with 39 allocations — about 25× faster and 22× fewer allocations than graphql-go.
    link: /benchmarks
    linkText: See benchmarks
  - icon: 🧩
    title: Zero runtime dependencies
    details: A single Go module, go 1.22+, no third-party imports. The whole runtime is auditable, vendorable, and easy to upgrade.
  - icon: 📄
    title: Code-first schemas
    details: Define your schema as plain Go structs and methods. Reflection runs once at startup; the per-request hot path stays allocation-aware.
    link: /concepts/schema-mapping
    linkText: Schema mapping
  - icon: ⚡
    title: Built-in subscriptions
    details: Realtime out of the box with the graphql-transport-ws subprotocol over WebSocket and GraphQL over Server-Sent Events.
    link: /guides/subscriptions
    linkText: Subscriptions guide
  - icon: 🔌
    title: Pluggable transports
    details: HTTP+JSON, WebSocket, SSE, or your own — anything that speaks GraphQL is a core.Transport registered on the server.
    link: /concepts/transports
    linkText: Transports
  - icon: 📡
    title: Pub/Sub built in
    details: A typed pubsub.PubSub primitive with an in-process default and an optional Redis backend lets mutations fan events to subscriptions across goroutines or replicas.
    link: /concepts/pubsub
    linkText: Pub/Sub
  - icon: 🪝
    title: Plugin lifecycle
    details: Hook into request, parse, validate, execute, field-resolve, and response phases. Compose logging, tracing, and auth without touching the core.
    link: /concepts/plugins
    linkText: Plugins
---

## Start here

| Topic | Description |
| --- | --- |
| [Getting Started](/getting-started) | Install grx, define your first schema, and run the server in under five minutes. |
| [Concepts](/concepts/architecture) | Architecture, schema mapping, executor, transports, subscriptions, plugins, and pub/sub. |
| [Guides](/guides/query-mutation-server) | Task-oriented walkthroughs for queries, mutations, subscriptions, plugins, and custom transports. |
| [Benchmarks](/benchmarks) | Comparative numbers vs graphql-go and graph-gophers, plus how to reproduce them locally. |
| [Migrate to grx](/guides/migrate/) | Step-by-step migration from graphql-go/graphql or graph-gophers/graphql-go. |
| [API Reference](/reference/) | Package-level reference sourced from Go doc comments. |

## Project status

grx implements a useful subset of the [GraphQL October 2021 specification](https://spec.graphql.org/October2021/) with production-grade transports for queries, mutations, and subscriptions. It is under active development; see the [Roadmap](/roadmap) for what is and is not supported today.
