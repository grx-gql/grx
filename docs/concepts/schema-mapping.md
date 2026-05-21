---
title: Schema Mapping
description: How grx turns plain Go structs and methods into a GraphQL schema.
outline: [2, 3]
---

grx is a code-first GraphQL server. You don't write SDL; you write Go, and
the `schema` package reflects your types into the metadata the executor uses
at runtime.

## Root types

The schema container is `schema.Config`:

```go
// schema/builder.go
type Config struct {
    Query        any
    Mutation     any
    Subscription any
}
```

`Query` is required. `Mutation` and `Subscription` are optional. Pass
the resolver bundle via `grx.WithSchema(schema.Config{...})`; the runtime
calls `schema.Build` for you.

Each root is a plain user-defined struct; its **exported methods** become
the fields on the corresponding root type. Method names are lowercased to
produce the GraphQL field name — `User` becomes `user`, `CreatePost` becomes
`createPost`.

```go
type Query struct{}

func (Query) Hello(ctx context.Context) (string, error) {
    return "world", nil
}
```

That is enough to expose `{ hello }`.

## Object types

Object types come from struct types referenced by your resolvers — either as
return types or as nested fields on other return types.

```go
type User struct {
    ID    string  `gql:"id,nonNull"`
    Name  string  `gql:"name,nonNull"`
    Email *string `gql:"email"`
}
```

The `gql` struct tag controls two things:

- **Name override.** `gql:"id"` exposes the field as `id` regardless of the
  Go field name.
- **Non-null.** `gql:",nonNull"` (or `nonNull` in the comma list) wraps the
  type with `!`. Pointer fields are nullable by default; non-pointer values
  are nullable unless tagged otherwise.

## Input types

Resolver argument types are plain Go structs. Nested input objects work the
same way:

```go
type UserCreateInput struct {
    Name  string  `gql:"name,nonNull"`
    Email *string `gql:"email"`
}

type UserCreateArgs struct {
    Input UserCreateInput `gql:"input,nonNull"`
}
```

A resolver receiving `UserCreateArgs` exposes a single `input` argument of
type `UserCreateInput!`.

## Scaling to many entities

`schema.Build` enumerates root methods via `reflect.Type.NumMethod()`, which
includes promoted methods from embedded fields. That gives you a clean
one-file-per-entity layout:

```go
// graph/schema.go
type Query struct {
    UserQuery
    PostQuery
}

type Mutation struct {
    UserMutation
    PostMutation
}

type Subscription struct {
    UserSubscription
}
```

```go
// graph/user.go
type UserQuery struct{}
func (UserQuery) User(ctx context.Context, args UserArgs) (*User, error) { /* ... */ }

type UserMutation struct{}
func (UserMutation) CreateUser(ctx context.Context, args UserCreateArgs) (*UserCreatePayload, error) { /* ... */ }
```

Per-entity dependencies (services, repositories, loaders) belong as fields
on the per-entity resolver struct, not on the root types.

:::caution[Method-name collisions]
Two embedded structs that expose the same method name on the same root will
silently lose one — `reflect` drops ambiguous methods. Treat collisions as a
design smell and rename one side.
:::

## Built-in scalars

The following Go types map to GraphQL scalars out of the box:

| Go type              | GraphQL  |
| -------------------- | -------- |
| `string`             | `String` |
| `int`, `int32`       | `Int`    |
| `float32`, `float64` | `Float`  |
| `bool`               | `Boolean`|

For nullability: pointers (`*string`, `*int`) are nullable. Non-pointer
fields default to nullable in the absence of `nonNull`. Use the tag to
require values.

## What isn't supported yet

The following are tracked on the [Roadmap](/roadmap) and are not yet
usable:

- Enum types
- Interface types and `implements` lists
- Union types
- Custom scalar registration
- Schema/type/field directives, descriptions, and deprecation metadata
- Default argument values

If you need any of these today, please file an issue on GitHub so the
roadmap reflects the priority.
