---
title: Production hardening
description: Hub linking Security, Introspection, Limits, Deployment, and auth—bookmark-friendly URL preserved.
outline: deep
---

# Production hardening

This site splits operational concerns into focused guides — **Security**, **Introspection**, **Limits**, plus **Deployment** — so navigation matches how teams reason about risk and shipping.

| Topic | Guide |
| --- | --- |
| **Security** | [**Security**](/guides/production-security) — mask internal errors, operation/field authorizers, rate-limit hooks before resolvers run |
| **Introspection** | [**Introspection**](/guides/introspection) — disable `__schema`, retire GraphiQL, avoid accidental SDL export |
| **Limits** | [**Limits**](/guides/request-limits) — HTTP body + variable caps, clocks, persisted/trusted query modes, selection guards |
| **Deployment** | [**Deployment**](/guides/deployment) — Docker, Compose, reverse proxies, Kubernetes, managed platforms |

Cross-cutting identity patterns live in [**Authentication & authorization**](/guides/auth). Deep execution notes stay in [**Execution pipeline**](/concepts/execution). Transport wiring is summarized in [**Routers & transports**](/concepts/transports).
