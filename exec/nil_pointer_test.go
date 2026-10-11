package exec

import "testing"

func TestIsNilPointer(t *testing.T) {
	type object struct{ ID string }
	var nilObject *object
	var nilSlice []object
	cases := []struct {
		name  string
		value any
		want  bool
	}{
		{"typed nil pointer", nilObject, true},
		{"non-nil pointer", &object{}, false},
		{"nil slice stays a list", nilSlice, false},
		{"struct value", object{}, false},
		{"string", "", false},
	}
	for _, c := range cases {
		if got := isNilPointer(c.value); got != c.want {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}
