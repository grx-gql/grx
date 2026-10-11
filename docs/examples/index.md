---
title: Examples
description: Copy-paste starters, subscription wiring, router mounts - with the full Go source on the page.
outline: deep
---

# Examples

In this section you’ll find **the same Go that ships in `examples/`** - ready to skim, paste, or tweak - plus **tiny router mounts** for production-style URLs.

::: tip Runs from repo root  
The commands assume your working directory is a clone of **`github.com/grx-gql/grx`**. Prefer your own module path when you copy snippets into another module (everything under **`examples/*/`** is importable paths from this repo).  
:::

---

## Starter servers {#examples-starters}

### Basic GraphQL HTTP server {#examples-basic-http}

GraphiQL at **`GET /`**, **`POST /graphql`** for queries and mutations - the smallest complete loop.

#### `examples/basic/graph/schema.go`

```go
package graph

import "github.com/grx-gql/grx/schema"

type Query struct{}

func Schema() schema.Config {
	return schema.Config{
		Query: Query{},
	}
}
```

#### `examples/basic/graph/user.go`

```go
package graph

import (
	"context"
)

type User struct {
	ID    string  `gql:"id,nonNull"`
	Name  string  `gql:"name,nonNull"`
	Email *string `gql:"email"`
}

type UserArgs struct {
	ID string `gql:"id,nonNull"`
}

func (Query) User(ctx context.Context, args UserArgs) (*User, error) {
	email := "ada@example.com"
	return &User{ID: args.ID, Name: "Ada Lovelace", Email: &email}, nil
}
```

#### `examples/basic/main.go`

```go
package main

import (
	"log"
	"net/http"

	"github.com/grx-gql/grx"
	"github.com/grx-gql/grx/examples/basic/graph"
)

func main() {
	srv, err := grx.NewServer(
		grx.WithSchema(graph.Schema()),
		grx.WithPlaygroundPath("/"),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("GraphQL playground: http://localhost:4000/")
	log.Println("GraphQL endpoint: POST http://localhost:4000/graphql")
	log.Fatal(http.ListenAndServe(":4000", srv))
}
```

```bash
go run ./examples/basic
```

Try inside GraphiQL:

```graphql
{
  user(id: "user_1") {
    id
    name
    email
  }
}
```

**Next concepts:** [**Get started**](/getting-started/) · [**Queries and mutations**](/guides/query-mutation-server)

---

### Bearer token + field authorizer {#examples-auth}

Shows how host HTTP middleware turns `Authorization: Bearer …` into a request-scoped **subject**, and how **`WithFieldAuthorizer`** blocks `viewer` until that subject exists - the same narrative as [**Authentication & authorization**](/guides/auth). Runs on **`http://localhost:4010/`** with playground at **`/`**.

#### `examples/auth/session/context.go`

```go
package session

import (
	"context"
)

type subjectKey struct{}

func ContextWithSubject(ctx context.Context, subject string) context.Context {
	return context.WithValue(ctx, subjectKey{}, subject)
}

func Subject(ctx context.Context) (string, bool) {
	s, ok := ctx.Value(subjectKey{}).(string)
	if !ok || s == "" {
		return "", false
	}
	return s, true
}
```

#### `examples/auth/graph/schema.go`

```go
package graph

import (
	"context"
	"errors"

	"github.com/grx-gql/grx/examples/auth/session"
	"github.com/grx-gql/grx/schema"
)

type Query struct{}

type User struct {
	ID          string `gql:"id,nonNull"`
	DisplayName string `gql:"displayName,nonNull"`
}

func Schema() schema.Config {
	return schema.Config{
		Query: Query{},
	}
}

func (Query) Ping() string {
	return "ok"
}

func (Query) Viewer(ctx context.Context) (*User, error) {
	sub, ok := session.Subject(ctx)
	if !ok {
		return nil, errors.New("unauthenticated")
	}
	return &User{ID: sub, DisplayName: "Signed-in subject " + sub}, nil
}
```

#### `examples/auth/main.go`

