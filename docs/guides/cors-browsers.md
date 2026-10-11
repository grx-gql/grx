---
title: CORS & browsers
description: Configure browser CORS at the HTTP host layer around a grx GraphQL handler.
outline: deep
---

# CORS & browsers

CORS is HTTP policy, not GraphQL execution. `grx` returns an
`http.Handler`; configure CORS in the HTTP server or framework that owns that
handler. This keeps preflight requests, credentials, and non-GraphQL routes in
one host-level policy.

`grx` does not provide a CORS package. Use the middleware for your HTTP stack.

## `net/http`

The [`rs/cors`](https://github.com/rs/cors) package wraps any `http.Handler`:

```go
graphql, err := grx.NewServer(
	grx.WithSchema(graph.New()),
)
if err != nil {
	log.Fatal(err)
}

handler := cors.New(cors.Options{
	AllowedOrigins:   []string{"https://app.example.com"},
	AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
	AllowedHeaders:   []string{"Content-Type", "Authorization"},
	AllowCredentials: true,
}).Handler(graphql)

log.Fatal(http.ListenAndServe(":4000", handler))
```

## Fiber

Use Fiber's native CORS middleware and adapt the `net/http` GraphQL handler at
the route boundary:

```go
app := fiber.New()
app.Use(fibercors.New(fibercors.Config{
	AllowOrigins:     "https://app.example.com",
	AllowMethods:     "GET,POST,OPTIONS",
	AllowHeaders:     "Content-Type, Authorization",
	AllowCredentials: true,
}))
app.All("/graphql", adaptor.HTTPHandler(graphql))

log.Fatal(app.Listen(":4000"))
```

## Gin

Use Gin's CORS middleware before mounting the adapted GraphQL handler:

```go
router := gin.New()
router.Use(gincors.New(gincors.Config{
	AllowOrigins:     []string{"https://app.example.com"},
	AllowMethods:     []string{"GET", "POST", "OPTIONS"},
	AllowHeaders:     []string{"Content-Type", "Authorization"},
	AllowCredentials: true,
}))
router.Any("/graphql", gin.WrapH(graphql))

log.Fatal(router.Run(":4000"))
```

## Chi

Chi uses ordinary `net/http` middleware, so the same CORS middleware can wrap
the GraphQL handler or the whole router:

```go
router := chi.NewRouter()
router.Use(cors.New(cors.Options{
	AllowedOrigins:   []string{"https://app.example.com"},
	AllowedMethods:   []string{"GET", "POST", "OPTIONS"},
	AllowedHeaders:   []string{"Content-Type", "Authorization"},
	AllowCredentials: true,
}).Handler)
router.Mount("/graphql", graphql)

log.Fatal(http.ListenAndServe(":4000", router))
```

## Credentials and subscriptions

Use explicit origins when credentials are enabled. Browsers reject credentialed
requests with `Access-Control-Allow-Origin: *`.

HTTP CORS does not replace WebSocket origin checks. Configure
`websocket.Config.CheckOrigin` separately for WebSocket subscriptions.

See [HTTP middleware](/guides/middleware) for authentication, request IDs,
and other host-level middleware.
