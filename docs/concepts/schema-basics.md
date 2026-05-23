---
title: Define your schema
description: For GraphQL developers—how grx turns Go types and methods into the schema and resolvers you already think in terms of.
outline: [2, 3]
---

::: tip Start here — schema lives in Go  
This is **code-first authoring**: structs + **`gql` tags** ARE your GraphQL schema at runtime (**no handwritten `.graphql`** unless you export one). Prefer a runnable stub first? [**Run your first server**](/getting-started/) — then read **[Security](/guides/production-security)**, **[Introspection](/guides/introspection)**, and **[Limits](/guides/request-limits)** before you expose a public ingress.

:::

# Define your schema

If you are comfortable with **GraphQL** (types, fields, arguments, `Query` / `Mutation` / `Subscription`) but new to **grx**, read this page once. It explains the **workflow** grx expects, not low-level Go package details.

## What stays the same

Nothing about GraphQL itself changes. Clients still send queries and mutations; your API still has a schema with object types, input types, nullability (`!`), lists, and field arguments. **grx is just another way to author that schema:** you describe it with **Go structs and methods** instead of hand-writing SDL or maintaining a parallel type registry.

## The grx way in five steps

Think “SDL and resolvers”, but the SDL is implied by your types and the resolvers are methods.

1. **Root operations** — You pass a `schema.Config` into `grx.WithSchema(...)`. Put a struct in `Query` (required), and optionally in `Mutation` and `Subscription`. Every **exported method** on that struct is one **top-level GraphQL field** on that root type. The GraphQL name is the method name with the first letter lowercased, so `func (Query) User(...)` is the `user` field on `Query`.

2. **Object types and output fields** — Any struct you **return** from a resolver becomes a GraphQL **object type**. Each **exported struct field** is a **field** on that type. Nested structs are nested object fields; slices are lists.

3. **Field arguments** — If a root (or nested) resolver needs arguments, add a **single struct parameter** to the method. Each **exported field** on that struct is one **GraphQL argument**. Nested structs inside it are **input object** types in GraphQL.

4. **Scalars and lists** — Ordinary Go types map to the usual GraphQL scalars (`string` → `String`, integer types → `Int`, floats → `Float`, `bool` → `Boolean`). `[]T` (or a fixed array) is a GraphQL list of `T`.

5. **Subscriptions** — A subscription field is still a method on your `Subscription` struct, but it returns a **receive-only channel** of values (`<-chan *YourType`). Each value pushed to the client is resolved like a normal object.

**Enums, interfaces, unions, and custom scalars** do not appear from structs alone. When you need them, you register them on the same `schema.Config` you already pass to the server. For the exact fields, see [`schema.Config`](/reference/schema/#Config). For what is fully supported today, see the [Roadmap](/roadmap).

## The `gql` struct tag (when you need it) {#gql-tags}

Many fields need **no tag**: the GraphQL field name defaults from the Go field name (first letter lowercased). Use **`gql:"..."`** when you want to **rename** a field for the API, mark something **required** (`!`), set a **default** on an argument or input field, **hide** a Go field from the schema, or add **description / deprecation** on an **output** field for tools like GraphiQL.

### How to read a tag

Write a comma-separated list. The **first part** is the GraphQL **name** for that field or argument (leave it empty to keep the default name from Go, or use `-` to omit the field from the API). Everything after the first comma is an extra option; order does not matter.

Example: `gql:"legacyId,nonNull,deprecated=Use id"` means: GraphQL name `legacyId`, type is non-null (`!`), field is deprecated with reason `Use id`.

### Options (plain GraphQL terms)

| Option | What clients see | Use it on |
| --- | --- | --- |
| *(first part)* name, e.g. `gql:"authorId"` | That name in the schema instead of the Go field name. | Any exposed field or argument. |
| *(first part empty)* `gql:",nonNull"` | Default GraphQL name from the Go field (`UserID` → `userID`). | Same. |
| `gql:"-"` | Field does not exist in GraphQL at all. | Any field you want to keep in Go only. |
| `nonNull` | The type has a `!` (required for clients). | Output fields, input fields, arguments. |
| `default=value` | Argument or input field has a default if the client omits it (`= value` in GraphQL terms). | **Only** argument structs and input structs—not return types. |
| `description=...` | Doc string on that **output** field in schema explorers. | **Only** fields on structs you **return** from resolvers (not on argument/input structs in today’s behaviour). |
| `deprecated` or `deprecated=reason` | Shows as **`@deprecated`** in explorers. | Same as **`description`**—**output fields only via tags** today (**[Built-in directives](/guides/schema-directives)** for SDL‑only deprecation). |

Structural tags capture **schema** metadata. Executable directives (**`@skip`**, **`@include`**, **`@defer`**, **`@stream`**) belong in queries—see **[Built-in directives](/guides/schema-directives)**.

**Commas:** tags split on commas, so avoid commas inside **`description=`** / **`deprecated=`** messages.

### Example

```go
package graph

type User struct {
	ID        string  `gql:"id,nonNull,description=Stable id"`
	Name      string  `gql:"name,nonNull"`
	LegacyKey *string `gql:"legacyKey,deprecated=Use id instead"`
	internal  string  `gql:"-"` // not exposed in GraphQL
}

type UserArgs struct {
	ID string `gql:"id,nonNull"`
}
```

## Putting it together

At server startup you call `grx.WithSchema(schema.Config{ ... })`. grx walks your types **once**, builds the schema your GraphQL clients expect, and runs operations against the methods and structs you defined. You do not manually register each field twice (once for type, once for resolver): **the type shape and the resolver live in the same Go code.**

## Where to go next

- [Organize your code](/concepts/schema-mapping) — grow from one file to many without a central “register every field” list.
- [Resolver methods](/concepts/resolvers) — optional `context`, errors, and subscription channels in more detail.
