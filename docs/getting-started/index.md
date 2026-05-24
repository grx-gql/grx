---
title: Get started
description: Minimal GraphQL servers in plain Go - or mounted next to Chi, Gin, Echo, Fiber, and the standard library mux.
outline: deep
---

# Get started

::: tip Reading order  
1. New to **grx**? Skim **[What grx is](/concepts/what-is-grx)** once so advertised features (**HTTP**, subscriptions, authorizers…) map cleanly to packages.  
2. Scan **[Define your schema](/concepts/schema-basics)** so the **`gql` tags** beside each field slot into mental model quickly.  
3. Copy the snippets under [**Quick start**](#quick-start), then run `go run .`.  
4. Background reading - [**How GraphQL backends work**](/guides/graphql-backend-essentials).  
5. Harden and ship - [**Security**](/guides/production-security), [**Introspection**](/guides/introspection), [**Limits**](/guides/request-limits), then **[Deployment](/guides/deployment)** (Docker, reverse proxies, Kubernetes) when you leave localhost.

:::

**New here?** **[What grx is](/concepts/what-is-grx)** inventories the bundled surface (queries/mutations/subscriptions over HTTP/WebSocket/SSE, plugins, limits, transports). Short version:

> Declare types and resolver methods once, attach them to **`Query`**, **`Mutation`**, and **`Subscription`** (optional); the runtime materializes your executable GraphQL schema without obligatory SDL codegen or parallel handwritten type registers.

::: tip Audience  
Comfortable maintaining Go services? Jump to [**Quick Start**](#quick-start), paste the **`graph`** package, open the framework recipe you ship with (**[Chi](./chi)** · **[Gin](./gin)** · **[Echo](./echo)** · **[Fiber](./fiber)**), and return to [**Define your schema**](/concepts/schema-basics) when you graduate past the tutorial snippets. Already productive with GraphQL elsewhere? skim the wording here for how structs replace schema files inside this toolchain.  
:::

## Quick start {#quick-start}

```bash
mkdir hello-grx && cd hello-grx
go mod init example.com/hello-grx
go get github.com/grx-gql/grx@latest
```

Next: add **`graph/schema.go`** (below), **`main.go`**, **`go run .`**. Prefer a clone-and-run checklist? **`examples/basic/`** stays the quickest mirror of this lesson.

---

## Prerequisites

- Go **1.22 or newer**
- Terminal + editor

---

## Minimal schema {#minimal-schema}

Exported structs describe GraphQL objects. Methods on **`Query`** become top‑level **`query`** fields (`User` ⇒ GraphQL **`user`**). **`gql:"..."`** maps wire names, nullability, and tags so Go stays idiomatic.

Create **`graph/schema.go`**:

```go
package graph

import (
	"context"

	"github.com/grx-gql/grx/schema"
)

type User struct {
	ID   string `gql:"id,nonNull"`
	Name string `gql:"name,nonNull"`
}

type UserArgs struct {
	ID string `gql:"id,nonNull"`
}

type Query struct{}

func (Query) User(ctx context.Context, args UserArgs) (*User, error) {
	return &User{ID: args.ID, Name: "Ada Lovelace"}, nil
}

func NewSchema() schema.Config {
	return schema.Config{Query: Query{}}
}
```

<details>
<summary><strong>No standalone GraphQL SDL file?</strong></summary>

Many teams ship faster when structs author the canonical contract beside resolvers; SDL appears through introspection, docs, or export tooling whenever someone downstream needs text.

</details>

---

## Choose your framework {#pick-a-framework}

[**`grx.NewServer**](/reference/grx/) returns **`http.Handler`.** Decide how URLs reach that handler:

| Path | Typical use-case |
| --- | --- |
| [**Basic · `net/http`**](./basic-http) | Dedicated port, simplest mental model (`GET /` playground + `POST /graphql`). |
| [**`net/http` ServeMux**](./servemux) | Embed GraphQL beside REST probes on one listener. |
| [**Chi**](./chi) | Composable routers, lightweight middleware stacks. |
| [**Gin**](./gin) | High‑level HTTP API ergonomics (`gin.WrapH`). |
| [**Echo**](./echo) | Minimal surface area with first-party `WrapHandler` helpers. |
| [**Fiber**](./fiber) | `fasthttp` stack - bridge through Fibre’s adaptor to reach `ServeHTTP`. |

Every integration below shares:

1. **`WithPlaygroundPath("/playground")` + `WithGraphQLPath("/query")`**
2. **`http.StripPrefix("/api", gql)` so external URLs look like **`/api/playground`** · **`POST /api/query`** while internals stay `/playground`, `/query`.  
   Dedicated listeners keep defaults **`/`**, **`POST /graphql`**.

Authenticate, trace, throttle, inject metadata by wrapping **`http.Handler`** middleware around **`delegated`**.

::: tip Dependencies  
Install each router beside grx - for example **`go get github.com/go-chi/chi/v5`**, **`github.com/gin-gonic/gin`**, **`github.com/labstack/echo/v4`**, **`github.com/gofiber/fiber/v2`**, **`github.com/gofiber/adaptor/v2`**.  
:::

---

## Try the introspection-ready query {#try-query}

```graphql
{
  user(id: "1") {
    id
    name
  }
}
```

Run it inside GraphiQL once your chosen guide prints the playground URL.

---

## What's next {#whats-next}

- [**Define your schema**](/concepts/schema-basics)  -  lists, enums, unions, inputs.
- [**Organize your code**](/concepts/schema-mapping)  -  multi-file schemas without registrar glue.
- [**Queries & mutations**](/guides/query-mutation-server) · [**Subscriptions**](/guides/subscriptions)
- [**How it fits together**](/concepts/architecture) · [**Migrate**](/guides/migrate/)
