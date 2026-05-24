---
layout: home
title: grx
titleTemplate: false

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
    title: Shipped capability matrix
    details: Tables that list executable schema authoring JSON GraphQL transports optional GraphiQL gzip and CORS subscriptions over WebSocket and SSE publishers for mutations field and operation guardrails timeouts and payload caps persisted and trusted queries plugins pub sub backends and benchmarking notes.
    link: /concepts/what-is-grx
    linkText: Open the feature tables
  - icon: 📘
    title: Struct-first schema authoring
    details: gql struct tags declare fields arguments nullability deprecation and descriptions while schema.Config binds Query Mutation and Subscription roots resolver methods stay beside the structs they expose for code review and refactoring.
    link: /concepts/schema-basics
    linkText: Define types and roots
  - icon: 📡
    title: Bundled HTTP surface
    details: Serve JSON operations over configurable GraphQL paths add the playground path helper when debugging enable gzip middleware for responses and Cors helper for browsers everything composes through net/http so chi Gin Fiber Echo or ServeMux own routing.
    link: /concepts/transports
    linkText: Mount transports
  - icon: 🛡
    title: Operational guardrail APIs
    details: Turn introspection GraphiQL and persisted-query modes per environment mask surfaced errors authorize whole operations or single fields clamp document depth alias counts variable bytes elapsed time and streamed payloads using documented server and executor configuration.
    link: /concepts/graphql-security-production
    linkText: Review hardening options
  - icon: ⚡
    title: Streaming stack in the box
    details: graphql-transport-ws with idle and origin checks SSE one-way transports optional memory pub-sub plus redis pubsub module for horizontally scaled fleets typed publish subscribe helpers mutations publish subscription resolvers stream Go channels flushed by executor.
    link: /guides/subscriptions
    linkText: Enable WebSocket SSE pub sub
  - icon: 🧭
    title: Lifecycle plugin hooks
    details: Inspect parse validation subscription setup and resolver phases through the plugin interfaces build logging denial telemetry or cache layers transport extensions reuse the same executor by implementing core.Transport for bespoke gateways.
    link: /concepts/plugins
    linkText: See plugins transports
  - icon: 📦
    title: Exported packages and refs
    details: Dedicated modules for schema exec core server websocket sse http client cors plugin observability memory pubsub redis-pubsub gomarkdoc-derived reference pages duplicate Go doc prose so navigating signatures feels like browsing any idiomatic dependency.
    link: /reference/
    linkText: Browse package refs

footer:
  message: Execution targets the GraphQL October 2021 spec.
  copyright: MIT License · Open source on GitHub
---