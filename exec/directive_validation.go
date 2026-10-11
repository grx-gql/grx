package exec

import (
	"strings"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/schema"
)

func validateDefinedDirectiveArguments(loc core.Location, d directive, definition *schema.DirectiveDefinition, variables variableScope) []validationError {
	var errs []validationError
	field := schema.Field{Args: definition.Args}
	for name, value := range d.Args {
		arg, exists := fieldArg(&field, name)
		if !exists {
			errs = append(errs, newValidationError(loc, `Unknown argument "%s" on directive "@%s".`, name, d.Name))
			continue
		}
		errs = append(errs, validateInputValue(arg.Type, value, loc, name)...)
		errs = append(errs, validateVariablePositions(arg.Type, value, arg.DefaultValue, loc, variables)...)
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
	if _, ok := value.(variableRef); ok {
		return nil, false
	}
	return value, exists
}

func validateDefinitionDirectives(s *schema.Schema, bundle documentBundle, op document, reachable map[string]bool) []validationError {
	var errs []validationError
	if len(op.Directives) > 0 {
		errs = validateDirectives(s, selection{Directives: op.Directives, Location: op.Location}, strings.ToUpper(string(op.Kind)), variableScope{Types: op.VariableTypes, Definitions: op.VariableDefinitions})
	}
	for _, definition := range op.VariableDefinitions {
		if len(definition.Directives) == 0 {
			continue
		}
		errs = append(errs, validateDirectives(s, selection{Directives: definition.Directives, Location: definition.Location}, "VARIABLE_DEFINITION", variableScope{Types: op.VariableTypes, Definitions: op.VariableDefinitions})...)
	}
	for name, fragment := range bundle.fragments {
		if len(fragment.Directives) == 0 || !reachable[name] {
			continue
		}
		errs = append(errs, validateDirectives(s, selection{Directives: fragment.Directives, Location: locationForOffset(bundle.source, fragment.NameOffset)}, "FRAGMENT_DEFINITION", variableScope{Types: op.VariableTypes, Definitions: op.VariableDefinitions})...)
	}
	return errs
}
