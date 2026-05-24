package core

import "testing"

func TestOrderedObjectSetFieldsAndMap(t *testing.T) {
	obj := NewOrderedObject(2)
	obj.Set("a", 1)
	obj.Set("b", 2)
	obj.Set("b", 3)
	obj.Set("a", 4)

	fields := obj.Fields()
	if len(fields) != 2 {
		t.Fatalf("fields len = %d", len(fields))
	}
	if fields[0].Name != "a" || fields[0].Value != 4 {
		t.Fatalf("first field = %#v", fields[0])
	}
	if got := obj.Map()["b"]; got != 3 {
		t.Fatalf("map b = %#v", got)
	}
}
