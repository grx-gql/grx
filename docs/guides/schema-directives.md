---
title: Built-in directives
description: Every directive grx recognises (@skip/@include/@defer/@stream and schema-side @deprecated/@specifiedBy/@oneOf) and how to author them alongside code-first structs.
outline: deep
---

# Built-in directives

Introspection’s **`__schema.directives`** list is fixed to **seven** built-ins (**[`exec/introspection.go`](https://github.com/patrickkabwe/grx/blob/main/exec/introspection.go)**). Executable directives are enforced while flattening selections; schema directives surface through reflection, **`schema.ScalarConfig`**, or **`exec.ParseSDL`**.

::: tip Executable directives beyond this list  

Arbitrary **`directive @example on FIELD` declarations** that attach custom semantics (like gqlgen/SDL plugins) are **not** part of **`schema.Build`** ergonomics yet—track broader extensibility on the **[Roadmap](/roadmap)**.

:::

---

## Supported directives (overview)

| Name | Executable / schema | GraphQL introspection locations | Arguments surfaced to clients |
| --- | --- | --- | --- |
| **`skip`** | Executable | **`FIELD`**, **`FRAGMENT_SPREAD`**, **`INLINE_FRAGMENT`** | **`if: Boolean!`** |
| **`include`** | Executable | **`FIELD`**, **`FRAGMENT_SPREAD`**, **`INLINE_FRAGMENT`** | **`if: Boolean!`** |
| **`defer`** | Executable (incremental delivery) | **`FIELD`**, **`FRAGMENT_SPREAD`**, **`INLINE_FRAGMENT`** (repeatable) | **`if`:** **`Boolean`** = **`true`**, **`label`:** **`String`** |
| **`stream`** | Executable (incremental delivery; list-valued fields only) | **`FIELD`** (repeatable) | **`if`:** **`Boolean`** = **`true`**, **`label`:** **`String`**, **`initialCount`:** **`Int`** = **`0`** |
| **`deprecated`** | Schema | **`FIELD_DEFINITION`**, **`ARGUMENT_DEFINITION`**, **`INPUT_FIELD_DEFINITION`**, **`ENUM_VALUE`** | **`reason`:** **`String`** = **`No longer supported`** |
| **`specifiedBy`** | Schema | **`SCALAR`** | **`url`:** **`String!`** |
| **`oneOf`** | Schema | **`INPUT_OBJECT`** (**not** repeatable) | *(none)* |

---

## Executable directives

### **`@skip` / `@include`**

Evaluated automatically when **`exec`** merges selection sets (**[`evalSkipInclude`](https://github.com/patrickkabwe/grx/blob/main/exec/directives.go)**). Both require a Boolean **`if`** (literals or **`$variables`**).

```graphql
{
  profile {
    id
    nickname @include(if: $withNick)
    oldHandle @skip(if: $production)
  }
}
```

::: info Client tip  

Treat **`@skip(false)`** / **`@include(true)`** as no-ops; combine both on the same field only when logically consistent—validation rejects impossible directive combinations per **[Validation](/concepts/execution#validation)**.

:::

### **`@defer` / `@stream`** (incremental delivery)

**`@defer`** delays fragment or spread selections behind the primary response. **`if: false`** removes the directive (work runs eagerly). **`label`** namespaces incremental patches.

**`@stream`** batches list output: the resolver must return **Go slices or arrays**. **`initialCount`** is how many list elements stay in the **first** **`data`** payload; the remainder ship as **`incremental[]`** parts. **`initialCount: 0`** skips the eager prefix entirely—every element is scheduled as incremental work (see **`[completeListStreamed](https://github.com/patrickkabwe/grx/blob/main/exec/incremental.go)`**).

**Multipart HTTP:** set **`Accept: multipart/mixed`** so **`pkg/http`** calls **[`Executor.ExecuteIncremental`](https://pkg.go.dev/github.com/patrickkabwe/grx/exec#Executor.ExecuteIncremental)**, wires an incremental collector, and emits **`@defer` / `@stream`** payloads over **`multipart/mixed`** (**[`writeIncremental`](https://github.com/patrickkabwe/grx/blob/main/pkg/http/http_transport.go)**).

**Fallback JSON (`Execute`):** Without that **`Accept`** header—or inside tests invoking **`Execute` directly—the executor **does not** attach an incremental collector. **`@defer` fragments flatten eagerly** (`collectDefers = false`). **`@stream` on lists is ignored**, and the resolver’s slice is serialized in one **`completeValue` / list pass** exactly like ordinary GraphQL responses.

Regression-style coverage: **[incremental_delivery_test.go](https://github.com/patrickkabwe/grx/blob/main/incremental_delivery_test.go)** (**multipart round-trips against **`grx.NewServer`**).

---

## Schema directives **`@deprecated`**

### Output fields (code-first structs)

Deprecation metadata uses **`gql` tags**, matching **`FIELD_DEFINITION`** on struct-backed GraphQL objects:

```go
package graph

type Profile struct {
	ID         string `gql:"id,nonNull"`
	Username   string `gql:"username,nonNull"`
	LegacyName string `gql:"legacyName,deprecated=Use username instead"`
}
```

Introspection and GraphiQL read **`isDeprecated`** + **`deprecationReason`**. **`examples/directives/`** bundles a runnable demo.

::: info Arguments, inputs, enums  

Built-in **`@deprecated`** introspection spans **`ARGUMENT_DEFINITION`**, **`INPUT_FIELD_DEFINITION`**, and **`ENUM_VALUE`**, but **today’s `schema.Build` path wires deprecation only onto struct-backed object fields.** Argument/input/enum deprecation requires SDL-driven schema construction (**`exec.ParseSDL`**) until builder coverage expands.

:::

### Executable examples in sandbox

Combine struct-backed deprecation with client directives:

```graphql
{
  me {
    id
    username @include(if: true)
    legacyName @skip(if: true)
  }
}
```

---

## Schema directive **`@specifiedBy`**

Use **[`schema.ScalarConfig.SpecifiedByURL`](https://pkg.go.dev/github.com/patrickkabwe/grx/schema#ScalarConfig)** when registering custom scalars, or SDL:

```graphql
scalar DateTime @specifiedBy(url: "https://example.com/scalars/datetime")
```

Introspection exposes **`specifiedByURL`**.

---

## Schema directive **`@oneOf`**

Marks **input objects** so exactly **one** field may be supplied. grx honours **`inputType.IsOneOf`** during coercion (**[`input_coercion.go`](https://github.com/patrickkabwe/grx/blob/main/exec/input_coercion.go)**).

::: warning Struct-only authoring gap  

Pure **`schema.Config` + structs** cannot toggle **`IsOneOf`** directly—author SDL (**`input Foo @oneOf { … }`**) via **[`exec.ParseSDL`](https://pkg.go.dev/github.com/patrickkabwe/grx/exec#ParseSDL)** or extend the builder.

:::

---

## Debugging & tooling

| Need | Tip |
| --- | --- |
| Enumerate builtins | **`{ __schema { directives { name locations args { name } isRepeatable } } }`** |
| Validation errors | Mention directive locations sibling merge rules—**[Execution → Validation](/concepts/execution#validation)** |
| Incremental payloads | Inspect **`multipart/mixed`** parts or call **[`Executor.ExecuteIncremental`](https://pkg.go.dev/github.com/patrickkabwe/grx/exec#Executor.ExecuteIncremental)** from tests |

---

## Related

- [`examples/directives`](https://github.com/patrickkabwe/grx/tree/main/examples/directives)
- **[Execution pipeline](/concepts/execution)** (validation + incremental notes)
- **[Define your schema](/concepts/schema-basics)** (**`gql` tag primer**)