```go
package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/grx-gql/grx"
	"github.com/grx-gql/grx/examples/auth/graph"
	"github.com/grx-gql/grx/examples/auth/session"
)

const demoBearerToken = "demo-alice-token"

func parseBearer(header string) (token string, ok bool) {
	parts := strings.Fields(strings.TrimSpace(header))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	return parts[1], true
}

func bearerAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		token, hasBearerScheme := parseBearer(r.Header.Get("Authorization"))
		if hasBearerScheme {
			switch token {
			case demoBearerToken:
				ctx = session.ContextWithSubject(ctx, "alice")
			case "":
				http.Error(w, `invalid Authorization header`, http.StatusBadRequest)
				return
			default:
				http.Error(w, `invalid bearer token`, http.StatusUnauthorized)
				return
			}
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func requireViewerField() grx.FieldAuthorizer {
	return func(ctx context.Context, fc grx.FieldAuthorizationContext) error {
		if fc.ParentType != "Query" || fc.FieldName != "viewer" {
			return nil
		}
		if _, ok := session.Subject(ctx); !ok {
			return fmt.Errorf("viewer requires Authorization: Bearer %s", demoBearerToken)
		}
		return nil
	}
}

func main() {
	srv, err := grx.NewServer(
		grx.WithSchema(graph.Schema()),
		grx.WithPlaygroundPath("/"),
		grx.WithFieldAuthorizer(requireViewerField()),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Println(`GraphQL playground: http://localhost:4010/`)
	log.Println(`For { viewer { id } } add header Authorization: Bearer ` + demoBearerToken)
	log.Fatal(http.ListenAndServe(":4010", bearerAuth(srv)))
}
```

```bash
go run ./examples/auth
```

GraphiQL **Headers** pane:

```json
{ "Authorization": "Bearer demo-alice-token" }
```

Then query:

```graphql
{
  viewer {
    id
    displayName
  }
}
```

**Guide:** [**Authentication & authorization**](/guides/auth)

---

### Subscriptions (WebSocket + SSE) {#examples-subscriptions}

Memory pub/sub feeds [**`graphql-transport-ws`**](https://github.com/graphql/graphql-ws) - plus bundled SSE. Configure browser CORS at the host HTTP layer as shown in [**CORS & browsers**](/guides/cors-browsers).

#### `examples/subscriptions/main.go`

<<< @/../examples/subscriptions/main.go

With `graph.Schema(nil)`, only queries and `createPost` are exposed. Passing a bus with
`graph.Schema(bus)` also exposes `createUser`, `postMessage`, and the subscription root.
The snippets below are included from the tested example source.

#### `examples/subscriptions/graph/schema.go`

<<< @/../examples/subscriptions/graph/schema.go

#### `examples/subscriptions/graph/user.go`

<<< @/../examples/subscriptions/graph/user.go

Post provides queries and non-event mutations. Message publishes JSON payloads to a separate topic for each room; its subscription uses the same topic name.

#### `examples/subscriptions/graph/post.go`

<<< @/../examples/subscriptions/graph/post.go

#### `examples/subscriptions/graph/message.go`

<<< @/../examples/subscriptions/graph/message.go

```bash
go run ./examples/subscriptions
```

**Dig deeper:** [**Realtime subscriptions**](/guides/subscriptions)

---

## Router integrations {#examples-router-integrations}

`grx` answers **`ServeHTTP`** with exact path matches - you usually pair **`http.StripPrefix("/api", gql)`** with **`WithPlaygroundPath("/playground")`** and **`WithGraphQLPath("/query")`** so callers see **`/api/playground`** and **`POST /api/query`**.

Build **`graph`** any way you like (see **[Get started**](/getting-started/) or **Basic HTTP** above), then reuse this fragment before each snippet:

```go
	gql, err := grx.NewServer(
		grx.WithSchema(graph.Schema()),
		grx.WithPlaygroundPath("/playground"),
		grx.WithGraphQLPath("/query"),
	)
	if err != nil {
		log.Fatal(err)
	}
	delegated := http.StripPrefix("/api", gql)
```

**Long-form guides:** [**net/http ServeMux**](/getting-started/servemux) · [**Chi**](/getting-started/chi) · [**Gin**](/getting-started/gin) · [**Echo**](/getting-started/echo) · [**Fiber**](/getting-started/fiber)

The **Gin**, **Echo**, and **Fiber** excerpts below reuse **`delegated`** from the fragment above. **Chi** repeats the full **`grx.NewServer`** block for copy-paste convenience.

---

### net/http `ServeMux` {#examples-router-servemux}

```go
mux := http.NewServeMux()
mux.Handle("/health", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
	w.Write([]byte("ok"))
}))
mux.Handle("/api/", delegated)
```

---

### Chi {#examples-router-chi}

```bash
go get github.com/go-chi/chi/v5
```

```go
package main

import (
	"log"
	"net/http"

	"example.com/hello-grx/graph"
	"github.com/go-chi/chi/v5"
	"github.com/grx-gql/grx"
)

func main() {
	gql, err := grx.NewServer(
		grx.WithSchema(graph.Schema()),
		grx.WithPlaygroundPath("/playground"),
		grx.WithGraphQLPath("/query"),
	)
	if err != nil {
		log.Fatal(err)
	}

	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) { w.Write([]byte("ok")) })
	r.Mount("/api", http.StripPrefix("/api", gql))

	log.Fatal(http.ListenAndServe(":8080", r))
}
```

---

### Gin {#examples-router-gin}

```bash
go get github.com/gin-gonic/gin
```

```go
	engine := gin.Default()
	engine.GET("/health", func(c *gin.Context) { c.String(200, "ok") })
	engine.Any("/api/*proxyPath", gin.WrapH(delegated))
	log.Fatal(engine.Run(":8080"))
```

---

### Echo {#examples-router-echo}

```bash
go get github.com/labstack/echo/v4
```

```go
	e := echo.New()
	e.GET("/health", func(c echo.Context) error { return c.String(200, "ok") })
	e.Any("/api/*", echo.WrapHandler(delegated))
	log.Fatal(e.Start(":8080"))
```

---

### Fiber {#examples-router-fiber}

```bash
go get github.com/gofiber/fiber/v2
```

```go
	app := fiber.New()
	app.Get("/health", func(c *fiber.Ctx) error { return c.SendString("ok") })
	app.All("/api/*", adaptor.HTTPHandler(delegated))
	log.Fatal(app.Listen(":8080"))
```

(import **`github.com/gofiber/fiber/v2`** and **`github.com/gofiber/fiber/v2/middleware/adaptor`**)

---

## Measuring throughput {#examples-benchmarks}

Micro-benchmarks help compare resolver hot paths - they are **not** a substitute for profiling your full HTTP/RPS workload.

See **[Benchmarks](/benchmarks)** for scenarios, methodology, and latest tables.

---

## Source {#examples-source}

- [**GitHub repository**](https://github.com/grx-gql/grx)  -  history, CI, **`examples/`** tree mirroring these snippets.
- [**Discussions**](https://github.com/grx-gql/grx/discussions)  -  integrations, roadmap questions.

---

## Doc cross-links {#examples-more-docs}

- [**Define your schema**](/concepts/schema-basics)
- [**Organize larger codebases**](/concepts/schema-mapping)
- [**Authentication & authorization**](/guides/auth)
- [**Custom transports**](/guides/custom-transport) & [**Plugins**](/guides/custom-plugin)
- [**How it fits together**](/concepts/architecture)
