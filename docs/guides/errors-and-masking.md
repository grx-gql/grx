---
title: Errors & client responses
description: Field errors versus transport failures, controlling messages reaching clients with core.Error versus MaskInternalErrors.
outline: deep
---

# Errors & client responses

GraphQL combines **`data`** and **`errors`**: callers may receive **partial success** (`data` subtree non-null siblings while flagged fields **`null`** with **`errors[].path`**). Transports distinguish **422/400-ish protocol errors** versus **HTTP 200 envelopes** carrying **`errors`** for execution problems.

### Resolver returns

Return **`(, error)`** from resolvers - the executor maps **`error`** strings into **`[core.Error](https://pkg.go.dev/github.com/grx-gql/grx/core#Error)`** with stable **`path` segments**. When you need **exact messaging** attach **`&core.Error{ Message:, Path:, … }`** (**[Resolvers → Errors](/concepts/resolvers#errors)** deep dive).

::: warning Don’t leak stack traces  
Database driver text, JWT parsing failures, **`fmt.Errorf("%w")` chains exposing SQL** belong in observability backends - not raw **`errors[].message`** for anonymous clients. Prefer **`MaskInternalErrors`** on **`server.New(server.Config{…})`** and pair with **`zap`/OTEL** exporters off plugins - **[Security → Mask internal errors](/guides/production-security#mask-internal-errors)**.

:::

### What **`MaskInternalErrors`** changes

Configured through **`server.Config`**, masking swaps internal resolver/plugin/panic fallout for a deterministic client string while richer detail still reaches **`plugins.Plugin.Error`** hooks for logging dashboards.

::: tip Designing mutation payloads  
For CRUD ergonomics expose **`MutationResult { ok user userErrors validationErrors }`** style unions consciously - GraphQL **`errors`** are transport-level signalling, not necessarily your UX domain **`errors`** field.

:::

## See also

- **[Resolver semantics](/concepts/resolvers#errors)**  -  **`core.Error` examples
- **[Security → masking](/guides/production-security#mask-internal-errors)**
