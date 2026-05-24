package schema

import (
	"fmt"
	"strings"
)

// Coordinate is a parsed GraphQL schema coordinate
// (https://spec.graphql.org/draft/#sec-Schema-Coordinates). Exactly one of the
// type-form (TypeName set) or directive-form (DirectiveName set) is populated.
type Coordinate struct {
	// TypeName is the named type for a type coordinate (empty for directives).
	TypeName string
	// MemberName is the field, enum value, or input-field name when present.
	MemberName string
	// ArgName is the argument name when the coordinate addresses an argument.
	ArgName string
	// DirectiveName is set (without the leading "@") for a directive coordinate.
	DirectiveName string
}

// ParseCoordinate parses a schema coordinate string. Supported forms:
//
//	Name                       -> a named type
//	Name.field                 -> a field, enum value, or input field
//	Name.field(arg:)           -> a field/directive argument
//	@directive                 -> a directive definition
//	@directive(arg:)           -> a directive argument
//
// The trailing colon on the argument form is optional.
func ParseCoordinate(coord string) (Coordinate, error) {
	c := strings.TrimSpace(coord)
	if c == "" {
		return Coordinate{}, fmt.Errorf("empty schema coordinate")
	}

	var argName string
	if open := strings.IndexByte(c, '('); open >= 0 {
		if !strings.HasSuffix(c, ")") {
			return Coordinate{}, fmt.Errorf("invalid schema coordinate %q: missing closing ')'", coord)
		}
		argName = strings.TrimSpace(c[open+1 : len(c)-1])
		argName = strings.TrimSuffix(argName, ":")
		argName = strings.TrimSpace(argName)
		if argName == "" {
			return Coordinate{}, fmt.Errorf("invalid schema coordinate %q: empty argument name", coord)
		}
		c = c[:open]
	}

	if strings.HasPrefix(c, "@") {
		name := strings.TrimSpace(c[1:])
		if name == "" || strings.Contains(name, ".") {
			return Coordinate{}, fmt.Errorf("invalid directive coordinate %q", coord)
		}
		return Coordinate{DirectiveName: name, ArgName: argName}, nil
	}

	parts := strings.Split(c, ".")
	switch len(parts) {
	case 1:
		if argName != "" {
			return Coordinate{}, fmt.Errorf("invalid schema coordinate %q: arguments require a field", coord)
		}
		return Coordinate{TypeName: strings.TrimSpace(parts[0])}, nil
	case 2:
		return Coordinate{
			TypeName:   strings.TrimSpace(parts[0]),
			MemberName: strings.TrimSpace(parts[1]),
			ArgName:    argName,
		}, nil
	default:
		return Coordinate{}, fmt.Errorf("invalid schema coordinate %q", coord)
	}
}

// ResolveCoordinate resolves a schema coordinate string against the schema and
// returns the addressed element. The concrete dynamic type is one of:
//
//	Type (*Object, *Interface, *Union, *Enum, *Scalar, *InputObject)
//	*Field        (object/interface field, or input-object field)
//	EnumValue     (enum member)
//	InputValue    (field argument, input field arg, or directive argument)
//	*DirectiveDefinition
//
// It returns an error when any segment of the coordinate does not exist.
func (s *Schema) ResolveCoordinate(coord string) (any, error) {
	c, err := ParseCoordinate(coord)
	if err != nil {
		return nil, err
	}

	if c.DirectiveName != "" {
		def, ok := s.DirectiveDefinitions[c.DirectiveName]
		if !ok || def == nil {
			return nil, fmt.Errorf("unknown directive @%s", c.DirectiveName)
		}
		if c.ArgName == "" {
			return def, nil
		}
		for i := range def.Args {
			if def.Args[i].Name == c.ArgName {
				return def.Args[i], nil
			}
		}
		return nil, fmt.Errorf("directive @%s has no argument %q", c.DirectiveName, c.ArgName)
	}

	typ, ok := s.Types[c.TypeName]
	if !ok {
		return nil, fmt.Errorf("unknown type %q", c.TypeName)
	}
	if c.MemberName == "" {
		return typ, nil
	}

	switch t := typ.(type) {
	case *Object:
		return resolveFieldCoordinate(c, t.Fields[c.MemberName], t.TypeName)
	case *Interface:
		return resolveFieldCoordinate(c, t.Fields[c.MemberName], t.TypeName)
	case *InputObject:
		field := t.Fields[c.MemberName]
		if field == nil {
			return nil, fmt.Errorf("type %q has no input field %q", c.TypeName, c.MemberName)
		}
		if c.ArgName != "" {
			return nil, fmt.Errorf("input field %q.%q has no arguments", c.TypeName, c.MemberName)
		}
		return field, nil
	case *Enum:
		if c.ArgName != "" {
			return nil, fmt.Errorf("enum value %q.%q has no arguments", c.TypeName, c.MemberName)
		}
		for _, v := range t.Values {
			if v.Name == c.MemberName {
				return v, nil
			}
		}
		return nil, fmt.Errorf("enum %q has no value %q", c.TypeName, c.MemberName)
	default:
		return nil, fmt.Errorf("type %q (%s) has no members", c.TypeName, typ.Kind())
	}
}

func resolveFieldCoordinate(c Coordinate, field *Field, typeName string) (any, error) {
	if field == nil {
		return nil, fmt.Errorf("type %q has no field %q", typeName, c.MemberName)
	}
	if c.ArgName == "" {
		return field, nil
	}
	for i := range field.Args {
		if field.Args[i].Name == c.ArgName {
			return field.Args[i], nil
		}
	}
	return nil, fmt.Errorf("field %q.%q has no argument %q", typeName, c.MemberName, c.ArgName)
}
