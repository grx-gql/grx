# grx Documentation Rewrite Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Rewrite the grx documentation so readers can move from product fit to a working server, core concepts, integrations, production guides, examples, and API reference in a clear order.

**Architecture:** Keep stable page URLs where practical, but reorganize the sidebar and rewrite content around a progressive reading path. Separate grx runtime responsibilities from host application and platform responsibilities. Use short concept pages for mental models and focused guides for configuration and implementation.

**Tech Stack:** Markdown, VitePress, TypeScript navigation config, Go code examples, Bun.

**Spec:** `docs/superpowers/specs/2026-10-11-documentation-information-architecture-design.md`

## Global Constraints

- Use simple English with precise technical terms.
- Avoid em dashes.
- Preserve stable URLs unless a target is clearly obsolete.
- Keep the repository root `ROADMAP.md` unchanged.
- Remove roadmap content from the public docs site.
- Keep code examples aligned with the current `Schema` constructor API.
- Do not claim that grx provides CORS middleware; explain host middleware placement.
- Run `bun run build` in `docs/` and `go test ./...` before completion.

## Review Focus

- A new reader can reach a working server without reading every concept page.
- The navigation distinguishes concepts, integrations, guides, examples, API reference, and internals.
- CORS and other host concerns are not incorrectly attributed to grx.
- Subscription scaling guidance distinguishes in-memory pub/sub from shared backends.
- All internal links, constructor names, and code examples remain current.

### Task 1: Rebuild navigation and public entry points

**Files:**
- Modify: `docs/.vitepress/config.ts`
- Modify: `docs/index.md`
- Modify: `docs/README.md`
- Create or modify: `docs/concepts/why-grx.md`
- Modify: `docs/roadmap.md` or remove its public navigation/content according to the existing docs generation flow

**Interfaces:**
- Produces the navigation labels and links used by every later documentation task.

- [ ] Rewrite the sidebar into `Start here`, `Concepts`, `Integrations`, `Guides`, `Examples`, `API reference`, and `Under the hood` groups.
- [ ] Move the router pages under the `Integrations` label while keeping `/getting-started/...` URLs stable.
- [ ] Remove the Roadmap entry and public roadmap page from the docs site without changing root `ROADMAP.md`.
- [ ] Add `Why grx` to the Start here path and the home page.
- [ ] Write `why-grx.md` with product fit, design choices, tradeoffs, and cases where another tool is a better fit.
- [ ] Build the docs site with `bun run build` and verify the navigation renders.
- [ ] Commit with `docs: reorganize documentation navigation`.

### Task 2: Rewrite the onboarding path

**Files:**
- Modify: `docs/getting-started/index.md`
- Modify: `docs/getting-started/basic-http.md`
- Modify: `docs/concepts/schema-basics.md`
- Modify: `docs/concepts/resolvers.md`
- Modify: `docs/concepts/schema-mapping.md`

**Interfaces:**
- Consumes the navigation and Why grx page from Task 1.
- Produces the beginner path linked from the home page.

- [ ] Rewrite the opening of the getting-started page around an explicit outcome: a running GraphQL server.
- [ ] Explain the code-first schema model before showing advanced options.
- [ ] Show the request path from HTTP request to transport, validation, resolver, and response.
- [ ] Remove duplicated framework setup from the basic onboarding page.
- [ ] Link each page to the next concept and to the relevant API reference.
- [ ] Search the rewritten pages for em dashes and stale constructor names.
- [ ] Build the docs site and run `go test ./...`.
- [ ] Commit with `docs: rewrite onboarding documentation`.

### Task 3: Rewrite runtime and boundary concepts

**Files:**
- Modify: `docs/concepts/transports.md`
- Modify: `docs/concepts/execution.md`
- Modify: `docs/concepts/plugins.md`
- Modify: `docs/concepts/subscriptions.md`
- Modify: `docs/concepts/pubsub.md`
- Create or modify: `docs/concepts/runtime-boundaries.md`
- Modify: `docs/guides/cors-browsers.md`

**Interfaces:**
- Consumes the onboarding vocabulary from Task 2.
- Produces the boundary and runtime concepts used by deployment and integration guides.

