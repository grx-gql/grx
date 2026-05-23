---
title: Gin
description: Forward GraphQL requests through Gin with wildcard routes and gin.WrapH.
outline: [2, 3]
---

# Gin

[**Gin**](https://gin-gonic.com/) pairs well when your service already declares REST handlers with **`gin.Context`** helpers—you keep JSON validation, JWT middleware, telemetry, and **`http.Handler`** fallbacks aligned in one **`Engine`**.

`grx` stays a plain **`http.Handler`**; **`gin.WrapH`** lets Gin emit the response while **`http.StripPrefix`** keeps `/api`-visible URLs routed to **`/playground`** and **`/query`**.

Prep **`graph`** using **[Minimal schema](/getting-started/#minimal-schema)**.

## Dependencies

```bash
go get github.com/gin-gonic/gin
```

## `main.go`

```go
package main

import (
	"log"
	"net/http"

	"example.com/hello-grx/graph"
	"github.com/gin-gonic/gin"
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

	engine := gin.Default()
	engine.GET("/health", func(c *gin.Context) {
		c.String(200, "ok")
	})
	engine.Any("/api/*proxyPath", gin.WrapH(delegated))

	log.Println("GraphiQL: http://localhost:8080/api/playground")
	log.Println("GraphQL POST: http://localhost:8080/api/query")
	log.Fatal(engine.Run(":8080"))
}
```

Prefer composition over rewriting headers—wrap **`delegated`** (logging, JWT gating, request IDs) instead of patching Gin-specific hooks when possible.

---

## Related

- **[Get started](/getting-started/)** · **[Echo](./echo)** · **[Chi](./chi)**
