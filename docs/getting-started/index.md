---
title: Run your first server
description: Build and run a small GraphQL server in Go with grx.
outline: deep
---

# Run your first server

By the end of this page you will have a Go GraphQL server running on
`http://localhost:4000`.

You will define a schema with Go types, build a grx server, run it with
`net/http`, and send one query. Framework integrations come after this page.

## What you will build

The request flow is:

1. `net/http` receives the request.
2. The grx handler selects the HTTP transport.
3. grx parses and validates the GraphQL document.
4. The executor calls the `Query` resolver.
5. grx writes a JSON response.

This is the default path for queries and mutations. Subscriptions add a
WebSocket or SSE transport later.

## 1. Create the module

```bash
mkdir hello-grx
cd hello-grx
go mod init example.com/hello-grx
go get github.com/grx-gql/grx@latest
mkdir graph
```

## 2. Define the schema

Create `graph/schema.go`:

```go
package graph

import (
	"context"

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

func (Query) User(ctx context.Context, args UserArgs) (*User, error) {
	return &User{ID: args.ID, Name: "Ada Lovelace"}, nil
}

func Schema() schema.Config {
	return schema.Config{Query: Query{}}
}
```

`Query` is the required root. Its exported methods become top-level query
fields. The `User` method becomes the GraphQL field `user`.

The `gql` tags make the public contract explicit. Here they mark `id` and
`name` as non-null and define the argument name as `id`.

Read [Define your schema](/concepts/schema-basics) for objects, inputs, lists,
custom scalars, and root composition.

## 3. Build the server

Create `main.go`:

```go
package main

import (
	"log"
	"net/http"

	"example.com/hello-grx/graph"
	"github.com/grx-gql/grx"
)

func main() {
	server, err := grx.NewServer(
		grx.WithSchema(graph.Schema()),
		grx.WithPlaygroundPath("/"),
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Println("GraphiQL: http://localhost:4000/")
	log.Println("GraphQL: POST http://localhost:4000/graphql")
	log.Fatal(http.ListenAndServe(":4000", server))
}
```

Run it:

```bash
go run .
```

Open `http://localhost:4000/` and run:

```graphql
{
  user(id: "user_1") {
    id
    name
  }
}
```

## What grx owns

grx builds and executes the GraphQL schema and provides the GraphQL
transports. Your application still owns routing, CORS, authentication
middleware, TLS, databases, and deployment.

Read [Runtime boundaries](/concepts/runtime-boundaries) before adding those
concerns to a production service.

## Next steps

- [Why choose grx](/concepts/why-grx)
- [Define your schema](/concepts/schema-basics)
- [Organize your code](/concepts/schema-mapping)
- [Choose an HTTP integration](/getting-started/basic-http)
- [HTTP, WebSocket, and SSE](/concepts/transports)
- [Authentication and authorization](/guides/auth)
- [Deployment and scaling](/guides/deployment-and-scaling)
