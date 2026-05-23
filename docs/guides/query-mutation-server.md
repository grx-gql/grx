---
title: Queries and mutations
description: Build a small GraphQL API with multiple entities—queries, mutations, and clean file layout.
outline: [2, 3]
---

# Queries and mutations

This guide builds a small server with a `User` and a `Post` entity, each
with its own query and mutation surface, composed into the root types via
embedding.

If you think in GraphQL SDL, you are building two object types plus `Query` and `Mutation` roots; in grx that is the same shape, but each entity’s resolvers live in one Go file and get **composed** into the roots with embedding instead of a central “register everything” list.

```
hello-grx/
├── go.mod
├── main.go
└── graph/
    ├── schema.go    # composes the root Query and Mutation types
    ├── user.go      # User type, UserQuery, UserMutation
    └── post.go      # Post type, PostQuery, PostMutation
```

## Per-entity files

Each entity owns its types, inputs, payloads, and resolvers in a single
file. This keeps the diff for "add a new entity" to one new file plus one
embedded field per applicable root.

```go
// graph/user.go
package graph

import "context"

type User struct {
    ID    string  `gql:"id,nonNull"`
    Name  string  `gql:"name,nonNull"`
    Email *string `gql:"email"`
}

type UserArgs struct {
    ID string `gql:"id,nonNull"`
}

type UserCreateInput struct {
    Name  string  `gql:"name,nonNull"`
    Email *string `gql:"email"`
}

type UserCreateArgs struct {
    Input UserCreateInput `gql:"input,nonNull"`
}

type UserCreatePayload struct {
    User *User `gql:"user,nonNull"`
}

type UserQuery struct{}

func (UserQuery) User(ctx context.Context, args UserArgs) (*User, error) {
    email := "ada@example.com"
    return &User{ID: args.ID, Name: "Ada Lovelace", Email: &email}, nil
}

type UserMutation struct{}

func (UserMutation) CreateUser(ctx context.Context, args UserCreateArgs) (*UserCreatePayload, error) {
    user := &User{
        ID:    "user_1",
        Name:  args.Input.Name,
        Email: args.Input.Email,
    }
    return &UserCreatePayload{User: user}, nil
}
```

```go
// graph/post.go
package graph

import "context"

type Post struct {
    ID     string `gql:"id,nonNull"`
    Title  string `gql:"title,nonNull"`
    Body   string `gql:"body,nonNull"`
    Author *User  `gql:"author,nonNull"`
}

type PostArgs struct {
    ID string `gql:"id,nonNull"`
}

type PostCreateInput struct {
    Title    string `gql:"title,nonNull"`
    Body     string `gql:"body,nonNull"`
    AuthorID string `gql:"authorId,nonNull"`
}

type PostCreateArgs struct {
    Input PostCreateInput `gql:"input,nonNull"`
}

type PostCreatePayload struct {
    Post *Post `gql:"post,nonNull"`
}

type PostQuery struct{}

func (PostQuery) Post(ctx context.Context, args PostArgs) (*Post, error) {
    return &Post{
        ID:     args.ID,
        Title:  "Hello, grx",
        Body:   "Composing root types from per-entity resolver structs.",
        Author: &User{ID: "user_1", Name: "Ada Lovelace"},
    }, nil
}

type PostMutation struct{}

func (PostMutation) CreatePost(ctx context.Context, args PostCreateArgs) (*PostCreatePayload, error) {
    return &PostCreatePayload{
        Post: &Post{
            ID:     "post_1",
            Title:  args.Input.Title,
            Body:   args.Input.Body,
            Author: &User{ID: args.Input.AuthorID, Name: "Ada Lovelace"},
        },
    }, nil
}
```

## Compose the roots

```go
// graph/schema.go
package graph

import "github.com/patrickkabwe/grx/schema"

type Query struct {
    UserQuery
    PostQuery
}

type Mutation struct {
    UserMutation
    PostMutation
}

func NewSchema() schema.Config {
    return schema.Config{
        Query:    Query{},
        Mutation: Mutation{},
    }
}
```

## Wire up the server

```go
// main.go
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
    )
    if err != nil {
        log.Fatal(err)
    }

    log.Println("listening on http://localhost:4000")
    log.Fatal(http.ListenAndServe(":4000", srv))
}
```

## Try it

```bash
go run .
```

Open the playground at [http://localhost:4000](http://localhost:4000) and
run:

```graphql
mutation {
  createUser(input: { name: "Ada", email: "ada@example.com" }) {
    user { id name email }
  }
}

query {
  user(id: "1") { id name email }
  post(id: "1") { id title author { id name } }
}
```

## Adding a new entity

1. Create `graph/<entity>.go` with the entity struct, input/payload
   structs, `XQuery`, and `XMutation`.

## See also

- **[Testing with the HTTP client](/guides/testing)** — **`httptest.Server`** plus **`pkg/client`** for integration tests against your handler.

