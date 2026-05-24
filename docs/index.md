---
layout: home
title: "grx: Go GraphQL server for struct-first APIs"
titleTemplate: false
description: Struct-first GraphQL server for Go with net/http integration, subscriptions, SSE, WebSocket, persisted queries, plugins, pub/sub, and production controls.

hero:
  name: grx
  text: "GraphQL in Go: structs are the schema."
  tagline: Your router, not ours. Add WebSocket/SSE subscriptions and safety toggles when you're ready.
  image:
    src: /hero.svg
    alt: Abstract graph illustrating how fields connect in a GraphQL response.
  actions:
    - theme: brand
      text: Get started
      link: /getting-started/
    - theme: alt
      text: Github
      link: https://github.com/grx-gql/grx

features:
  - icon: ✨
    title: Production-ready GraphQL out of the box
    details: Persisted queries, subscriptions, streaming transports, limits, CORS, gzip, plugins, pub/sub, and security controls built into the runtime.
    link: /concepts/what-is-grx
    linkText: Explore capabilities
  - icon: 📘
    title: Define schemas directly from Go structs
    details: Build schemas from Go types and struct tags so fields, arguments, nullability, descriptions, and resolvers stay close to your code.
    link: /concepts/schema-basics
    linkText: See schemas
  - icon: 📡
    title: Works with the Go HTTP stack you already use
    details: Compose with net/http, Gin, Fiber, Echo, Chi, and custom routers without giving up your existing middleware or observability.
    link: /concepts/transports
    linkText: Mount GRX
  - icon: 🛡
    title: Secure and control GraphQL in production
    details: Control introspection, trusted operations, depth, complexity, payloads, authorization, and abusive traffic from one runtime surface.
    link: /concepts/graphql-security-production
    linkText: Review controls
  - icon: ⚡
    title: Real-time GraphQL without extra infrastructure
    details: Run subscriptions over graphql-transport-ws or SSE with connection handling, origin checks, idle timeouts, and optional Redis pub/sub.
    link: /guides/subscriptions
    linkText: Build realtime APIs
  - icon: 🧭
    title: Observe and extend the execution pipeline
    details: Hook into parsing, validation, execution, transports, subscriptions, and resolvers for logging, tracing, auth, caching, and metrics.
    link: /concepts/plugins
    linkText: Inspect hooks
  - icon: 📦
    title: Modular packages without framework lock-in
    details: Use focused packages for execution, schemas, transports, subscriptions, plugins, HTTP integration, CORS, pub/sub, and streaming.
    link: /reference/
    linkText: Browse packages

footer:
  message: Execution targets the GraphQL October 2021 spec.
  copyright: MIT License · Open source on GitHub
---
