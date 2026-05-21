---
title: Migrate from graph-gophers/graphql-go
description: Step-by-step migration from the graph-gophers/graphql-go schema-first executor to grx.
outline: [2, 3]
---

[`github.com/graph-gophers/graphql-go`](https://github.com/graph-gophers/graphql-go)
is schema-first — you ship an SDL string and a tree of resolver structs
that mirror it. grx is code-first; the schema is *derived* from your Go
types, so the SDL string and the per-object resolver wrappers both
disappear.

This guide walks through the swap using the schema shared by the
[bundled benchmarks](/benchmarks) as the running example.

## The mental model

| graph-gophers/graphql-go                                              | grx                                                                |
| --------------------------------------------------------------------- | ------------------------------------------------------------------ |
| SDL string parsed by `graphql.ParseSchema(sdl, root)`                 | Plain Go structs with `gql:"..."` tags                             |
| Per-object resolver struct (`type userResolver struct{ u *User }`)    | The data type *is* the resolver — no wrapper                       |
| Methods returning Go scalars/objects mapped to GraphQL fields         | Same shape, but on the **root** struct only — nested fields read directly from struct fields |
| `func (r *userResolver) ID() graphql.ID`                              | `ID string \`gql:"id,nonNull"\`` field on the struct               |
| Anonymous arg struct: `func(args struct{ ID graphql.ID })`            | Named arg struct: `type IDArgs struct { ID string \`gql:"id,nonNull"\` }` |
| `graphql.ID`                                                          | `string`                                                           |
| `int32` for `Int!` fields                                             | `int` or `int32`                                                   |
| `relay.Handler{Schema: schema}`                                       | `grx.NewServer(grx.WithSchema(...), ...opts)` — already an `http.Handler`       |
| `graphql.MaxParallelism(N)`                                           | Not needed — grx executes synchronously by design                  |

## Type mapping

| graph-gophers idiom                          | grx equivalent                                |
| -------------------------------------------- | --------------------------------------------- |
| `graphql.ID`                                 | `string`                                      |
| `string` for `String!`                       | `string` (with `gql:",nonNull"`)              |
| `*string` for `String`                       | `*string`                                     |
| `int32` for `Int!`                           | `int` or `int32`                              |
| `[]*Foo`                                     | `[]*Foo`                                      |
| `*Foo` (nullable resolver result)            | `*Foo`                                        |
| Resolver method returning `Foo`              | Struct field of type `Foo` on the parent      |

## Step 1 — Drop the SDL string

In graph-gophers, the schema is the SDL string:

```go
const sdl = `
schema { query: Query }
type Query {
  user(id: ID!): User
  post(id: ID!): Post
  users(count: Int!): [User!]!
}
type User {
  id: ID!
  name: String!
  email: String
}
type Post {
  id: ID!
  title: String!
  body: String!
  author: User!
}
`
```

In grx the SDL doesn't exist as a string. The struct definitions in
the next steps *are* the schema; grx derives the equivalent SDL from
them at startup. Delete the SDL.

:::caution[`@deprecated` / `@specifiedBy` directives]
If your SDL carries directives, see the
[Roadmap](/roadmap#built-in-directives) — directive support and
description metadata aren't through the executor yet. You'll lose them
in the migration today.
:::

## Step 2 — Object resolver structs collapse into data types

**Before** (graph-gophers — note three structs per object):

```go
type userResolver struct{ u *User }

func (r *userResolver) ID() graphql.ID { return graphql.ID(r.u.ID) }
func (r *userResolver) Name() string   { return r.u.Name }
func (r *userResolver) Email() *string { return r.u.Email }

type postResolver struct{ p *Post }

func (r *postResolver) ID() graphql.ID    { return graphql.ID(r.p.ID) }
func (r *postResolver) Title() string     { return r.p.Title }
func (r *postResolver) Body() string      { return r.p.Body }
func (r *postResolver) Author() *userResolver {
    return &userResolver{u: r.p.Author}
}
```

**After** (grx — the data struct *is* the resolver):

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

What disappears:

- The `userResolver` / `postResolver` wrappers.
- The `graphql.ID` conversions (just use `string`).
- The `Author()` method that re-wraps the inner pointer — grx walks
  `*Post.Author` directly because it's already a `*User`.

What you keep:

- Nullability via pointers (`*string` for `String`, value type for
  `String!`).
- Lists as Go slices (`[]*User`).

If a field needs **computed** logic (not just direct field access),
turn the type into a struct **with methods on the root**, not on the
data type itself — see the
[Resolvers](/concepts/resolvers) page.

## Step 3 — Root resolver methods stay; argument types get names

**Before**:

```go
type root struct{}

func (*root) User(args struct{ ID graphql.ID }) *userResolver {
    return &userResolver{u: loadUser(string(args.ID))}
}

func (*root) Users(args struct{ Count int32 }) []*userResolver {
    src := loadUsers(int(args.Count))
    out := make([]*userResolver, len(src))
    for i, u := range src {
        out[i] = &userResolver{u: u}
    }
    return out
}
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

Notable changes:

- Argument structs are **named types** so they can carry `gql` tags.
  The graph-gophers anonymous-struct trick doesn't fit grx's reflection
  model; promote each one to a top-level type.
- Resolvers return `(value, error)` instead of just a value. Errors
  become entries in `errors[]` with the field path populated.
- A `context.Context` is supplied as the first parameter (optional —
  drop it if the resolver doesn't need it).
- The wrapper resolver structs (`*userResolver`, `[]*userResolver`)
  are gone; return the data types directly.

## Step 4 — Splitting one root into per-entity structs

graph-gophers users typically have one giant `root` struct with every
resolver method on it. grx supports the same shape, but
`reflect.Type.NumMethod()` includes promoted methods from embedded
fields, so you can split the root by entity:

```go
// graph/user.go
type UserQuery struct{}
func (UserQuery) User(ctx context.Context, args IDArgs) (*User, error) { /* ... */ }
func (UserQuery) Users(ctx context.Context, args UsersArgs) ([]*User, error) { /* ... */ }

// graph/post.go
type PostQuery struct{}
func (PostQuery) Post(ctx context.Context, args IDArgs) (*Post, error) { /* ... */ }

// graph/schema.go
type Query struct {
    UserQuery
    PostQuery
}
```

Each entity now lives in one file. See
[Build a Query and Mutation Server](/guides/query-mutation-server) for
the full pattern.

## Step 5 — Replace `graphql.ParseSchema` and `relay.Handler`

**Before**:

```go
schema, err := graphql.ParseSchema(sdl, &root{}, graphql.MaxParallelism(1))
if err != nil { log.Fatal(err) }

http.Handle("/graphql", &relay.Handler{Schema: schema})
http.ListenAndServe(":4000", nil)
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

Notes:

- `grx.NewServer` returns an `http.Handler` that already includes the
  GraphiQL playground at `/`. You can drop any `playground` package
  you were wiring up alongside `relay.Handler`.
- `MaxParallelism` doesn't translate. grx executes synchronously by
  design (see [Execution](/concepts/execution)) which is why it
  benchmarks faster than graph-gophers running with the equivalent
  `MaxParallelism(1)` setting.

## Step 6 — Subscriptions

graph-gophers supports subscriptions through resolver methods returning
`<-chan` values. The grx signature is essentially the same — only the
schema/server wiring changes.

**Before** (graph-gophers):

```go
func (*root) UserCreated(ctx context.Context) (<-chan *userResolver, error) {
    out := make(chan *userResolver)
    go func() {
        defer close(out)
        for u := range bus {
            select {
            case <-ctx.Done(): return
            case out <- &userResolver{u: u}:
            }
        }
    }()
    return out, nil
}
```

**After** (grx):

```go
type UserSubscription struct{}

func (UserSubscription) UserCreated(ctx context.Context) (<-chan *User, error) {
    out := make(chan *User)
    go func() {
        defer close(out)
        for u := range bus {
            select {
            case <-ctx.Done(): return
            case out <- u:
            }
        }
    }()
    return out, nil
}

type Subscription struct{ UserSubscription }
```

Then opt into the transports:

```go
import (
    "github.com/patrickkabwe/grx"
    "github.com/patrickkabwe/grx/pkg/sse"
    "github.com/patrickkabwe/grx/pkg/websocket"
    "github.com/patrickkabwe/grx/schema"
)

grx.NewServer(
    grx.WithSchema(schema.Config{
        Query:        Query{},
        Mutation:     Mutation{},
        Subscription: Subscription{},
    }),
    grx.WithTransports(websocket.New(), sse.New()),
)
```

The wire protocol is `graphql-transport-ws` (the modern protocol used
by `graphql-ws` v5+ and Apollo Client v3.5+). graph-gophers users on
the legacy `subscriptions-transport-ws` protocol will need to upgrade
their clients — see [Subscriptions](/concepts/subscriptions).

## Step 7 — Auth, tracing, request-id

In graph-gophers you typically wrap `relay.Handler` in HTTP middleware
to attach values to `r.Context()`. grx prefers
[Plugins](/concepts/plugins) for the same job because they get to
participate in the GraphQL lifecycle, not just the HTTP boundary:

```go
type RequestID struct{ plugin.Base }

func (RequestID) RequestStart(ctx context.Context, _ core.Request) (context.Context, error) {
    return context.WithValue(ctx, requestIDKey{}, newID()), nil
}
```

`RequestStart` is the only hook that may return a derived context. See
[Write a Custom Plugin](/guides/custom-plugin) for the full pattern.

If you genuinely need HTTP-level middleware (CORS, gzip, IP allowlists),
keep wrapping `srv` like any other `http.Handler`:

```go
http.ListenAndServe(":4000", corsMiddleware(srv))
```

## Things that aren't yet supported

If your graph-gophers schema uses any of these, check the
[Roadmap](/roadmap) before committing:

- **Enums** (`type Direction enum`).
- **Interfaces and unions** including `implements` lists.
- **Custom scalars** registered via `graphql.ScalarConfig`.
- **Directives**, including `@deprecated` and `@specifiedBy`.
- **Field aliases**, **fragments**, **`@skip` / `@include`** — parser
  and executor gaps.
- **Default argument values** declared in the SDL.

## Verification checklist

Before deleting `graph-gophers/graphql-go` from your `go.mod`:

- [ ] Build the grx schema in a sibling package; keep the old SDL
  around as a reference.
- [ ] Run a representative query set through both executors and diff
  the JSON. Pay attention to nullability — graph-gophers tends to
  return non-null fields as zero values; grx returns `null` for
  pointer-typed `nil`.
- [ ] Move HTTP middleware that *only cares about HTTP* (CORS, gzip)
  by wrapping `srv`. Move middleware that touches the GraphQL
  lifecycle into a plugin.
- [ ] Confirm subscriptions still work end-to-end — clients on the old
  `subscriptions-transport-ws` protocol must upgrade.
- [ ] Re-benchmark your real workload. graph-gophers is the closest
  competitor in the [bundled benchmarks](/benchmarks) (~3.7× slower
  on the nested query); your numbers will depend on the size and
  shape of your responses.

When everything passes:

```bash
go mod edit -droprequire github.com/graph-gophers/graphql-go
go mod tidy
```
