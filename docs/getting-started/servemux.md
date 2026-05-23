---
title: net/http ServeMux
description: Mount grx beside health checks or REST handlers using the Go 1.22+ ServeMux patterns.
outline: [2, 3]
---

# net/http ServeMux

`ServeMux` is enough when GraphQL joins other plain handlers on **one listener**—readiness probes, internal JSON routes, Prometheus scrapes—with zero third-party routers.

Assume you reproduced **[Minimal schema](/getting-started/#minimal-schema)**. The pattern below configures inner routes with **`WithPlaygroundPath("/playground")`** and **`WithGraphQLPath("/query")`**; see the **[full **`main`**](#wire-servemux)** for imports.

## Wire `ServeMux`

```go
package main

import (
	"log"
	"net/http"

	"example.com/hello-grx/graph"
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

	mux := http.NewServeMux()
	mux.Handle("/health", http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok"))
	}))
	mux.Handle("/api/", delegated)

	log.Println("GraphiQL: http://localhost:8080/api/playground")
	log.Println("GraphQL POST: http://localhost:8080/api/query")
	log.Fatal(http.ListenAndServe(":8080", mux))
}
```

`/api/playground` and **`POST /api/query`** are the externally visible URLs—the handler still sees **`/playground`** and **`/query`**.

::: tip Middleware  
Wrap **`mux`** (`http.Handler`) with timeouts, JWT verification, telemetry, gzip—GraphQL behaves like any subtree.  
:::

---

## Related

- **[Chi](./chi)** · **[Gin](./gin)** · **[Echo](./echo)** for richer routing ergonomics  
- **[Get started overview](./)**
