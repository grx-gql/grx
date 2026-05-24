---
title: Organize your code
description: How to grow your Query, Mutation, and Subscription across files while keeping the same GraphQL API shape.
outline: [2, 3]
---

# Organize your code

You already know [how grx maps GraphQL to Go](/concepts/schema-basics). This page is about **project layout**: where to put types and resolvers when the app gets bigger.

## Roots in GraphQL terms

In GraphQL you always have a `Query` type; `Mutation` and `Subscription` are optional. In grx you supply Go values for those same three ideas:

- **`Query`**  -  required. Its methods are your top-level **queries**.
- **`Mutation`**  -  optional. Its methods are your top-level **mutations**.
- **`Subscription`**  -  optional. Its methods are your top-level **subscriptions** (each returns a channel; see [Realtime subscriptions](/guides/subscriptions)).

You bundle them in `schema.Config` and pass that to `grx.WithSchema(...)`. Most apps only set `Query`, `Mutation`, and `Subscription`. Extra slices on the same struct register enums, unions, custom scalars, and so on - see the [schema package reference](/reference/schema/#Config) when you need them.

```go
type Config struct {
    Query          any
    Mutation       any
    Subscription   any
    Scalars        []ScalarConfig
    Enums          []EnumConfig
    Interfaces     []InterfaceConfig
    Unions         []UnionConfig
}
```

Each root value is a struct; **exported methods** become root fields (`CreateUser` → `createUser`).

```go
type Query struct{}

func (Query) Hello(ctx context.Context) (string, error) {
    return "world", nil
}
```

## Objects, inputs, and the `gql` tag

Output types are structs you return from resolvers. Arguments and nested inputs are structs you use as method parameters. Field names, `!`, defaults, and hiding fields use the **`gql`** tag - see [**Define your schema**](/concepts/schema-basics#gql-tags) for the full cheat sheet.

```go
type User struct {
    ID    string  `gql:"id,nonNull"`
    Name  string  `gql:"name,nonNull"`
    Email *string `gql:"email"`
}

type UserCreateInput struct {
    Name  string  `gql:"name,nonNull"`
    Email *string `gql:"email"`
}

type UserCreateArgs struct {
    Input UserCreateInput `gql:"input,nonNull"`
}
```

## Splitting across files (embedding)

A common pattern: one small struct for `Query`, `Mutation`, or `Subscription` that **embeds** one struct per area of your product (`UserQuery`, `PostQuery`, …). In Go, embedded types **promote** their methods to the outer type, so those methods still become top-level GraphQL fields - no second registration step.

```go
type Query struct {
    UserQuery
    PostQuery
}

type Mutation struct {
    UserMutation
    PostMutation
}
```

Put dependencies you need in resolvers (database pool, config, clients) on **`UserQuery`**, **`PostQuery`**, and so on - not on the tiny root struct that only embeds them.

::: warning Duplicate method names
If two embedded types both expose a method with the **same** name on the same root, Go’s method rules hide one of them. Rename a resolver so each top-level field name is unique.
:::

## See also

- [Resolver methods](/concepts/resolvers)  -  `context`, errors, subscription streams.
- [Roadmap](/roadmap)  -  feature coverage.
