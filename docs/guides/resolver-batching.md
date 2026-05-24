---
title: Resolver batching & N+1
description: Nested field execution and datastore round-trips - mitigations before a built-in loader lands in grx.
outline: deep
---

# Resolver batching & the **N+1** problem

GraphQL invokes resolvers independently. A query like **`users { posts { title } }`** can become **one query for users** and **one per user for posts** if each **`posts`** field resolver hits the database alone.

## What grx provides today

**No first-class batch loader hook** sits inside **`exec`** yet (**[Roadmap](/roadmap)** tracks DataLoader-style support). Until it ships:

- Implement **repository batch APIs** keyed per HTTP request (**`context`‑scoped** maps, OSS Go DataLoader-style helpers).
- Prefer **`IN`/`JOIN`** preloads where query shapes repeat.
- **Instrument** resolver timing + DB statements (`plugin` hooks **or** APM wrappers).

::: tip Separate concern from selection limits  

**Document-shape caps** stop pathological parses but **do not** consolidate SQL - combine with this guide’s data-access strategies.

:::

## See also

- **[Graph fundamentals (incl. N+1)](/guides/graphql-backend-essentials)**
- **[Query & document limits](/guides/query-limits)**
