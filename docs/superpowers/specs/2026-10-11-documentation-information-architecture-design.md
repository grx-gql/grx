# grx Documentation Information Architecture Design

## Goal

Rewrite the grx documentation so a Go developer can move from product fit, to a working server, to core concepts, to framework integrations, to focused production guides, without repeated explanations or unclear navigation.

## Audience

- Go developers evaluating GraphQL server libraries.
- Go developers building their first grx service.
- Developers integrating grx with an existing HTTP router.
- Developers operating GraphQL services in production.

## Writing rules

- Use simple English with precise technical terms.
- Avoid em dashes.
- Lead each page with its purpose and expected outcome.
- Explain one concept before depending on it in another concept.
- Prefer short paragraphs, direct headings, and runnable examples.
- Keep API details in the API reference instead of repeating them in guides.
- Link to the next useful page at the end of each major section.

## Information architecture

### 1. Start here

The first reading path gives the reader a working result quickly.

Contents:

- What grx is and the main feature surface.
- Why choose grx, including tradeoffs and cases where another approach is better.
- Installation and a minimal HTTP server.
- The code-first schema model.
- A complete request flow from HTTP request to resolver response.

The quickstart remains available without requiring the reader to study the product rationale first.

The product overview must explain the boundary between grx and the host HTTP
application. grx handles GraphQL schema building, execution, the default
HTTP transport, optional WebSocket and SSE transports, and GraphQL-specific
limits and hooks. The host application handles HTTP concerns around the
returned `http.Handler`, including CORS, authentication middleware, routing,
TLS, and non-GraphQL endpoints.

### 2. Concepts

Concept pages explain the model used by all integrations and guides.

Contents:

- What grx handles and what the host application handles.
- Schema mapping and root resolver types.
- Resolver methods and arguments.
- Execution and validation.
- Transports.
- Plugins and lifecycle hooks.
- Subscriptions and pub/sub.
- Runtime architecture.

The HTTP boundary concept must include a CORS example. It must explain that
grx does not provide a CORS package, show how to wrap the returned handler with
`rs/cors` for `net/http`, show the equivalent middleware placement for common
routers, and explain that WebSocket origin checks use
`websocket.Config.CheckOrigin` separately.

Each concept page should state its prerequisites and link to the next related concept.

### 3. Integrations

The current Getting Started integration group becomes Integrations. Existing URLs remain unchanged.

Contents:

- Basic `net/http`.
- Standard library `ServeMux`.
- Chi.
- Gin.
- Echo.
- Fiber.

These pages should focus on mounting the already-understood grx handler. Schema design and GraphQL concepts should link back to Start here or Concepts instead of being repeated in every integration page.

### 4. Guides

Guides answer focused implementation and production questions.

Groups:

- Query and mutation behavior.
- Subscriptions and pub/sub backends.
- Schema extensions such as custom scalars and directives.
- Security, authentication, limits, introspection, and errors.
- Middleware, CORS, uploads, persisted queries, and testing.
- Deployment and migration from other Go GraphQL libraries.

Each guide should define the problem, show the smallest useful implementation, explain production considerations, and link to the relevant reference package.

The CORS guide should begin with the request path through the host server,
then cover preflight requests, allowed origins, credentials, headers, router
middleware placement, and the separate WebSocket origin check. It must state
clearly which behavior comes from grx and which behavior comes from the host
framework.

### 5. Examples

Examples remain a separate collection of runnable applications. The examples page should explain when to use an example and link back to the concept or guide that explains the code.

### 6. API reference

The API reference contains package-level API documentation and short package purpose statements. It should not carry onboarding content or duplicate guides.

### 7. Internals

Runtime execution and subscription implementation details remain available for maintainers and advanced users, but they should not be part of the beginner path.

## Navigation changes

- Keep `Start here` as the first sidebar group.
- Add or expand a clear `Why grx` page linked from the home page and Start here.
- Rename the current `Getting Started` sidebar group to `Integrations`.
- Keep integration URLs stable under `/getting-started/` for this rewrite.
- Remove `Roadmap` from the docs sidebar and public docs content.
- Keep the repository root `ROADMAP.md` unchanged.
- Keep API reference pages grouped together.
- Move runtime-only pages into an `Under the hood` or equivalent advanced group.

## Content rewrite scope

- Rewrite the home page and onboarding pages for the new reading path.
- Rewrite duplicated introductions and transitions in concept pages.
- Rewrite integration introductions so they assume the reader already has a grx handler.
- Rewrite guide introductions and cross-links where the current order is unclear.
- Update examples and code snippets to use the current `Schema` constructor API.
- Remove stale roadmap links, sidebar entries, and roadmap landing content.
- Preserve technical behavior and existing stable page URLs unless a link target is clearly obsolete.

## Validation

- Search all docs for stale constructor names and removed roadmap links.
- Check internal links and headings for renamed pages.
- Run `bun run build` in `docs/`.
- Run `go test ./...` because examples are embedded in documentation and must stay valid.
- Review the built navigation order and the first paragraph of each top-level page.
