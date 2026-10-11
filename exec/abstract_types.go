package exec

import (
	"fmt"
	"github.com/grx-gql/grx/schema"
)

// resolveAbstractType resolves the concrete object type for an interface or
// union value, consulting the configured AbstractTypeResolver first and falling
// back to the schema's default reflection-based resolver.
func (e *Executor) resolveAbstractType(abstractName string, value any, fallback func(any) (*schema.Object, error)) (*schema.Object, error) {
	if e.abstractTypeResolver != nil {
		name, err := e.abstractTypeResolver(value)
		if err != nil {
			return nil, err
		}
		if name != "" {
			typ, ok := e.Schema.Types[name]
			if !ok {
				return nil, fmt.Errorf("abstract type resolver returned unknown type %q for %s", name, abstractName)
			}
			object, ok := typ.(*schema.Object)
			if !ok {
				return nil, fmt.Errorf("abstract type resolver returned non-object type %q for %s", name, abstractName)
			}
			if !e.matchesTypeCondition(object, abstractName) {
				return nil, fmt.Errorf("abstract type resolver returned type %q which is not a possible type of %s", name, abstractName)
			}
			return object, nil
		}
	}
	return fallback(value)
}

// matchesTypeCondition includes union membership and interface possible types
// while retaining the direct object/interface fast path.
func (e *Executor) matchesTypeCondition(object *schema.Object, condition string) bool {
	if fragmentTypeMatches(object, condition) {
		return true
	}
	if object == nil || e.Schema == nil {
		return false
	}
	var possible []*schema.Object
	switch typed := e.Schema.Types[condition].(type) {
	case *schema.Union:
		possible = typed.Types
	case *schema.Interface:
		possible = typed.PossibleTypes
	default:
		return false
	}
	for _, candidate := range possible {
		if candidate != nil && candidate.Name() == object.Name() {
			return true
		}
	}
	return false
}
