---
title: Basic · net/http
description: Run GraphQL on a standalone listener—the smallest way to inspect grx wiring.
outline: [2, 3]
---

# Basic · net/http

Best when GraphQL deserves its own TCP port (local tools, playgrounds, gateways that reverse-proxy cleanly). **`grx.Server`** answers **`GET`** for bundled GraphiQL and **`POST /graphql`** for JSON bodies.

Reuse the **`graph`** package from **[Get started › Minimal schema](/getting-started/#minimal-schema)**.

## `main.go`

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
		grx.WithPlaygroundPath("/"),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("GraphiQL playground: http://localhost:4000/")
	log.Println("GraphQL POST: http://localhost:4000/graphql")
	log.Fatal(http.ListenAndServe(":4000", gql))
}
```

## Run

```bash
go run .
```

Open `http://localhost:4000` and paste the **[sample query](/getting-started/#try-query)**.

::: tip Need REST on the same process?  
See **[ServeMux](./servemux)** or pick **[Chi](./chi)** · **[Gin](./gin)** · **[Echo](./echo)**.  
:::

---

## Related

- **[Examples › Basic HTTP](../examples/#examples-basic-http)** — runnable repo copy  
- **[Transports conceptual doc](/concepts/transports)**
