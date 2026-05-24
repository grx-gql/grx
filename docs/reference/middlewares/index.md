---
title: middlewares
description: API reference for HTTP middleware helpers.
outline: [2, 4]
lastUpdated: false
---

# middlewares

```go
import "github.com/grx-gql/grx/middlewares"
```

Package `middlewares` contains HTTP middleware helpers that run outside the GraphQL execution lifecycle.

Use middleware for transport-level concerns that need `http.Request` or `http.ResponseWriter`, such as headers, cookies, CORS, request IDs, authentication, and request size limits. Use [`plugins`](/reference/plugins/) for GraphQL lifecycle concerns after a transport has decoded the operation.

## Index

- [type Middleware](#Middleware)
- [func RequestID(header string) Middleware](#RequestID)

<a name="Middleware"></a>
## type Middleware

```go
type Middleware func(http.Handler) http.Handler
```

Middleware wraps an HTTP handler.

<a name="RequestID"></a>
## func RequestID

```go
func RequestID(header string) Middleware
```

RequestID returns middleware that ensures each request carries a request ID in context through `core.RequestIDFromContext` and echoes it on the response. When `header` is empty, `X-Request-Id` is used. If the incoming request omits the header, a random ID is generated.

The root `grx.RequestID` helper delegates to this package for convenience.
