---
title: Runtime boundaries
description: What grx owns and what belongs to your application or deployment platform.
outline: deep
---

# Runtime boundaries

grx owns GraphQL behavior. Your application and deployment platform own the
system around the GraphQL handler.

## Responsibility map

| Concern | Owner | Typical grx integration |
| --- | --- | --- |
| Schema, parsing, validation, and execution | grx | `schema.Config`, `grx.NewServer` |
| HTTP, WebSocket, and SSE GraphQL protocols | grx | Built-in transports |
| Field and operation policy | Shared | Authorizers, plugins, and application rules |
| CORS and browser preflight | Host application | Middleware around the handler |
| Authentication extraction | Host application | Router middleware or WebSocket `OnConnect` |
| TLS and public routing | Proxy or host | Reverse proxy and router configuration |
| Database, transactions, and business rules | Application | Services called by resolvers |
| Metrics, tracing, and request IDs | Host and observability stack | Middleware and plugins |
| Scaling and connection routing | Deployment platform | Shared pub/sub and load balancer policy |

## CORS is outside the GraphQL runtime

grx returns an `http.Handler`. Wrap that handler with the CORS middleware for
your HTTP stack. This lets one policy cover GraphQL, health checks, and other
routes owned by the host application.

See [CORS and browsers](/guides/cors-browsers) for preflight requests,
credentials, allowed headers, and router examples.

HTTP CORS does not configure WebSocket origins. Set an explicit origin policy
with `websocket.Config.CheckOrigin` for browser WebSocket connections.

## Continue

- [HTTP, WebSocket, and SSE](/concepts/transports)
- [Authentication and authorization](/guides/auth)
- [Deployment and scaling](/guides/deployment-and-scaling)
