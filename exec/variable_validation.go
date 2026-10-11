package exec

import (
	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/schema"
	"strings"
)

type variableScope struct {
	Types       map[string]string
	Definitions map[string]variableDefinition
}

func validateVariablePositions(expected schema.Type, value any, defaultValue any, loc core.Location, variables variableScope) []validationError {
	if ref, ok := value.(variableRef); ok {
		declared := variables.Types[ref.Name]
		if declared == "" {
			return nil
		}
		allowedType := expected.Name()
		definition := variables.Definitions[ref.Name]
		if !strings.HasSuffix(declared, "!") && (defaultValue != nil || definition.HasDefault && definition.DefaultValue != nil) {
			allowedType = strings.TrimSuffix(allowedType, "!")
		}
		if !variableTypeAllowed(declared, allowedType) {
			return []validationError{newValidationError(loc, `Variable "$%s" of type "%s" used in position expecting type "%s".`, ref.Name, declared, expected.Name())}
		}
		return nil
	}
	switch typed := expected.(type) {
	case *schema.NonNull:
		return validateVariablePositions(typed.OfType, value, defaultValue, loc, variables)
	case *schema.List:
		if values, ok := value.([]any); ok {
			var errs []validationError
			for _, item := range values {
				errs = append(errs, validateVariablePositions(typed.OfType, item, nil, loc, variables)...)
			}
			return errs
		}
		return validateVariablePositions(typed.OfType, value, nil, loc, variables)
	case *schema.InputObject:
		fields, ok := value.(map[string]any)
		if !ok {
			return nil
		}
		var errs []validationError
		for name, item := range fields {
			if field := typed.Fields[name]; field != nil {
				errs = append(errs, validateVariablePositions(field.Type, item, field.DefaultValue, loc, variables)...)
			}
		}
		return errs
	default:
		return nil
	}
}

func variableTypeFromReference(s *schema.Schema, reference string) schema.Type {
	if strings.HasSuffix(reference, "!") {
		return &schema.NonNull{OfType: variableTypeFromReference(s, strings.TrimSuffix(reference, "!"))}
	}
	if strings.HasPrefix(reference, "[") {
		return &schema.List{OfType: variableTypeFromReference(s, reference[1:len(reference)-1])}
	}
	return s.Types[reference]
}
