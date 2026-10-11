---
title: HTTP middleware
description: Add HTTP middleware around the grx handler in the host application.
outline: deep
---

# HTTP middleware

Use this guide when a concern needs HTTP headers, cookies, the client address,
or the whole host route. Use a grx plugin when the concern needs operation,
field, or execution lifecycle data.

`grx.Server` implements [`http.Handler`](https://pkg.go.dev/net/http#Handler).
HTTP middleware belongs in the host application, around that handler. This
keeps GraphQL configuration focused on GraphQL and lets each HTTP framework
use its own middleware ecosystem.

```go
srv, err := grx.NewServer(
    grx.WithSchema(graph.Schema()),
)
if err != nil {
    log.Fatal(err)
}

handler := securityHeaders(authenticate(srv))
log.Fatal(http.ListenAndServe(":4000", handler))
```

Use the middleware provided by your HTTP stack for concerns such as:

- authentication and request IDs
- CORS and browser preflight
- security headers and rate limits
- request size limits and IP allowlists
- access logging and tracing

For CORS examples, see [CORS and browsers](/guides/cors-browsers). For
authentication that also authorizes GraphQL fields, see
[Authentication and authorization](/guides/auth).

## `net/http`

Any middleware with the standard `func(http.Handler) http.Handler` shape can
wrap the server directly:

```go
srv, err := grx.NewServer(grx.WithSchema(graph.Schema()))
if err != nil {
    log.Fatal(err)
}

handler := requestID(securityHeaders(srv))
log.Fatal(http.ListenAndServe(":4000", handler))
```

Middleware that attaches trusted request data to the context should preserve
the request context when calling the next handler:

```go
func tenant(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        value := strings.TrimSpace(r.Header.Get("X-Tenant"))
        if value == "" {
            http.Error(w, "X-Tenant is required", http.StatusBadRequest)
            return
        }

        ctx := context.WithValue(r.Context(), tenantKey{}, value)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

## GraphQL-aware behavior

Use [plugins](/concepts/plugins) when behavior needs parsed GraphQL
operations, validation, field execution, response hooks, or GraphQL errors.
Use `WithOperationAuthorizer` for operation-level authorization and
`WithFieldAuthorizer` for field-level authorization.

HTTP middleware still runs before GraphQL decoding, so it is the appropriate
place to reject unauthenticated requests or handle browser preflight requests.
