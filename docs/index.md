---
layout: home
title: grx
titleTemplate: false

hero:
  name: grx
  text: GraphQL in Go—plain structs, one module, sharp edges optional.
  tagline: grx builds your schema from structs, serves queries and mutations over HTTP, subscriptions over WebSocket or SSE—and ships as plain net/http.Handler. Read what it covers before you scaffold.
  image:
    src: /hero.svg
    alt: Stylized graph of nodes and edges suggesting a GraphQL execution tree.
  actions:
    - theme: brand
      text: What grx is — features
      link: /concepts/what-is-grx
    - theme: alt
      text: Run the tutorial
      link: /getting-started/
    - theme: alt
      text: GitHub
      link: https://github.com/patrickkabwe/grx

features:
  - icon: ✨
    title: Feature surface & scope
    details: Tables of bundled capabilities—schemas from structs, HTTP + GraphiQL, plugins, realtime, persisted queries—and who typically adopts grx.
    link: /concepts/what-is-grx
    linkText: What grx is
  - icon: 📘
    title: Schema in Go
    details: Exported structs plus gql tags—the contract clients see. Read this before branching into advanced patterns.
    link: /concepts/schema-basics
    linkText: Define your schema
  - icon: 🛡
    title: Production hardening
    details: Separate guides for masking and authorizers, introspection / GraphiQL knobs, payload caps and persisted-query modes—all linked from one hub page.
    link: /concepts/graphql-security-production
    linkText: Open overview
  - icon: ❓
    title: GraphQL fundamentals (FAQ)
    details: REST vs GraphQL, resolvers vs schema, N+1, contextual vocabulary when pairing with teammates.
    link: /guides/graphql-backend-essentials
    linkText: Read the FAQ
  - icon: ⚡
    title: Realtime when you want it
    details: Subscriptions over WebSocket or Server-Sent Events, plus optional pub/sub so mutations can fan out events without bolting on another stack.
    link: /guides/subscriptions
    linkText: Add subscriptions
  - icon: 🧭
    title: Go further when you need to
    details: Plugins for auth and observability, custom transports, and a clear picture of how requests move through the runtime—only open these when your problem needs them.
    link: /concepts/architecture
    linkText: How it fits together
  - icon: 📦
    title: API you can read
    details: Same package layout as any Go library. Generated reference mirrors the doc comments in the repo.
    link: /reference/
    linkText: Browse packages
---

## What grx is — in one glance

[**grx**](https://pkg.go.dev/github.com/patrickkabwe/grx) is **a GraphQL server and runtime for Go**: struct types and methods compile into executable schema (**code-first**, no obligatory SDL codegen). Queries and mutations ride the **default HTTP** transport you mount as **`http.Handler`**; **`Subscription`** pushes over **WebSocket** or **SSE** when you attach those transports.

**Included surface today:** **`gql` tags** + roots (**`schema.Config`**), bundled **GraphiQL** for dev (**opt-in/off** later), [**plugins**](/concepts/plugins), operation and **field authorizers**, persisted / trusted-query patterns, document shape limits **and timeouts** (**[Production hardening](/concepts/graphql-security-production)** overview), optional gzip and CORS helpers, **pub/sub adapters** (`pkg/pubsub/redis`) for multi-instance fan-out, and benchmarks against other servers.

👉 [**Full breakdown: capability tables and who it suits →**](/concepts/what-is-grx)

## Choose a path

**Mapping GraphQL vocabulary to APIs?** [How GraphQL backends work](/guides/graphql-backend-essentials) answers the usual “why” questions alongside links into grx specifics.

**Modeling shapes?** Always start **[Define your schema](/concepts/schema-basics)** (listed under **Guides** alongside security and mutations).

**Operational safety?** **[Security](/guides/production-security)** (masking, authorizers), **[Introspection](/guides/introspection)** (**`__schema`**, GraphiQL), **[Limits](/guides/request-limits)** (payload caps, timeouts, persisted queries), **[Deployment](/guides/deployment)** (Docker, proxies)—or the **[overview hub](/concepts/graphql-security-production)** for a single bookmark.

**Building a larger API?** [Queries and mutations](/guides/query-mutation-server), [Realtime subscriptions](/guides/subscriptions), and the [Roadmap](/roadmap) for what is supported end-to-end.

## Project status

grx targets the [GraphQL October 2021 spec](https://spec.graphql.org/October2021/) for execution and wire behaviour. Coverage and gaps are tracked on the [Roadmap](/roadmap). Numbers versus other Go servers live on [Benchmarks](/benchmarks).
