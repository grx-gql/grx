package core

import "net/http"

// Middleware wraps the GraphQL server's HTTP handler. It runs outside the
// GraphQL execution lifecycle (CORS, request IDs, auth headers, etc.) and
// uses the standard Go shape func(next http.Handler) http.Handler.
type Middleware func(http.Handler) http.Handler
