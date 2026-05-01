---
title: schema
description: API reference for the schema package, generated from Go doc comments.
tableOfContents:
  minHeadingLevel: 2
  maxHeadingLevel: 4
editUrl: false
---



```go
import "github.com/patrickkabwe/grx/schema"
```

Package schema describes the runtime metadata that the executor uses to resolve GraphQL operations. It also exposes the [Build](<#Build>) entry point that reflects user\-defined Go types into this metadata.

The types here mirror the GraphQL type system from the October 2021 specification. Reflection is confined to schema\-build time so the executor's per\-request hot path stays allocation\-aware.

## Index

- [type Builder](<#Builder>)
- [type Config](<#Config>)
- [type Field](<#Field>)
- [type InputObject](<#InputObject>)
  - [func \(i \*InputObject\) Kind\(\) Kind](<#InputObject.Kind>)
  - [func \(i \*InputObject\) Name\(\) string](<#InputObject.Name>)
- [type InputValue](<#InputValue>)
- [type Interface](<#Interface>)
  - [func \(i \*Interface\) Kind\(\) Kind](<#Interface.Kind>)
  - [func \(i \*Interface\) Name\(\) string](<#Interface.Name>)
- [type Kind](<#Kind>)
- [type List](<#List>)
  - [func \(l \*List\) Kind\(\) Kind](<#List.Kind>)
  - [func \(l \*List\) Name\(\) string](<#List.Name>)
- [type NonNull](<#NonNull>)
  - [func \(n \*NonNull\) Kind\(\) Kind](<#NonNull.Kind>)
  - [func \(n \*NonNull\) Name\(\) string](<#NonNull.Name>)
- [type Object](<#Object>)
  - [func \(o \*Object\) Kind\(\) Kind](<#Object.Kind>)
  - [func \(o \*Object\) Name\(\) string](<#Object.Name>)
- [type ResolveParams](<#ResolveParams>)
- [type Resolver](<#Resolver>)
- [type Scalar](<#Scalar>)
  - [func \(s \*Scalar\) Kind\(\) Kind](<#Scalar.Kind>)
  - [func \(s \*Scalar\) Name\(\) string](<#Scalar.Name>)
- [type Schema](<#Schema>)
  - [func Build\(config Config\) \(\*Schema, error\)](<#Build>)
- [type Type](<#Type>)
- [type Union](<#Union>)
  - [func \(u \*Union\) Kind\(\) Kind](<#Union.Kind>)
  - [func \(u \*Union\) Name\(\) string](<#Union.Name>)


<a name="Builder"></a>
## type Builder

Builder accumulates the type registry while a [Schema](<#Schema>) is being assembled. It is not safe for concurrent use; construct one per Build call.

```go
type Builder struct {
    // contains filtered or unexported fields
}
```

<a name="Config"></a>
## type Config

Config holds the user\-supplied root resolvers that [Build](<#Build>) reflects into a runtime [Schema](<#Schema>). Query is required; Mutation and Subscription are optional.

```go
type Config struct {
    // Query is the root query resolver value. Its exported methods are
    // reflected into fields on the GraphQL Query root type.
    Query any
    // Mutation is the root mutation resolver value. Optional.
    Mutation any
    // Subscription is the root subscription resolver value. Optional.
    Subscription any
}
```

<a name="Field"></a>
## type Field

Field describes a single field on an object, interface, or input object. Args is empty when the field takes no arguments.

```go
type Field struct {
    Name     string
    Type     Type
    Args     []InputValue
    Resolver Resolver
}
```

<a name="InputObject"></a>
## type InputObject

InputObject describes a GraphQL input object type. The Resolver field of each entry in Fields is unused; only Name and Type are meaningful.

```go
type InputObject struct {
    TypeName string
    Fields   map[string]*Field
}
```

<a name="InputObject.Kind"></a>
### func \(\*InputObject\) Kind

```go
func (i *InputObject) Kind() Kind
```

Kind returns [InputObjectKind](<#ScalarKind>).

<a name="InputObject.Name"></a>
### func \(\*InputObject\) Name

```go
func (i *InputObject) Name() string
```

Name returns the input object type name.

<a name="InputValue"></a>
## type InputValue

InputValue describes a single argument or input\-object field. DefaultValue is nil when the input has no default.

```go
type InputValue struct {
    Name         string
    Type         Type
    DefaultValue any
}
```

<a name="Interface"></a>
## type Interface

Interface describes a GraphQL interface type.

```go
type Interface struct {
    TypeName string
    Fields   map[string]*Field
}
```

<a name="Interface.Kind"></a>
### func \(\*Interface\) Kind

```go
func (i *Interface) Kind() Kind
```

Kind returns [InterfaceKind](<#ScalarKind>).

<a name="Interface.Name"></a>
### func \(\*Interface\) Name

```go
func (i *Interface) Name() string
```

Name returns the interface type name.

<a name="Kind"></a>
## type Kind

Kind identifies a GraphQL type kind, matching the introspection \_\_TypeKind enum.

```go
type Kind string
```

<a name="ScalarKind"></a>GraphQL type kinds. See https://spec.graphql.org/October2021/#sec-Types.

```go
const (
    ScalarKind      Kind = "SCALAR"
    ObjectKind      Kind = "OBJECT"
    InterfaceKind   Kind = "INTERFACE"
    UnionKind       Kind = "UNION"
    EnumKind        Kind = "ENUM"
    InputObjectKind Kind = "INPUT_OBJECT"
    ListKind        Kind = "LIST"
    NonNullKind     Kind = "NON_NULL"
)
```

<a name="List"></a>
## type List

List wraps another [Type](<#Type>) to represent a GraphQL list \(\`\[T\]\`\).

```go
type List struct {
    OfType Type
}
```

<a name="List.Kind"></a>
### func \(\*List\) Kind

```go
func (l *List) Kind() Kind
```

Kind returns [ListKind](<#ScalarKind>).

<a name="List.Name"></a>
### func \(\*List\) Name

```go
func (l *List) Name() string
```

Name returns the printable list type name, e.g. "\[Int\]" or "\[User\!\]".

<a name="NonNull"></a>
## type NonNull

NonNull wraps another [Type](<#Type>) to represent a GraphQL non\-null type \(\`T\!\`\). Resolving a NonNull field to nil produces a runtime error.

```go
type NonNull struct {
    OfType Type
}
```

<a name="NonNull.Kind"></a>
### func \(\*NonNull\) Kind

```go
func (n *NonNull) Kind() Kind
```

Kind returns [NonNullKind](<#ScalarKind>).

<a name="NonNull.Name"></a>
### func \(\*NonNull\) Name

```go
func (n *NonNull) Name() string
```

Name returns the printable non\-null type name, e.g. "Int\!" or "\[User\]\!".

<a name="Object"></a>
## type Object

Object describes a GraphQL object type. Fields is keyed by GraphQL field name \(already lowercased from the originating Go method name\).

```go
type Object struct {
    TypeName string
    Fields   map[string]*Field
}
```

<a name="Object.Kind"></a>
### func \(\*Object\) Kind

```go
func (o *Object) Kind() Kind
```

Kind returns [ObjectKind](<#ScalarKind>).

<a name="Object.Name"></a>
### func \(\*Object\) Name

```go
func (o *Object) Name() string
```

Name returns the object type name.

<a name="ResolveParams"></a>
## type ResolveParams

ResolveParams carries the inputs delivered to a [Resolver](<#Resolver>). Source is the parent object value \(nil at the root\) and Args is the validated argument map for the field.

```go
type ResolveParams struct {
    Source any
    Args   map[string]any
}
```

<a name="Resolver"></a>
## type Resolver

Resolver is the function the executor invokes to produce the value of a single field. Implementations should be safe for concurrent use.

```go
type Resolver func(ctx context.Context, params ResolveParams) (any, error)
```

<a name="Scalar"></a>
## type Scalar

Scalar represents a GraphQL scalar type such as String, Int, Float, Boolean, ID, or a custom user\-defined scalar.

```go
type Scalar struct {
    TypeName string
}
```

<a name="Scalar.Kind"></a>
### func \(\*Scalar\) Kind

```go
func (s *Scalar) Kind() Kind
```

Kind returns [ScalarKind](<#ScalarKind>).

<a name="Scalar.Name"></a>
### func \(\*Scalar\) Name

```go
func (s *Scalar) Name() string
```

Name returns the scalar type name.

<a name="Schema"></a>
## type Schema

Schema is the fully built type system the executor operates on. Query is required; Mutation and Subscription are nil when the user did not supply the corresponding root. Types is the type registry keyed by GraphQL type name and is used by the introspection fast\-path.

```go
type Schema struct {
    Query        *Object
    Mutation     *Object
    Subscription *Object
    Types        map[string]Type
}
```

<a name="Build"></a>
### func Build

```go
func Build(config Config) (*Schema, error)
```

Build reflects the supplied resolver values into a runtime [Schema](<#Schema>). It returns an error when the configuration is inconsistent \(e.g. missing Query root, both Schema and root fields supplied\) or when a resolver cannot be reflected into the GraphQL type system.

<a name="Type"></a>
## type Type

Type is the common interface implemented by every GraphQL type. Name returns the printable type name \(e.g. "User", "\[Int\!\]\!"\) and Kind returns the [Kind](<#Kind>) discriminator.

```go
type Type interface {
    Name() string
    Kind() Kind
}
```

<a name="Union"></a>
## type Union

Union describes a GraphQL union type as the set of object types it can resolve to.

```go
type Union struct {
    TypeName string
    Types    []*Object
}
```

<a name="Union.Kind"></a>
### func \(\*Union\) Kind

```go
func (u *Union) Kind() Kind
```

Kind returns [UnionKind](<#ScalarKind>).

<a name="Union.Name"></a>
### func \(\*Union\) Name

```go
func (u *Union) Name() string
```

Name returns the union type name.

Generated by [gomarkdoc](<https://github.com/princjef/gomarkdoc>)
