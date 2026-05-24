---
title: HTTP middleware
description: Wrap grx with net/http middleware for request IDs, auth, CORS, size gates, and router-level policy before GraphQL decoding.
outline: deep
---

# HTTP middleware

Middleware runs before a request becomes GraphQL. It can read and write
[`http.Request`](https://pkg.go.dev/net/http#Request) and
[`http.ResponseWriter`](https://pkg.go.dev/net/http#ResponseWriter), so it is
the right layer for headers, cookies, CORS, request IDs, IP allowlists,
authentication, and early request rejection.

Use [plugins](/concepts/plugins) after the transport has decoded a
[`core.Request`](https://pkg.go.dev/github.com/grx-gql/grx/core#Request) and
you need GraphQL lifecycle details such as operation names, validation,
resolved fields, response hooks, or execution errors.

| Concern | Prefer |
| --- | --- |
| Request ID headers | Middleware |
| Cookie / Bearer token parsing | Middleware |
| CORS and browser preflight | Middleware |
| IP allowlist / reverse-proxy trust boundary | Middleware |
| GraphQL operation authorization | `WithOperationAuthorizer` |
| Field-level authorization | `WithFieldAuthorizer` |
| Field tracing and execution metrics | Plugin |

## Add middleware to grx

[`grx.WithMiddleware`](https://pkg.go.dev/github.com/grx-gql/grx#WithMiddleware)
wraps the final GraphQL handler. Middleware is applied in the order supplied,
so the first middleware sees the request first.

```go
package main

import (
	"log"
	"net/http"

	"example.com/hello-grx/graph"

	"github.com/grx-gql/grx"
)

func main() {
	srv, err := grx.NewServer(
		grx.WithSchema(graph.NewSchema()),
		grx.WithMiddleware(
			grx.RequestID("X-Request-Id"),
			securityHeaders,
		),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Fatal(http.ListenAndServe(":4000", srv))
}

func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		next.ServeHTTP(w, r)
	})
}
```

`grx.RequestID` is a convenience wrapper around
[`middlewares.RequestID`](/reference/middlewares/). It reads an incoming
request ID from the named header, generates one when missing, stores it in
context through `core.WithRequestID`, and echoes it on the response.

Resolvers, plugins, and error responses can read the value with
[`core.RequestIDFromContext`](https://pkg.go.dev/github.com/grx-gql/grx/core#RequestIDFromContext).

## Build custom middleware

Reusable middleware usually has two parts:

- A small config type with explicit fields.
- A constructor that validates config once and returns `func(http.Handler) http.Handler`.

This example requires a fixed header value before GraphQL decoding. It is a
simple pattern for internal gateways, cron callers, or preview environments
where a reverse proxy injects a shared header.

```go
package main

import (
	"errors"
	"net/http"
	"strings"
)

type HeaderGateConfig struct {
	Header string
	Value  string
}

func HeaderGate(config HeaderGateConfig) (func(http.Handler) http.Handler, error) {
	header := strings.TrimSpace(config.Header)
	value := strings.TrimSpace(config.Value)
	if header == "" {
		return nil, errors.New("header gate: Header is required")
	}
	if value == "" {
		return nil, errors.New("header gate: Value is required")
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if r.Header.Get(header) != value {
				http.Error(w, "forbidden", http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}, nil
}
```

Wire it at startup and fail fast if the middleware is misconfigured:

```go
gate, err := HeaderGate(HeaderGateConfig{
	Header: "X-Internal-GraphQL",
	Value:  "allow",
})
if err != nil {
	log.Fatal(err)
}

srv, err := grx.NewServer(
	grx.WithSchema(graph.NewSchema()),
	grx.WithMiddleware(gate),
)
```

For request-scoped data, prefer storing a typed value on context instead of
mutating globals or package variables:

```go
package main

import (
	"context"
	"net/http"
	"strings"
)

type tenantKey struct{}

func TenantFromHeader(header string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			tenant := strings.TrimSpace(r.Header.Get(header))
			if tenant == "" {
				http.Error(w, "tenant header is required", http.StatusBadRequest)
				return
			}

			ctx := context.WithValue(r.Context(), tenantKey{}, tenant)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
```

Keep middleware cheap. It runs for every GraphQL request, including CORS
preflight, playground requests, HTTP queries, SSE streams, and WebSocket
upgrades when mounted on the same handler.

## Attach identity to context

GraphQL resolvers receive `context.Context`, not the raw HTTP request.
Middleware should verify transport credentials once, attach trusted identity to
context, and pass `r.WithContext(ctx)` downstream.

```go
package main

import (
	"context"
	"net/http"
	"strings"
)

type subjectKey struct{}

func withSubject(ctx context.Context, subject string) context.Context {
	return context.WithValue(ctx, subjectKey{}, subject)
}

func bearerAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if token == "" {
			next.ServeHTTP(w, r)
			return
		}
		if token != "demo-alice-token" {
			http.Error(w, "invalid bearer token", http.StatusUnauthorized)
			return
		}

		ctx := withSubject(r.Context(), "alice")
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
```

Pair this with [`WithFieldAuthorizer`](/guides/auth#field-level-guards) or
resolver-side checks. The [Authentication & authorization](/guides/auth) guide
shows the full pattern with typed context helpers.

## Compose CORS

Browser CORS belongs outside GraphQL execution. Use
[`grx.Cors`](https://pkg.go.dev/github.com/grx-gql/grx#Cors) as normal
middleware:

```go
srv, err := grx.NewServer(
	grx.WithSchema(graph.NewSchema()),
	grx.WithMiddleware(grx.Cors(grx.CorsConfig{
		AllowedOrigins:   []string{"https://app.example.com"},
		AllowedMethods:   []string{http.MethodGet, http.MethodPost, http.MethodOptions},
		AllowedHeaders:   []string{"Content-Type", "Authorization"},
		AllowCredentials: true,
	})),
)
```

Keep credentialed browser deployments on explicit origins. `AllowedOrigins:
[]string{"*"}` with `AllowCredentials: true` fails closed because reflecting
arbitrary origins with credentials is unsafe.

For subscriptions, pair HTTP CORS with
[`websocket.Config.CheckOrigin`](/guides/subscriptions#origin-policy-checkorigin).

## Wrap from a router

`grx.Server` is an `http.Handler`, so router middleware can wrap it directly.
This is useful when REST and GraphQL share the same auth, logging, timeout, or
quota policy.

```go
package main

import (
	"net/http"

	"example.com/hello-grx/graph"

	"github.com/go-chi/chi/v5"
	"github.com/grx-gql/grx"
)

func main() {
	gql, err := grx.NewServer(
		grx.WithSchema(graph.NewSchema()),
		grx.WithGraphQLPath("/query"),
	)
	if err != nil {
		panic(err)
	}

	r := chi.NewRouter()
	r.Use(bearerAuth)
	r.Mount("/graphql", http.StripPrefix("/graphql", gql))
	_ = http.ListenAndServe(":4000", r)
}
```

When mounting under a prefix, keep the server's internal paths aligned with
whatever reaches `grx.Server` after `StripPrefix`. The router guides for
[ServeMux](/getting-started/servemux), [Chi](/getting-started/chi),
[Gin](/getting-started/gin), [Echo](/getting-started/echo), and
[Fiber](/getting-started/fiber) show framework-specific adapters.

## Ordering

Use order to keep cheap rejection and context setup before expensive work:

```go
grx.WithMiddleware(
	grx.RequestID("X-Request-Id"),
	ipAllowlist,
	bearerAuth,
	grx.Cors(corsConfig),
)
```

The first middleware sees the request first. Put request IDs early so later
middleware, transports, plugins, and resolvers can all observe the same ID.
Put authentication before GraphQL authorizers so `WithOperationAuthorizer` and
`WithFieldAuthorizer` can read trusted identity from context.

## What not to put in middleware

Middleware should not parse GraphQL documents, inspect selected fields, or
rewrite variables. Use executor options, authorizers, custom transports, or
plugins for GraphQL-aware behavior.

Good middleware examples:

- Reject missing or invalid HTTP credentials.
- Attach request-scoped identity, tenant, request ID, or trace IDs.
- Handle browser CORS preflight.
- Set response security headers.
- Enforce proxy-level body limits before GraphQL decoding.

Good plugin/authorizer examples:

- Record validation and execution timings.
- Log selected field paths.
- Reject mutations for anonymous users.
- Block sensitive fields for a role.

## Related

- [Authentication & authorization](/guides/auth)
- [CORS & browsers](/guides/cors-browsers)
- [Hooks and plugins](/concepts/plugins)
- [API reference: middlewares](/reference/middlewares/)
