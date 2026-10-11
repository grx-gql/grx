package schema

import (
	"fmt"
	"strings"
)

func validateSchemaName(kind, name string) []SchemaError {
	if strings.HasPrefix(name, "__") {
		return []SchemaError{{Message: fmt.Sprintf("%s %q must not begin with \"__\".", kind, name)}}
	}
	return nil
}

func validateOutputFields(parent string, fields map[string]*Field) []SchemaError {
	var errs []SchemaError
	for name, field := range fields {
		errs = append(errs, validateSchemaName("Field on "+parent, name)...)
		if !isOutputType(field.Type) {
			errs = append(errs, SchemaError{Message: fmt.Sprintf("Field %s.%s must have an output type.", parent, name)})
		}
		seen := make(map[string]bool, len(field.Args))
		for _, arg := range field.Args {
			if seen[arg.Name] {
				errs = append(errs, SchemaError{Message: fmt.Sprintf("Argument %s.%s(%s:) must be unique.", parent, name, arg.Name)})
			}
			seen[arg.Name] = true
			errs = append(errs, validateInputValue(parent+"."+name, arg)...)
		}
	}
	return errs
}

func validateInputValue(parent string, value InputValue) []SchemaError {
	errs := validateSchemaName("Input value on "+parent, value.Name)
	if !isInputType(value.Type) {
		errs = append(errs, SchemaError{Message: fmt.Sprintf("Input value %s.%s must have an input type.", parent, value.Name)})
	}
	if _, required := value.Type.(*NonNull); required && value.DefaultValue == nil && value.IsDeprecated {
		errs = append(errs, SchemaError{Message: fmt.Sprintf("Required input value %s.%s must not be deprecated.", parent, value.Name)})
	}
	return errs
}

func isInputType(t Type) bool {
	switch typed := t.(type) {
	case *NonNull:
		if _, nested := typed.OfType.(*NonNull); nested {
			return false
		}
		return isInputType(typed.OfType)
	case *List:
		return isInputType(typed.OfType)
	case *Scalar, *Enum, *InputObject:
		return true
	default:
		return false
	}
}

func isOutputType(t Type) bool {
	switch typed := t.(type) {
	case *NonNull:
		if _, nested := typed.OfType.(*NonNull); nested {
			return false
		}
		return isOutputType(typed.OfType)
	case *List:
		return isOutputType(typed.OfType)
	case *Scalar, *Enum, *Object, *Interface, *Union:
		return true
	default:
		return false
	}
}
