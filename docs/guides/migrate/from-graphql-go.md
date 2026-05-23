---
title: Migrate from graphql-go/graphql
description: Step-by-step migration from the graphql-go/graphql code-first builder API to grx.
outline: [2, 3]
---

# Migrate from graphql-go/graphql

[`github.com/graphql-go/graphql`](https://github.com/graphql-go/graphql)
is code-first like grx, but builds the type system imperatively with
`graphql.NewObject` / `graphql.Field` constructors and per-field
`Resolve` callbacks. grx replaces both with plain Go structs and
methods.

This guide walks through the swap field by field, using the schema
shared by the [bundled benchmarks](/benchmarks) as the running
example.

## The mental model

| graphql-go/graphql                                            | grx                                                                |
| ------------------------------------------------------------- | ------------------------------------------------------------------ |
| `graphql.NewObject(graphql.ObjectConfig{...})`                | A plain Go struct                                                  |
| `graphql.Fields{ "name": &graphql.Field{Type: ...} }`         | Struct fields with `gql:"name,nonNull"` tags                       |
| `Resolve: func(p graphql.ResolveParams) (any, error)`         | A method on a root struct: `func(T) Field(ctx, args) (R, error)`   |
| `p.Args["id"].(string)`                                       | A typed `args` struct argument                                     |
| `graphql.NewSchema(graphql.SchemaConfig{Query: queryType})`   | `grx.WithSchema(schema.Config{Query: Query{}})` in the `grx.NewServer` option list |
| `graphql.Do(graphql.Params{Schema: ..., RequestString: ...})` | The HTTP handler returned by `grx.NewServer` (or `core.Executor`)  |

## Type mapping

| graphql-go type                              | grx equivalent                                |
| -------------------------------------------- | --------------------------------------------- |
| `graphql.String`                             | `string`                                      |
| `graphql.Int`                                | `int` (or `int32`)                            |
| `graphql.Float`                              | `float64`                                     |
| `graphql.Boolean`                            | `bool`                                        |
| `graphql.ID`                                 | `string`                                      |
| `graphql.NewNonNull(T)`                      | `gql:",nonNull"` tag (or non-pointer field)   |
| `graphql.NewList(T)`                         | `[]T`                                         |
| `graphql.NewList(graphql.NewNonNull(T))`     | `[]T` with element type non-pointer           |
| `graphql.NewObject(...)`                     | A struct type                                 |
| `graphql.NewInputObject(...)`                | An input struct used as an argument type      |

## Step 1 — Object types become structs

**Before** (graphql-go):

```go
userType := graphql.NewObject(graphql.ObjectConfig{
    Name: "User",
    Fields: graphql.Fields{
        "id":    &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
        "name":  &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
        "email": &graphql.Field{Type: graphql.String},
    },
})

postType := graphql.NewObject(graphql.ObjectConfig{
    Name: "Post",
    Fields: graphql.Fields{
        "id":    &graphql.Field{Type: graphql.NewNonNull(graphql.ID)},
        "title": &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
        "body":  &graphql.Field{Type: graphql.NewNonNull(graphql.String)},
        "author": &graphql.Field{
            Type: graphql.NewNonNull(userType),
            Resolve: func(p graphql.ResolveParams) (any, error) {
                if post, ok := p.Source.(*Post); ok {
                    return post.Author, nil
                }
                return nil, nil
            },
        },
    },
})
```

**After** (grx):

```go
type User struct {
    ID    string  `gql:"id,nonNull"`
    Name  string  `gql:"name,nonNull"`
    Email *string `gql:"email"`
}

type Post struct {
    ID     string `gql:"id,nonNull"`
    Title  string `gql:"title,nonNull"`
    Body   string `gql:"body,nonNull"`
    Author *User  `gql:"author,nonNull"`
}
```

Two things to notice:

- The nested `author` field needs no resolver — grx walks the Go struct
  graph automatically. Source-typecasting like
  `p.Source.(*Post)` is gone.
- Nullability moves from the type wrapper to either a pointer field
  (`*string`) or the `nonNull` tag.

## Step 2 — Root `Query` becomes a struct with methods

**Before**:

```go
queryType := graphql.NewObject(graphql.ObjectConfig{
    Name: "Query",
    Fields: graphql.Fields{
        "user": &graphql.Field{
            Type: userType,
            Args: graphql.FieldConfigArgument{
                "id": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.ID)},
            },
            Resolve: func(p graphql.ResolveParams) (any, error) {
                id, _ := p.Args["id"].(string)
                return loadUser(id), nil
            },
        },
        "users": &graphql.Field{
            Type: graphql.NewNonNull(graphql.NewList(graphql.NewNonNull(userType))),
            Args: graphql.FieldConfigArgument{
                "count": &graphql.ArgumentConfig{Type: graphql.NewNonNull(graphql.Int)},
            },
            Resolve: func(p graphql.ResolveParams) (any, error) {
                count, _ := p.Args["count"].(int)
                return loadUsers(count), nil
            },
        },
    },
})
```

**After**:

```go
type IDArgs struct {
    ID string `gql:"id,nonNull"`
}

type UsersArgs struct {
    Count int `gql:"count,nonNull"`
}

type Query struct{}

func (Query) User(ctx context.Context, args IDArgs) (*User, error) {
    return loadUser(args.ID), nil
}

func (Query) Users(ctx context.Context, args UsersArgs) ([]*User, error) {
    return loadUsers(args.Count), nil
}
```

What changed:

- Field name comes from the lowercased method name (`User` → `user`).
- Arguments are a typed Go struct, not a `map[string]any` you have to
  type-assert by hand.
- `ctx context.Context` is wired through automatically; it's the same
  context as the HTTP request, already cancelled when the client
  disconnects.
- Both `ctx` and `args` are optional. Omit either when a resolver
  doesn't need it.

## Step 3 — Input types become argument structs

**Before**:

```go
userInput := graphql.NewInputObject(graphql.InputObjectConfig{
    Name: "UserCreateInput",
    Fields: graphql.InputObjectConfigFieldMap{
        "name":  &graphql.InputObjectFieldConfig{Type: graphql.NewNonNull(graphql.String)},
        "email": &graphql.InputObjectFieldConfig{Type: graphql.String},
    },
})
// ...used as
Args: graphql.FieldConfigArgument{
    "input": &graphql.ArgumentConfig{Type: graphql.NewNonNull(userInput)},
}
```

**After**:

```go
type UserCreateInput struct {
    Name  string  `gql:"name,nonNull"`
    Email *string `gql:"email"`
}

type CreateUserArgs struct {
    Input UserCreateInput `gql:"input,nonNull"`
}
```

Nested input objects compose just like outputs — keep nesting structs.

## Step 4 — Replace `graphql.Do` with the grx HTTP server

**Before**:

```go
schema, _ := graphql.NewSchema(graphql.SchemaConfig{Query: queryType})

http.HandleFunc("/graphql", func(w http.ResponseWriter, r *http.Request) {
    var body struct {
        Query     string         `json:"query"`
        Variables map[string]any `json:"variables"`
    }
    json.NewDecoder(r.Body).Decode(&body)
    res := graphql.Do(graphql.Params{
        Schema:         schema,
        RequestString:  body.Query,
        VariableValues: body.Variables,
        Context:        r.Context(),
    })
    json.NewEncoder(w).Encode(res)
})
```

**After**:

```go
import (
    "github.com/patrickkabwe/grx"
    "github.com/patrickkabwe/grx/schema"
)

srv, err := grx.NewServer(
    grx.WithSchema(schema.Config{Query: Query{}}),
    grx.WithPlaygroundPath("/"),
)
if err != nil { log.Fatal(err) }

http.ListenAndServe(":4000", srv)
```

This handler also serves the GraphiQL playground at `/` for free; you
can drop your existing playground wiring.

## Step 5 — Subscriptions (no migration; new capability)

`graphql-go/graphql` doesn't ship a subscription executor. If you were
implementing realtime out-of-band (e.g. Redis pub/sub + WebSocket
handling on the side), grx replaces both halves. See
[Realtime subscriptions](/guides/subscriptions).

Resolver shape for a subscription:

```go
type Subscription struct{}

func (Subscription) UserCreated(ctx context.Context) (<-chan *User, error) {
    stream := make(chan *User)
    go publishInto(ctx, stream)
    return stream, nil
}
```

Then register the WebSocket and SSE transports via `grx.WithTransports(...)`.

## Step 6 — Plugins replace HTTP middleware

Anything you wrapped around `graphql.Do` — request logging, tracing,
auth — moves to a [Plugin](/concepts/plugins). The grx executor calls
your plugins at the matching lifecycle hook so you don't need a custom
HTTP middleware ring anymore.

```go
import (
    "log/slog"
    "os"

    "github.com/patrickkabwe/grx"
    "github.com/patrickkabwe/grx/plugin/logger"
    "github.com/patrickkabwe/grx/schema"
)

loggerPlugin, _ := logger.New(logger.Config{
    Logger: slog.New(slog.NewJSONHandler(os.Stdout, nil)),
})

grx.NewServer(
    grx.WithSchema(schema.Config{Query: Query{}}),
    grx.WithPlugins(loggerPlugin),
)
```

## Things that aren't yet supported

If your `graphql-go` schema uses any of the following, hold the
migration until the matching [roadmap](/roadmap) item is checked:

- **Enums.** `graphql.NewEnum(...)` — not yet mapped from Go types.
- **Interfaces and unions.** `graphql.NewInterface`, `graphql.NewUnion`.
- **Custom scalars.** `graphql.NewScalar(...)` — only built-in scalars
  for now.
- **Directives** including `@deprecated`.
- **Field aliases**, **fragments**, **`@skip` / `@include`** — these
  are parser/executor gaps.
- **Default argument values** in the schema definition.

## Verification checklist

Before deleting `graphql-go/graphql` from your `go.mod`:

- [ ] Stand up the grx schema in a parallel package.
- [ ] Run a representative set of queries through both executors and
  diff the JSON responses.
- [ ] Move at least one plugin (logging is easiest) and confirm the
  hook fires on every request.
- [ ] If you have subscriptions, confirm a client can connect, receive
  values, and disconnect cleanly.
- [ ] Re-benchmark your real workload — `graphql-go`'s ~25× overhead is
  most visible on small queries; the relative win shrinks but stays
  positive on large list responses.

When everything passes:

```bash
go mod edit -droprequire github.com/graphql-go/graphql
go mod tidy
```
