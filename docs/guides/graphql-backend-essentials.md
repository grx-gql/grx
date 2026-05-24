---
title: GraphQL server essentials
description: Answers newcomers’ top questions - what GraphQL is, how schemas and resolvers work, and where grx fits - before you touch a router integration.
outline: deep
---

# GraphQL server essentials

If you are **new to GraphQL** or spinning up **your first backend**, this page collects the questions people search for most (“what is X?”, “do I need Y?”) and points you to **grx-specific guides** afterward. Prefer hands-on steps first? [**Run your first server**](/getting-started/) and treat this doc as backup reading.

Official background when you want the spec voice: [**GraphQL Learn**](https://graphql.org/learn/) and [**FAQ**](https://graphql.org/faq/).

---

## What is GraphQL - and how is it different from REST?

**GraphQL is a specification** for querying and changing data over the network using a single **typed** API layer. Clients send a **single document** (query, mutation, or subscription operation) describing the shape of data they want; the server returns JSON that mirrors that shape.

Compared to typical REST:

| Topic | REST (often) | GraphQL |
| --- | --- | --- |
| **Requests** | Many endpoints (`/users`, `/users/1/posts`, …) | Usually **one HTTP URL**; operation type encoded in the body |
| **Payload** | Fixed server-defined responses | Clients choose fields - **less over-fetching** |
| **Versioning** | `/v2` prefixes or duplicate routes | Prefer **additive** schema changes |

GraphQL says **nothing** about your database - you implement how data is loaded (SQL, RPCs, other HTTP APIs).

**In grx:** you expose an `http.Handler` (see [**Routers & transports**](/concepts/transports) and [**Get started**](/getting-started/)).

---

## What do I need to actually build a GraphQL server?

Every conforming GraphQL server needs:

1. A **schema**  -  the typed contract your clients can legally request (`Query`, and optionally `Mutation` / `Subscription`, plus object types and scalars).
2. **Resolution logic**  -  code that fulfills each requested field when a document runs (**resolvers**, expressed in Go methods in grx).
3. **A transport**  -  HTTP (POST with JSON body is the usual), and optionally WebSocket/SSE for subscriptions.

Missing any of those, clients cannot reliably talk to your service.

---

## What is a schema - and do I write SDL by hand?

A **schema** is the authoritative description of types and fields GraphQL exposes. Teams often describe it using **SDL** (Schema Definition Language), the text form you see in tutorials (`type Query { hello: String }`).

Common workflow labels:

| Style | Roughly |
| --- | --- |
| **SDL / schema‑first** | Write `.graphql`, then attach resolvers. |
| **Code‑first** | Define types and fields in programming language primitives; SDL can be emitted or inferred. |

Either way the **runtime artifact** at execute time is the same pairing: validated schema graph + resolver functions.

**In grx:** schemas are authored from **exported Go structs and methods**, with **`gql` struct tags** for wire naming and optionality - not a handwritten SDL drift problem. Concepts: [**Define your schema**](/concepts/schema-basics) · [**Organize your code**](/concepts/schema-mapping). Clients still see a normal GraphQL API (including **[introspection](#what-is-introspection-and-should-it-stay-on-in-production)** when enabled).

---

## What are Query, Mutation, and Subscription?

These are GraphQL **operation types** mapped to **`Query`, `Mutation`, and `Subscription` root types** in the schema:

| Operation | Typical use | Writes data? |
| --- | --- | --- |
| **Query** | Read tree-shaped data idempotently | No |
| **Mutation** | Create/update/delete workflows | Usually yes |
| **Subscription** | Long-lived stream of server-pushed payloads | Depends on domain |

Naming note: **`query` lowercase** refers to HTTP / operation kind; **`type Query`** is the schema root for read operations - easy to confuse in docs.

**In grx:** attach structs to **`schema.Config`** roots and implement methods - see [**Get started**](/getting-started/), [**Queries and mutations**](/guides/query-mutation-server), [**Realtime subscriptions**](/guides/subscriptions).

---

## What is a resolver?

A **resolver** is the logic that yields the runtime value for a **single schema field**. Execution walks the AST of the client operation; for each field the engine calls **parent type → field** resolution (conceptually asynchronous).

From the [**GraphQL execution overview**](https://graphql.org/learn/execution/):

- Execution begins at **`Query`**, **`Mutation`**, or **`Subscription`** roots.
- Resolvers commonly receive **`context`**, **`args`**, **`parent`** (object value), plus metadata - grx aligns with **`context.Context`** and typed structs for arguments.

Thin resolvers delegating to a **service/store layer** scales better than fat data access sprinkled everywhere.

---

## Does GraphQL call my SQL database automatically?

No. GraphQL resolves **graphs of fields**. **You** connect to Postgres, DynamoDB, gRPC backends, caches, whatever your resolvers orchestrate - the spec deliberately stays datasource-agnostic (see **[GraphQL FAQ](https://graphql.org/faq/)**).

Patterns that help:

| Concern | What teams do |
| --- | --- |
| **Maintainability** | Services/repositories invoked from resolver methods - not raw SQL sprinkled ad hoc in every resolver. |
| **`N+1` queries** | One query loads a list, then **one DB round-trip per row** for nested relation fields - pain at scale - see next section. |
| **Per-request scope** | **Batch loaders**, cache keys, tracing IDs - normally **constructed per HTTP request**, not globals. |

**In Go:** idiomatic layering + optional DataLoader-like batching libs for your datastore; wire **identity** via `context` (**[Authentication & authorization](/guides/auth)**).

---

## What is the “N+1 problem”?

Classical symptom: **`users { posts { title } }`** runs **`1 + N`** database queries because each user’s **`posts`** field triggers its own resolver and DB hit.

Mitigations backend teams rely on:

- **Batch loaders** keyed per **request**.
- Smart **JOINs**, **preload**, or dataload patterns in repositories.
- **Observability** over resolver timings and SQL audit - catch explosions early.

Nothing in vanilla GraphQL “magically batches” datastore calls - you design for it once nested fields become hot paths.

---

## What is introspection - and should it stay on in production?

**Introspection** lets clients query the schema itself (`__schema`, types, fields, directives) - how GraphiQL discovers your surface.

Benefits:

- Excellent **DX** locally and in staging tools.
- **Codegen** pipelines (clients, mocks).

Operational tension:

Public internet facing APIs sometimes **disable introspection**, add **persisted queries**, or gate playground traffic.

**Decision factors:** attacker reconnaissance versus internal convenience. Tune staging vs production.

**With grx  -  disable introspection:** call **`grx.WithDisableIntrospection()`**. Related hardening reads: **[Security](/guides/production-security)** (masking, authorizers), **[Introspection](/guides/introspection)** (playground, **`__schema`**), **[Limits](/guides/request-limits)** (payloads, timeouts, persisted‑query modes) - or the **[overview hub](/concepts/graphql-security-production)**.

---

## How do authentication and authorization work?

GraphQL is usually **one URL**, so **`Authorization`**, cookies, mTLS identity, etc., land in **transport middleware**. Your resolvers - or authorizer hooks - read **trusted identity off `context`**.

Rough split:

| Layer | Responsibility |
| --- | --- |
| **Authenticate** HTTP | Parse tokens, rotate sessions **before** `Execute`. |
| **Authorize** GraphQL operations or fields | Reject forbidden shapes or fields after identity is known |

**Deep dive:** [**Authentication & authorization**](/guides/auth).

---

## How do partial success and errors work?

GraphQL distinguishes **transport errors** (`4xx`, `5xx`) from **GraphQL `errors`** in the execution response while still returning **`data`** for whichever branches succeeded. Design mutation payloads (**`payload`/`errors`** union patterns) consciously so clients behave predictably - the spec describes error locations and formatting.

Treat **resolver `error`** returns as intentional API surface: log, categorize, expose **opaque client messages** externally when security matters.

---

## One schema or many - “federation”?

For **clients**, one stable graph avoids orchestrating multiple subgraphs manually. Organizations either:

- Maintain a **single monolith schema**, or
- Compose **schemas across services** (federation) - more moving parts than most small teams need initially.

Start monolithic until **team boundaries force** otherwise.

---

## Where to go next in **grx**

| Goal | Next page |
| --- | --- |
| What ships in the box | [**What grx is**](/concepts/what-is-grx) |
| Run Hello World + routers | [**Get started**](/getting-started/) |
| Structs ↔ GraphQL modeling | [**Define your schema**](/concepts/schema-basics) |
| Multipart file mutations | [**File uploads**](/guides/file-upload) |
| Persisted / trusted ops | [**Persisted queries (APQ)**](/guides/persisted-queries) |
| Multi-module layout | [**Organize your code**](/concepts/schema-mapping) |
| CRUD-ish API wiring | [**Queries and mutations**](/guides/query-mutation-server) |
| Realtime streams | [**Subscriptions**](/guides/subscriptions) |
| Middleware + guards | [**Authentication & authorization**](/guides/auth) |
| Ops hardening (introspection off, payloads, masking) | [**Security**](/guides/production-security), [**Introspection**](/guides/introspection), [**Limits**](/guides/request-limits) |
| Internal mental model | [**Architecture**](/concepts/architecture) |
| Migrate from older Go servers | [**Migrate**](/guides/migrate/) |

You do **not** have to memorize the spec upfront - bookmark this page when someone on your team asks “wait, why is GraphQL …?” mid-sprint.
