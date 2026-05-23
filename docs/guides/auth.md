---
title: Authentication & authorization
description: Attach HTTP identity via middleware, propagate it through context, and guard fields with executor authorizers.
outline: deep
---

# Authentication & authorization

Pair this guide with **[Security](/guides/production-security)** (errors, hooks), **[Introspection](/guides/introspection)** (**`__schema`** / GraphiQL), and **[Limits](/guides/request-limits)** (**`MaxHTTPRequestBytes`**, timeouts) when tightening production surfaces—or use the **[overview hub](/concepts/graphql-security-production)**.

GraphQL transports call `executor.Execute` with the Go `context.Context`
from HTTP—**not** the raw [`net/http.Request`](https://pkg.go.dev/net/http#Request). That is a deliberate split:
[`core.Request`](https://pkg.go.dev/github.com/patrickkabwe/grx/core#Request)
only carries GraphQL payloads (document, operation name, variables).

To authenticate users:

1. **HTTP layer** — read `Authorization` (or cookies) inside
   [`grx.WithMiddleware`](/reference/grx/) middleware (or Chi/Gin adapters that wrap the [`grx.Server`](https://pkg.go.dev/github.com/patrickkabwe/grx#Server) handler).
2. **Attach identity** — put a verified subject/session on the context (`context.WithValue`).
3. **Authorize** — use [`WithFieldAuthorizer`](#field-level-guards),
   [`WithOperationAuthorizer`](#operation-level-rules), resolver checks—or a [`plugin.Plugin`](/reference/plugin/)
   hook when you want a single cross-cutting policy point.

The runnable **`examples/auth/`** folder follows that trio end to end (`go run ./examples/auth`).

## Bearer token middleware

This middleware parses `Authorization: Bearer <token>`, rejects stale tokens **before**
GraphQL runs, and records the fixed subject `"alice"` when callers present the demo token (`demo-alice-token`):

```go
package main

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/patrickkabwe/grx/examples/auth/session"
)

// demoBearerToken is static so callers can paste it into GraphiQL headers.
const demoBearerToken = "demo-alice-token"

func parseBearer(header string) (token string, ok bool) {
	parts := strings.Fields(strings.TrimSpace(header))
	if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
		return "", false
	}
	return parts[1], true
}

func bearerAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()

		token, hasBearerScheme := parseBearer(r.Header.Get("Authorization"))
		if hasBearerScheme {
			switch token {
			case demoBearerToken:
				ctx = session.ContextWithSubject(ctx, "alice")
			case "":
				http.Error(w, `invalid Authorization header`, http.StatusBadRequest)
				return
			default:
				http.Error(w, `invalid bearer token`, http.StatusUnauthorized)
				return
			}
		}

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
```

The meaningful line is **`next.ServeHTTP(w, r.WithContext(ctx))`**. The bundled
GraphQL-over-HTTP transport in [`pkg/http`](https://pkg.go.dev/github.com/patrickkabwe/grx/pkg/http)
executes each operation with `Request.Context()`, so downstream resolvers and authorizers reuse the enriched context unchanged.

::: tip Browsers must be allowed to send `Authorization`

Remember to open `Authorization` in CORS config (same pattern as the [subscriptions snippet](/examples/#examples-subscriptions)).

:::

### Resolver-side identity

Prefer unexported context keys typed to your app—the `examples/auth/session` package wraps `context.WithValue` so resolver code stays short:

```go
package graph

import (
	"context"
	"errors"

	"github.com/patrickkabwe/grx/examples/auth/session"
)

// User is declared alongside [Query] in examples/auth/graph.
func (Query) Viewer(ctx context.Context) (*User, error) {
	sub, ok := session.Subject(ctx)
	if !ok {
		return nil, errors.New("unauthenticated")
	}
	return &User{ID: sub, DisplayName: "Signed-in subject " + sub}, nil
}
```

## Field-level guards

[`WithFieldAuthorizer`](https://pkg.go.dev/github.com/patrickkabwe/grx#WithFieldAuthorizer)
runs immediately before coercion + resolver invocation. Returning `fmt.Errorf(...)`
(or any non-nil error) surfaces as the GraphQL error for `viewer` specifically:

```go
package main

import (
	"context"
	"fmt"

	"github.com/patrickkabwe/grx"
	"github.com/patrickkabwe/grx/examples/auth/graph"
	"github.com/patrickkabwe/grx/examples/auth/session"
)

const demoBearerToken = "demo-alice-token"

func requireViewerField() grx.FieldAuthorizer {
	return func(ctx context.Context, fc grx.FieldAuthorizationContext) error {
		if fc.ParentType != "Query" || fc.FieldName != "viewer" {
			return nil
		}
		if _, ok := session.Subject(ctx); !ok {
			return fmt.Errorf("viewer requires Authorization: Bearer %s", demoBearerToken)
		}
		return nil
	}
}

func wireServerWithFieldAuth() error {
	srv, err := grx.NewServer(
		grx.WithSchema(graph.NewSchema()),
		grx.WithMiddleware(bearerAuth),
		grx.WithFieldAuthorizer(requireViewerField()),
	)
	if err != nil {
		return err
	}
	_ = srv // listen with http.ListenAndServe in your entrypoint
	return nil
}
```

**`bearerAuth`** is the middleware from the first snippet (**`session`** attaches the subject in both). Full wiring lives in [`examples/auth/main.go`](https://github.com/patrickkabwe/grx/blob/main/examples/auth/main.go).

Pairing declarative guards (authorizers/plugins) with HTTP middleware avoids duplicating
header parsing logic across every resolver.

## Operation-level rules

[`WithOperationAuthorizer`](https://pkg.go.dev/github.com/patrickkabwe/grx#WithOperationAuthorizer)
runs while the executor validates the parsed document — **before** any field resolves.
Inspect [`OperationContext`](https://pkg.go.dev/github.com/patrickkabwe/grx/exec#OperationContext)
for operation kind/name and reuse the incoming [`core.Request`](https://pkg.go.dev/github.com/patrickkabwe/grx/core#Request):

```go
package main

import (
	"context"
	"fmt"

	"github.com/patrickkabwe/grx"
	"github.com/patrickkabwe/grx/core"
)

func hasElevatedClaim(ctx context.Context) bool {
	// Your policy — JWT scope, RBAC lookup, ...
	return false
}

// Pass into grx.NewServer(...)
var forbidAnonymousMutations = grx.WithOperationAuthorizer(func(ctx context.Context, op grx.OperationContext) error {
	if op.Kind == core.OperationKindMutation && !hasElevatedClaim(ctx) {
		return fmt.Errorf("mutations denied for anonymous callers")
	}
	return nil
})
```

Reach for operations when policies are coarse (“block every mutation”), and mix in field guards for row/column granularity (“only admins may request `viewer.email`”).

## Run the example locally

```bash
go run ./examples/auth
```

1. **`http://localhost:4010/`** — GraphiQL.
2. Run `{ ping }` without headers—it stays public on purpose.
3. Run `{ viewer { id displayName } }`; expect a GraphQL error describing the Bearer requirement until you add GraphiQL headers:

```json
{ "Authorization": "Bearer demo-alice-token" }
```

---

**Further reading**

- Lifecycle context: [Hooks and plugins](/concepts/plugins).
- Source tree: [`examples/auth`](https://github.com/patrickkabwe/grx/tree/main/examples/auth).
