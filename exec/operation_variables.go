package exec

import (
	"strings"

	"github.com/grx-gql/grx/schema"
)

// coerceOperationVariables checks the selected operation before any resolver
// runs. References carry their coerced state through argument completion.
func coerceOperationVariables(s *schema.Schema, doc document, provided map[string]any) (document, error) {
	if len(doc.Variables) == 0 {
		return doc, nil
	}
	values := make(map[string]any, len(doc.Variables))
	for _, name := range doc.Variables {
		definition := doc.VariableDefinitions[name]
		raw, present := provided[name]
		if !present && definition.HasDefault {
			raw, present = definition.DefaultValue, true
		}
		typeReference := doc.VariableTypes[name]
		required := strings.HasSuffix(typeReference, "!")
		if required && (!present || raw == nil) {
			return document{}, newValidationError(definition.Location, `Variable "$%s" of required type "%s" must be provided and must not be null.`, name, typeReference)
		}
		if !present {
			continue
		}
		inputType := variableTypeFromReference(s, strings.TrimSuffix(typeReference, "!"))
		value, err := coerceInputValueUsing(inputType, raw, coerceVariableScalarInput)
		if err != nil {
			return document{}, newValidationError(definition.Location, `Variable "$%s" of type "%s": %s`, name, typeReference, err.Error())
		}
		if required && value == nil {
			return document{}, newValidationError(definition.Location, `Variable "$%s" of type "%s": expected non-null %s`, name, typeReference, typeReference)
		}
		values[name] = variableRef{Name: name, Value: value, HasValue: true, Coerced: true}
	}
	return bindOperationVariables(doc, values), nil
}

func coerceVariableScalarInput(scalar *schema.Scalar, raw any) (any, error) {
	if raw == nil {
		return nil, nil
	}
	value, err := coerceScalarInput(scalar.TypeName, raw)
	if err != nil {
		return nil, err
	}
	return scalar.Parse(cloneVariableInput(value))
}
