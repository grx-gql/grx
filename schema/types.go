// Package schema describes the runtime metadata that the executor uses
// to resolve GraphQL operations. It also exposes the [Build] entry point
// that reflects user-defined Go types into this metadata.
//
// The types here mirror the GraphQL type system from the October 2021
// specification. Reflection is confined to schema-build time so the
// executor's per-request hot path stays allocation-aware.
package schema

import (
	"context"
	"fmt"
	"reflect"
)

// Kind identifies a GraphQL type kind, matching the introspection
// __TypeKind enum.
type Kind string

// GraphQL type kinds. See https://spec.graphql.org/October2021/#sec-Types.
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

// Type is the common interface implemented by every GraphQL type. Name
// returns the printable type name (e.g. "User", "[Int!]!") and Kind
// returns the [Kind] discriminator.
type Type interface {
	Name() string
	Kind() Kind
}

// AppliedDirective represents a directive applied to a schema element with its arguments.
type AppliedDirective struct {
	Name string
	Args map[string]any
}

// DirectiveDefinition describes a directive definition in the schema
// (both built-in and user-defined).
type DirectiveDefinition struct {
	Name         string
	Description  string
	Locations    []string
	Args         []InputValue
	IsRepeatable bool
}

// Scalar represents a GraphQL scalar type such as String, Int, Float,
// Boolean, ID, or a custom user-defined scalar.
type Scalar struct {
	TypeName       string
	Description    string
	SpecifiedByURL string
	serializeFn    func(any) (any, error)
}

// Name returns the scalar type name.
func (s *Scalar) Name() string {
	return s.TypeName
}

// Kind returns [ScalarKind].
func (s *Scalar) Kind() Kind {
	return ScalarKind
}

// Serialize converts a Go scalar value into a GraphQL response value.
func (s *Scalar) Serialize(value any) (any, error) {
	if s.serializeFn == nil {
		return value, nil
	}
	return s.serializeFn(value)
}

// List wraps another [Type] to represent a GraphQL list (`[T]`).
type List struct {
	OfType Type
}

// Name returns the printable list type name, e.g. "[Int]" or "[User!]".
func (l *List) Name() string {
	return "[" + l.OfType.Name() + "]"
}

// Kind returns [ListKind].
func (l *List) Kind() Kind {
	return ListKind
}

// NonNull wraps another [Type] to represent a GraphQL non-null type
// (`T!`). Resolving a NonNull field to nil produces a runtime error.
type NonNull struct {
	OfType Type
}

// Name returns the printable non-null type name, e.g. "Int!" or "[User]!".
func (n *NonNull) Name() string {
	return n.OfType.Name() + "!"
}

// Kind returns [NonNullKind].
func (n *NonNull) Kind() Kind {
	return NonNullKind
}

// EnumValue describes one legal enum member.
type EnumValue struct {
	Name              string
	Description       string
	IsDeprecated      bool
	DeprecationReason *string
}

// Enum represents a GraphQL enum type.
type Enum struct {
	TypeName    string
	Description string
	Values      []EnumValue
	valueByName map[string]any
	nameByValue map[string]string
}

// Name returns the enum type name.
func (e *Enum) Name() string {
	return e.TypeName
}

// Kind returns [EnumKind].
func (e *Enum) Kind() Kind {
	return EnumKind
}

// Parse validates input and returns the registered Go enum value.
func (e *Enum) Parse(input any) (any, error) {
	name, ok := input.(string)
	if !ok {
		if _, exists := e.nameByValue[enumValueKey(input)]; exists {
			return input, nil
		}
		return nil, fmt.Errorf("expected enum %s input to be string, got %T", e.TypeName, input)
	}
	value, ok := e.valueByName[name]
	if !ok {
		return nil, fmt.Errorf("invalid enum %s value %q", e.TypeName, name)
	}
	return value, nil
}

// Serialize converts a Go enum value into its GraphQL enum name.
func (e *Enum) Serialize(value any) (any, error) {
	key := enumValueKey(value)
	name, ok := e.nameByValue[key]
	if !ok {
		return nil, fmt.Errorf("invalid enum %s value %v", e.TypeName, value)
	}
	return name, nil
}

