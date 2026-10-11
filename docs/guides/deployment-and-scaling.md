---
title: Deployment and scaling
description: Deploy grx services with HTTP, WebSocket, SSE, and shared pub/sub.
outline: deep
---

# Deployment and scaling

grx can run behind the HTTP infrastructure you already use. The deployment
model changes when subscriptions keep long-lived WebSocket or SSE connections
open.

## HTTP-only services

HTTP queries and mutations are stateless when application state lives in your
database or other shared services. Run multiple replicas behind a load
balancer, terminate TLS at the proxy, and expose health and readiness checks
from the host application.

## Services with subscriptions

Each replica owns its connected WebSocket and SSE clients. To publish an event
to clients connected to different replicas, use a shared pub/sub backend such
as Redis. An in-memory backend only reaches clients connected to the same
process.

Plan for connection draining during deployments. Stop accepting new
connections, allow active streams to close within a deadline, and make clients
reconnect with an explicit backoff policy.

## Infrastructure checklist

- Set TLS, proxy, and request timeout policy outside grx.
- Set WebSocket upgrade and idle timeout policy in the load balancer.
- Use `CheckOrigin` for browser WebSocket connections.
- Use shared pub/sub for cross-replica fan-out.
- Expose readiness separately from liveness.
- Add request IDs, metrics, and tracing before increasing replica count.
- Test rolling deployments with active subscriptions.

See [Runtime boundaries](/concepts/runtime-boundaries) and [Custom pub/sub](/guides/custom-pubsub)
for the ownership model behind these choices.
