---
title: Echo
description: Bridge Echo routes to grx with echo.WrapHandler and wildcard paths.
outline: [2, 3]
---

# Echo

[**Echo**](https://echo.labstack.com/) targets teams who prefer the lightweight handler signature (`echo.HandlerFunc`) alongside batteries-included middleware. GraphQL behaves like another subtree: **`echo.WrapHandler`** adapts **`http.Handler`**, **`http.StripPrefix`** aligns external **`/api/...`** routes with **`grx`'s configured inner paths.**

Start from **[Minimal schema](/getting-started/#minimal-schema)** for **`graph`**.

## Dependencies

```bash
go get github.com/labstack/echo/v4
```

## `main.go`

```go
package main

import (
	"log"
	"net/http"

	"example.com/hello-grx/graph"
	"github.com/labstack/echo/v4"
	"github.com/patrickkabwe/grx"
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

	e := echo.New()
	e.GET("/health", func(c echo.Context) error {
		return c.String(200, "ok")
	})
	e.Any("/api/*", echo.WrapHandler(delegated))

	log.Println("GraphiQL: http://localhost:8080/api/playground")
	log.Println("GraphQL POST: http://localhost:8080/api/query")
	log.Fatal(e.Start(":8080"))
}
```

Register global middleware (**`e.Use`**) for auth/logging before attaching GraphQL—the wildcard route inherits whatever runs earlier in the Echo chain.

---

## Related

- **[Get started](/getting-started/)** · **[Gin](./gin)** · **[Fiber](./fiber)**
