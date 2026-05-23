---
title: CORS & browsers
description: Serving GraphQL and GraphiQL from browser origins—grx.Cors plus matching WebSocket origin checks for subscriptions.
outline: deep
---

# CORS & browsers

SPA **GraphQL** traffic combines **`credentials`**, headers like **`Authorization`**, **`OPTIONS` preflight**, and—for subscriptions—WebSocket **`Origin`** checks. That lives in **HTTP middleware** and **`websocket.Transport`**, not resolvers alone.

[`grx.Cors`](https://pkg.go.dev/github.com/patrickkabwe/grx#Cors) attaches standard CORS middleware and can align with websocket origin policies when transports share configs.

Minimal pattern:

```go
package main

import (
	"log"
	"net/http"

	"example.com/hello-grx/graph"

	"github.com/patrickkabwe/grx"
)

func main() {
	srv, err := grx.NewServer(
		grx.WithSchema(graph.NewSchema()),
		grx.WithPlaygroundPath("/"),
		grx.WithMiddleware(grx.Cors(grx.CorsConfig{
			AllowedOrigins:   []string{"https://app.example.com"},
			AllowedMethods:   []string{http.MethodPost, http.MethodGet, http.MethodOptions},
			AllowedHeaders:   []string{"Content-Type", "Authorization"},
			AllowCredentials: true,
		})),
	)
	if err != nil {
		log.Fatal(err)
	}
	_ = srv
}
```

Prefer explicit origins over `*` whenever **`AllowCredentials`** is **`true`** (browser enforced).

::: tip Companion doc  
[**Authentication & authorization**](/guides/auth) expands Bearer propagation through middleware before **`executor.Execute`** runs.

:::

## Related

- [**Realtime subscriptions**](/guides/subscriptions) — **`websocket.Config.CheckOrigin`** and idle timeouts alongside CORS tuning.
