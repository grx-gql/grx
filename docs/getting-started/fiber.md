---
title: Fiber
description: Bridge FastHTTP with grx via the built-in Fiber v2 adaptor.
outline: [2, 3]
---

# Fiber

[**Fiber**](https://gofiber.io/) sits on **`fasthttp`**, so **`net/http`** handlers - including **`grx.Server`** - go through **`github.com/gofiber/fiber/v2/middleware/adaptor`**.

Build **`graph`** from **[Minimal schema](/getting-started/#minimal-schema)** once, then reuse the same **`StripPrefix`** pattern as Gin or Echo:

## Dependencies

```bash
go get github.com/gofiber/fiber/v2
```

## `main.go`

```go
package main

import (
	"log"
	"net/http"

	"example.com/hello-grx/graph"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/adaptor"
	"github.com/grx-gql/grx"
)

func main() {
	gql, err := grx.NewServer(
		grx.WithSchema(graph.NewSchema()),
		grx.WithPlaygroundPath("/playground"),
		grx.WithGraphQLPath("/query"),
	)
	if err != nil {
		log.Fatal(err)
	}

	delegated := http.StripPrefix("/api", gql)

	app := fiber.New()
	app.Get("/health", func(c *fiber.Ctx) error {
		return c.SendString("ok")
	})
	app.All("/api/*", adaptor.HTTPHandler(delegated))

	log.Println("GraphiQL: http://localhost:8080/api/playground")
	log.Println("GraphQL POST: http://localhost:8080/api/query")
	log.Fatal(app.Listen(":8080"))
}
```

`adaptor.HTTPHandler` runs the **`http.Handler`** inside Fiber’s lifecycle; **`StripPrefix`** must still peel **`/api`** before GraphQL executes so paths match **`WithPlaygroundPath` / WithGraphQLPath** exactly.

Prefer Fiber v3? Imports move to **`github.com/gofiber/fiber/v3/middleware/adaptor`** with equivalent helpers - see Fiber’s adaptor docs.

---

## Related

- **[Get started](/getting-started/)** · **[Examples › Fiber snippet](/examples/#examples-router-fiber)**
