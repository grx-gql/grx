---
title: Chi
description: Mount grx on Chi routers with explicit prefix stripping alongside REST routes.
outline: [2, 3]
---

# Chi

[**Chi**](https://github.com/go-chi/chi) keeps routing orthogonal to middleware—handy when REST and GraphQL share auth, traces, quotas, etc. **`r.Mount("/api", http.StripPrefix("/api", gql))`** trims the public prefix before **`grx.Server`** evaluates **`WithPlaygroundPath("/playground")`** and **`WithGraphQLPath("/query")`**, identical to **[ServeMux](./servemux)** but with Chi ergonomics.

Build **`graph`** from **[Minimal schema](/getting-started/#minimal-schema)** once.

## Dependencies

```bash
go get github.com/go-chi/chi/v5
```

## `main.go`

```go
package main

import (
	"log"
	"net/http"

	"example.com/hello-grx/graph"
	"github.com/go-chi/chi/v5"
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

	r := chi.NewRouter()
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("ok"))
	})

	r.Mount("/api", http.StripPrefix("/api", gql))

	log.Println("GraphiQL: http://localhost:8080/api/playground")
	log.Println("GraphQL POST: http://localhost:8080/api/query")
	log.Fatal(http.ListenAndServe(":8080", r))
}
```

Add **`r.Use(...)`** middleware before **`Mount`** to share behaviour across subtrees.

---

## Related

- **[Get started](/getting-started/)** · **[Gin](./gin)** · **[Echo](./echo)**
