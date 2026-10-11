---
title: Why choose grx
description: Decide whether grx fits your Go GraphQL service before learning the API.
outline: deep
---

# Why choose grx

Choose grx when you want a GraphQL server that fits the Go HTTP stack you
already use and lets Go types stay close to resolver code.

## The short version

grx gives you:

- Code-first schemas built from Go structs and methods.
- A normal `net/http.Handler` that works with standard routers and middleware.
- A built-in HTTP transport for queries and mutations.
- Optional WebSocket and SSE transports for subscriptions.
- Plugins, authorization hooks, request limits, persisted queries, and other
  GraphQL controls.
- A small set of focused packages instead of a framework that owns your whole
  application.

Your application still owns the database, business rules, authentication
system, deployment, and HTTP policy around the handler. See
[Runtime boundaries](/concepts/runtime-boundaries) before designing a
production service.

## Choose grx when

grx is a good fit when:

- Your service is written in Go.
- You want schema definitions and resolver code in the same language.
- You already use `net/http`, ServeMux, Chi, Gin, Echo, Fiber, or another
  router.
- You want subscriptions without replacing your HTTP server.
- You want to add production controls without adopting a large application
  framework.
- You prefer explicit Go interfaces for transports, plugins, and pub/sub.

## Choose another approach when

Consider another tool when:

- Your team needs a schema-first workflow with SDL as the primary contract.
- Your service is not written in Go and cannot expose a Go runtime.
- You need a managed GraphQL gateway rather than a library inside your service.
- Your main requirement is federation or a feature that grx does not support.
- You want a code generator to own most resolver and schema boilerplate.

The right choice depends on the ownership model your team wants. grx keeps
the runtime close to your Go code, while your application keeps control of
the surrounding system.

## How grx fits into a service

The typical request path is:

1. Your router and middleware receive the request.
2. The grx handler selects an HTTP, WebSocket, or SSE transport.
3. The transport decodes the GraphQL request.
4. grx parses and validates the document.
5. The executor calls resolver methods from your schema.
6. The transport writes the result to the client.

This separation lets you use grx for GraphQL behavior while keeping routing,
TLS, CORS, authentication extraction, databases, and deployment in the parts
of your system that already own them.

## Continue

- [Run your first server](/getting-started/)
- [Define your schema](/concepts/schema-basics)
- [Runtime boundaries](/concepts/runtime-boundaries)
- [HTTP, WebSocket, and SSE](/concepts/transports)
