package schema

import (
	"fmt"
	"strings"
)

// SchemaError is a validation error produced by [ValidateSchema].
type SchemaError struct {
	Message string
}

func (e SchemaError) Error() string { return e.Message }

// ValidateSchema checks a built schema for common type-system constraint
// violations. It is called automatically by [Build]; callers building
// schemas manually (e.g. via an SDL parser) should call it explicitly.
func ValidateSchema(s *Schema) []SchemaError {
	if s == nil {
		return nil
	}
	var errs []SchemaError

	for name := range s.Types {
		// Reserved prefix: names starting with "__" are forbidden for user-defined types.
		if strings.HasPrefix(name, "__") && !isBuiltinReservedType(name) {
			errs = append(errs, SchemaError{
				Message: fmt.Sprintf("Type %q must not begin with \"__\", which is reserved by GraphQL introspection.", name),
			})
		}
	}

	for name, t := range s.Types {
		if strings.HasPrefix(name, "__") {
			continue
		}
		switch typed := t.(type) {
		case *Object:
			errs = append(errs, validateObject(typed)...)
		case *Interface:
			errs = append(errs, validateInterface(typed)...)
		case *InputObject:
			errs = append(errs, validateInputObject(typed)...)
		case *Union:
			errs = append(errs, validateUnion(typed)...)
		case *Enum:
			errs = append(errs, validateEnum(typed)...)
		}
	}

	return errs
}

func isBuiltinReservedType(name string) bool {
	switch name {
	case "__Schema", "__Type", "__TypeKind", "__Field", "__InputValue",
		"__EnumValue", "__Directive", "__DirectiveLocation":
		return true
	}
	return false
}

func validateObject(o *Object) []SchemaError {
	var errs []SchemaError
	if len(o.Fields) == 0 {
		errs = append(errs, SchemaError{
			Message: fmt.Sprintf("Object type %q must define one or more fields.", o.TypeName),
		})
	}
	for fname := range o.Fields {
		if strings.HasPrefix(fname, "__") {
			errs = append(errs, SchemaError{
				Message: fmt.Sprintf("Field %q on type %q must not begin with \"__\".", fname, o.TypeName),
			})
		}
	}
	return errs
}

func validateInterface(i *Interface) []SchemaError {
	var errs []SchemaError
	if len(i.Fields) == 0 {
		errs = append(errs, SchemaError{
			Message: fmt.Sprintf("Interface type %q must define one or more fields.", i.TypeName),
		})
	}
	return errs
}

func validateInputObject(io *InputObject) []SchemaError {
	var errs []SchemaError
	if len(io.Fields) == 0 {
		errs = append(errs, SchemaError{
			Message: fmt.Sprintf("Input object type %q must define one or more fields.", io.TypeName),
		})
	}
	return errs
}

func validateUnion(u *Union) []SchemaError {
	var errs []SchemaError
	if len(u.Types) == 0 {
		errs = append(errs, SchemaError{
			Message: fmt.Sprintf("Union type %q must define one or more member types.", u.TypeName),
		})
	}
	return errs
}

func validateEnum(e *Enum) []SchemaError {
	var errs []SchemaError
	if len(e.Values) == 0 {
		errs = append(errs, SchemaError{
			Message: fmt.Sprintf("Enum type %q must define one or more values.", e.TypeName),
		})
	}
	for _, v := range e.Values {
		if v.Name == "true" || v.Name == "false" || v.Name == "null" {
			errs = append(errs, SchemaError{
				Message: fmt.Sprintf("Enum type %q cannot include value %q.", e.TypeName, v.Name),
			})
		}
	}
	return errs
}
