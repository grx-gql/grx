package schema

import "fmt"

// Required singular links cannot terminate in a finite input value. Nullable
// and list links break the chain because null or an empty list can terminate it.
func validateRequiredInputCycles(types map[string]Type) []SchemaError {
	var errs []SchemaError
	visited := make(map[*InputObject]bool)
	active := make(map[*InputObject]bool)
	var visit func(*InputObject)
	visit = func(input *InputObject) {
		if visited[input] {
			return
		}
		visited[input] = true
		active[input] = true
		for name, field := range input.Fields {
			required, ok := field.Type.(*NonNull)
			if !ok {
				continue
			}
			next, ok := required.OfType.(*InputObject)
			if !ok {
				continue
			}
			if active[next] {
				errs = append(errs, SchemaError{Message: fmt.Sprintf("Input field %s.%s forms a required singular input cycle.", input.TypeName, name)})
			} else {
				visit(next)
			}
		}
		delete(active, input)
	}
	for _, typ := range types {
		if input, ok := typ.(*InputObject); ok {
			visit(input)
		}
	}
	return errs
}
