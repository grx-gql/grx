---
title: Getting Started
description: Install grx, define your first schema, and run a working GraphQL server in a few minutes.
sidebar:
  order: 1
---

This page walks you from a fresh Go module to a running grx server with a
GraphiQL playground in under five minutes.

## Prerequisites

- Go 1.22 or newer.
- A terminal and an editor.

## 1. Create a module

```bash
mkdir hello-grx && cd hello-grx
go mod init example.com/hello-grx
go get github.com/patrickkabwe/grx@latest
```

## 2. Define a schema

Create `graph/schema.go`:

```go
package graph

import (
	"context"

	"github.com/patrickkabwe/grx/schema"
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

func NewSchema() schema.Config {
	return schema.Config{Query: Query{}}
}
```

The struct field tag `gql:"name,nonNull"` controls the GraphQL field name and
nullability. Method names are lowercased to produce GraphQL field names
(`User` becomes the `user` query field).

## 3. Run the server

Create `main.go`:

```go
package main

import (
	"log"
	"net/http"

	"example.com/hello-grx/graph"
	"github.com/patrickkabwe/grx/server"
)

func main() {
	srv, err := server.New(server.Config{
		Schema:         graph.NewSchema(),
		PlaygroundPath: "/",
	})
	if err != nil {
		log.Fatal(err)
	}

	log.Println("listening on http://localhost:4000")
	log.Fatal(http.ListenAndServe(":4000", srv))
}
```

Then:

```bash
go run .
```

Open [http://localhost:4000](http://localhost:4000) and you have a GraphiQL
playground talking to your schema. Try:

```graphql
{ user(id: "1") { id name } }
```

## What's next

- Read [Architecture](/concepts/architecture/) to understand how grx is
  structured.
- Learn how Go types become GraphQL types in
  [Schema Mapping](/concepts/schema-mapping/).
- Add a mutation and a subscription with the
  [Query &amp; Mutation Server](/guides/query-mutation-server/) and
  [Subscriptions](/guides/subscriptions/) guides.

If you're already running a GraphQL server with another Go library, jump
to [Migrate to grx](/guides/migrate/) for a step-by-step swap from
[`graphql-go/graphql`](/guides/migrate/from-graphql-go/) or
[`graph-gophers/graphql-go`](/guides/migrate/from-graph-gophers/).
