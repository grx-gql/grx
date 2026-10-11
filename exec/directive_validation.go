package exec

import (
	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/schema"
	"strings"
)

func validateDefinedDirectiveArguments(loc core.Location, d directive, definition *schema.DirectiveDefinition) []validationError {
	var errs []validationError
	field := schema.Field{Args: definition.Args}
	for name, value := range d.Args {
		arg, exists := fieldArg(&field, name)
		if !exists {
			errs = append(errs, newValidationError(loc, `Unknown argument "%s" on directive "@%s".`, name, d.Name))
			continue
		}
		errs = append(errs, validateInputValue(arg.Type, value, loc, name)...)
	}
	for _, arg := range definition.Args {
		if _, present := d.Args[arg.Name]; present || arg.DefaultValue != nil {
			continue
		}
		if _, required := arg.Type.(*schema.NonNull); required {
			errs = append(errs, newValidationError(loc, `Directive "@%s" argument "%s" of type "%s" is required, but it was not provided.`, d.Name, arg.Name, arg.Type.Name()))
		}
	}
	return errs
}

func directiveArgumentValue(args map[string]any, name string) (any, bool) {
	value, exists := args[name]
	if ref, ok := value.(variableRef); ok {
		return ref.Value, ref.HasValue
	}
	return value, exists
}

func validateDefinitionDirectives(s *schema.Schema, bundle documentBundle, op document) []validationError {
	errs := validateDirectives(s, selection{Directives: op.Directives, Location: op.Location}, strings.ToUpper(string(op.Kind)))
	for _, definition := range op.VariableDefinitions {
		errs = append(errs, validateDirectives(s, selection{Directives: definition.Directives, Location: definition.Location}, "VARIABLE_DEFINITION")...)
	}
	for _, fragment := range bundle.fragments {
		errs = append(errs, validateDirectives(s, selection{Directives: fragment.Directives, Location: locationForOffset(bundle.source, fragment.NameOffset)}, "FRAGMENT_DEFINITION")...)
	}
	return errs
}