- [ ] Add a responsibility table that identifies what grx owns and what the host application or platform owns.
- [ ] Cover CORS, authentication extraction, CSRF, middleware, request IDs, tracing, metrics, health checks, readiness, graceful shutdown, configuration, and secrets.
- [ ] Explain CORS configuration for `net/http`, Chi, Gin, and Fiber, including preflight and credentials.
- [ ] Explain WebSocket `CheckOrigin` separately from HTTP CORS.
- [ ] Rewrite transport and execution pages so prerequisites and flow are explicit.
- [ ] Explain pub/sub ownership, cancellation, buffering, delivery semantics, ordering, duplicates, and cleanup.
- [ ] Update all links to use current `Schema` examples.
- [ ] Build the docs site and search for broken internal references.
- [ ] Commit with `docs: explain runtime boundaries and transports`.

### Task 4: Add pub/sub extension and scaling guidance

**Files:**
- Modify: `docs/guides/subscriptions.md`
- Create or modify: `docs/guides/custom-pubsub.md`
- Create or modify: `docs/guides/deployment-and-scaling.md`
- Modify: `docs/guides/deployment.md`
- Modify: `docs/guides/migrate/index.md` if navigation grouping requires it

**Interfaces:**
- Consumes the pub/sub and transport concepts from Task 3.
- Produces operational guidance for multi-process and production deployments.

- [ ] Document the `pubsub.PubSub` interface and the responsibilities of a custom implementation.
- [ ] Explain best-effort in-memory delivery versus shared Redis-backed delivery.
- [ ] Cover horizontal HTTP scaling, external pub/sub, WebSocket routing, sticky sessions, TLS termination, reverse proxies, timeouts, connection limits, rolling deploys, and connection draining.
- [ ] Explain health checks, graceful shutdown, observability during scale-out, and failure behavior.
- [ ] Keep examples runnable and separate deployment assumptions from library behavior.
- [ ] Build the docs site and run `go test ./...`.
- [ ] Commit with `docs: document pubsub and scaling patterns`.

### Task 5: Rewrite application architecture and task guides

**Files:**
- Modify: `docs/guides/auth.md`
- Modify: `docs/guides/middleware.md`
- Modify: `docs/guides/testing.md`
- Modify: `docs/guides/query-mutation-server.md`
- Modify: `docs/guides/query-limits.md`
- Modify: `docs/guides/request-limits.md`
- Modify: `docs/guides/deployment.md`
- Modify: `docs/guides/file-upload.md`
- Modify: `docs/guides/persisted-queries.md`
- Modify: `docs/guides/custom-scalars.md`
- Modify: `docs/guides/schema-directives.md`
- Create or modify: `docs/guides/application-architecture.md`

**Interfaces:**
- Consumes the boundary vocabulary from Tasks 2 and 3.
- Produces focused task pages with consistent introductions and follow-up links.

- [ ] Group guides by problem instead of implementation history.
- [ ] Add database access, transactions, caching, queues, background jobs, data loading, N+1, and resolver/service boundaries.
- [ ] Make every guide state its problem, prerequisites, smallest useful implementation, production concerns, and next page.
- [ ] Remove duplicated explanations that belong in Concepts or API reference.
- [ ] Update examples and links for the current constructor API.
- [ ] Build the docs site and run `go test ./...`.
- [ ] Commit with `docs: align application guides with runtime concepts`.

### Task 6: Clean examples, reference pages, and final validation

**Files:**
- Modify: `docs/examples/index.md`
- Modify: `docs/reference/index.md`
- Modify: `docs/reference/*/index.md` only where package purpose or imports are stale
- Modify: all remaining docs files with stale links or constructor names

**Interfaces:**
- Consumes all previous content changes.
- Produces the final coherent docs site and validation report.

- [ ] Make the examples page explain when to use each runnable example and link to the relevant concepts and guides.
- [ ] Keep API reference pages concise and package-focused.
- [ ] Confirm package reference pages do not contain empty code fences.
- [ ] Search all docs for `NewSchema`, obsolete `graph.New`, roadmap links, and em dashes.
- [ ] Validate internal Markdown links and heading anchors.
- [ ] Run `make docs-content` if required by the repository workflow.
- [ ] Run `bun run build` from `docs/`.
- [ ] Run `go test ./...`.
- [ ] Review the generated navigation and top-level page openings.
- [ ] Commit with `docs: finish documentation rewrite`.
