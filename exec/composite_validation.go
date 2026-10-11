package exec

import "github.com/grx-gql/grx/schema"

func selectionField(parent schema.Type, name string) (*schema.Field, bool) {
	switch typed := parent.(type) {
	case *schema.Object:
		field, ok := typed.Fields[name]
		return field, ok
	case *schema.Interface:
		field, ok := typed.Fields[name]
		return field, ok
	default:
		return nil, false
	}
}

func compositeTypesOverlap(left schema.Type, right schema.Type) bool {
	if left.Name() == right.Name() {
		return true
	}
	if object, ok := left.(*schema.Object); ok {
		return containsPossibleObject(right, object.Name())
	}
	if object, ok := right.(*schema.Object); ok {
		return containsPossibleObject(left, object.Name())
	}
	for _, object := range possibleObjects(left) {
		if containsPossibleObject(right, object.Name()) {
			return true
		}
	}
	return false
}

func containsPossibleObject(composite schema.Type, name string) bool {
	for _, object := range possibleObjects(composite) {
		if object.Name() == name {
			return true
		}
	}
	return false
}

func possibleObjects(composite schema.Type) []*schema.Object {
	switch typed := composite.(type) {
	case *schema.Interface:
		return typed.PossibleTypes
	case *schema.Union:
		return typed.Types
	default:
		return nil
	}
}
