package exec

import "reflect"

// isNilPointer reports whether value is a typed nil pointer — (*T)(nil) held
// in an interface, which `value == nil` does not catch.
//
// Resolvers return pointers for nullable objects, so "no object" usually
// arrives typed. Without this check completeValue descended into it, every
// subfield answered "received nil source", and under a non-null wrapper those
// errors nulled the parent: a list item with an absent nested object took its
// whole list with it. Only pointers are treated this way — a nil slice is an
// empty list to Go code and must keep serialising as [].
func isNilPointer(value any) bool {
	v := reflect.ValueOf(value)
	return v.Kind() == reflect.Pointer && v.IsNil()
}