// Resolver is the function the executor invokes to produce the value of
// a single field. Implementations should be safe for concurrent use.
type Resolver func(ctx context.Context, params ResolveParams) (any, error)

// ResolveParams carries the inputs delivered to a [Resolver]. Source is
// the parent object value (nil at the root) and Args is the validated
// argument map for the field.
type ResolveParams struct {
	Source any
	Args   map[string]any
}

// Field describes a single field on an object, interface, or input
// object. Args is empty when the field takes no arguments.
type Field struct {
	Name              string
	Description       string
	Type              Type
	Args              []InputValue
	Resolver          Resolver
	DefaultValue      any
	IsDeprecated      bool
	DeprecationReason *string
	Directives        []AppliedDirective
}

// InputValue describes a single argument or input-object field.
// DefaultValue is nil when the input has no default.
type InputValue struct {
	Name              string
	Description       string
	Type              Type
	DefaultValue      any
	IsDeprecated      bool
	DeprecationReason *string
}

// Object describes a GraphQL object type. Fields is keyed by GraphQL
// field name (already lowercased from the originating Go method name).
type Object struct {
	TypeName    string
	Description string
	Fields      map[string]*Field
	Interfaces  []*Interface
	Directives  []AppliedDirective
	goType      reflect.Type
}

// Name returns the object type name.
func (o *Object) Name() string {
	return o.TypeName
}

// Kind returns [ObjectKind].
func (o *Object) Kind() Kind {
	return ObjectKind
}

// Interface describes a GraphQL interface type.
type Interface struct {
	TypeName      string
	Description   string
	Fields        map[string]*Field
	Interfaces    []*Interface // interfaces this interface implements
	PossibleTypes []*Object
	Directives    []AppliedDirective
}

// Name returns the interface type name.
func (i *Interface) Name() string {
	return i.TypeName
}

// Kind returns [InterfaceKind].
func (i *Interface) Kind() Kind {
	return InterfaceKind
}

// Resolve returns the concrete object metadata for value.
func (i *Interface) Resolve(value any) (*Object, error) {
	return resolvePossibleType(i.TypeName, i.PossibleTypes, value)
}

// Union describes a GraphQL union type as the set of object types it
// can resolve to.
type Union struct {
	TypeName    string
	Description string
	Types       []*Object
	Directives  []AppliedDirective
}

// Name returns the union type name.
func (u *Union) Name() string {
	return u.TypeName
}

// Kind returns [UnionKind].
func (u *Union) Kind() Kind {
	return UnionKind
}

// Resolve returns the concrete object metadata for value.
func (u *Union) Resolve(value any) (*Object, error) {
	return resolvePossibleType(u.TypeName, u.Types, value)
}

// InputObject describes a GraphQL input object type. The Resolver field
// of each entry in Fields is unused; only Name and Type are meaningful.
type InputObject struct {
	TypeName    string
	Description string
	IsOneOf     bool // @oneOf: exactly one field must be provided
	Fields      map[string]*Field
	Directives  []AppliedDirective
}

// Name returns the input object type name.
func (i *InputObject) Name() string {
	return i.TypeName
}

// Kind returns [InputObjectKind].
func (i *InputObject) Kind() Kind {
	return InputObjectKind
}

// Schema is the fully built type system the executor operates on. Query
// is required; Mutation and Subscription are nil when the user did not
// supply the corresponding root. Types is the type registry keyed by
// GraphQL type name and is used by the introspection fast-path.
type Schema struct {
	Query                *Object
	Mutation             *Object
	Subscription         *Object
	Types                map[string]Type
	Directives           []AppliedDirective
	DirectiveDefinitions map[string]*DirectiveDefinition
}

func enumValueKey(value any) string {
	return fmt.Sprintf("%T:%#v", value, value)
}

func resolvePossibleType(typeName string, possibleTypes []*Object, value any) (*Object, error) {
	if value == nil {
		return nil, fmt.Errorf("%s resolved to nil", typeName)
	}

	rawType := reflect.TypeOf(value)
	for rawType.Kind() == reflect.Pointer {
		rawType = rawType.Elem()
	}

	for _, object := range possibleTypes {
		if object.goType == rawType {
			return object, nil
		}
	}

	return nil, fmt.Errorf("no possible type on %s matches %T", typeName, value)
}
