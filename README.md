# grx

`grx` is a fast, lightweight GraphQL server and runtime for Go. You write plain Go structs and methods; `grx` turns them into an executable GraphQL schema — no SDL files, no code generation — and serves it as an ordinary `http.Handler`.

- **Code-first schema** — exported structs are object types, methods on `Query`, `Mutation` and `Subscription` are resolvers, and `gql` struct tags set names and nullability.
- **Queries, mutations and subscriptions** over HTTP, WebSocket (`graphql-transport-ws`) and Server-Sent Events.
- **Plain `net/http`** — mount it on the standard mux, Chi, Gin, Echo or Fiber.
- **Production controls** — request and depth limits, timeouts, persisted queries, operation and field authorizers, introspection toggles, gzip, plugins and middleware.

## Install

```bash
go get github.com/grx-gql/grx@latest
```

Requires Go 1.25 or newer.

## Quick start

```go
package main

import (
	"context"
	"log"
	"net/http"

	"github.com/grx-gql/grx"
	"github.com/grx-gql/grx/schema"
)

type User struct {
	ID   string `gql:"id,nonNull"`
	Name string `gql:"name,nonNull"`
}

type UserArgs struct {
	ID string `gql:"id,nonNull"`
}

type Query struct{}

// User becomes the `user(id: String!): User` field on Query.
func (Query) User(ctx context.Context, args UserArgs) (*User, error) {
	return &User{ID: args.ID, Name: "Ada Lovelace"}, nil
}

func main() {
	server, err := grx.NewServer(
		grx.WithSchema(schema.Config{Query: Query{}}),
		grx.WithPlaygroundPath("/"),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("playground on http://localhost:4000/, GraphQL on POST /graphql")
	log.Fatal(http.ListenAndServe(":4000", server))
}
```

Run it with `go run .`, open `http://localhost:4000/` and query:

```graphql
{
  user(id: "1") {
    id
    name
  }
}
```

## Examples

Runnable examples live in [`examples/`](./examples):

```bash
go run ./examples/basic           # queries, mutations, plugins
go run ./examples/subscriptions   # pub/sub over WebSocket and SSE
go run ./examples/auth            # bearer auth, context identity, field authorizer
```

Tagged **[Go module](https://go.dev/doc/modules/publishing)** releases (no binaries). The nested **`redis`** module is **not** in release-please — tag **`redis/v*`** yourself (the same workflow warms **`proxy.golang.org`** for those tags).

### Pub/Sub API migration

The current pub/sub API is byte-oriented and backend-neutral. Import
`github.com/grx-gql/grx/pubsub` for the `PubSub` contract,
`github.com/grx-gql/grx/pubsub/memory` for the in-process backend, or
`github.com/grx-gql/grx/pubsub/redis` for the separate Redis module. The former
`memory-pubsub` and `redis-pubsub` packages, typed wrapper, codecs, and
filters are no longer part of the public API; encode domain values in
resolvers before publishing and decode them after subscribing.

There are also examples for directives, enums, interfaces and unions, custom scalars and file uploads.

## Documentation

The full documentation is at **https://grx-gql.github.io/grx/**:

- [Get started](https://grx-gql.github.io/grx/getting-started/) — a first server, and mounting it on your router
- [Define your schema](https://grx-gql.github.io/grx/concepts/schema-basics) — types, lists, enums, unions and inputs
- [Subscriptions](https://grx-gql.github.io/grx/guides/subscriptions) — pub/sub, WebSocket and SSE
- [Security](https://grx-gql.github.io/grx/guides/production-security) and [Deployment](https://grx-gql.github.io/grx/guides/deployment) — going to production
- API reference on [pkg.go.dev](https://pkg.go.dev/github.com/grx-gql/grx)

`grx` is in active development and does not yet cover the whole GraphQL specification; [ROADMAP.md](./ROADMAP.md) tracks what is missing.

## Contributing

See [CONTRIBUTING.md](./CONTRIBUTING.md).

## License

[MIT](./LICENSE)
