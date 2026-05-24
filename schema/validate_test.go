package schema

import "testing"

func TestValidateSchemaBranches(t *testing.T) {
	errs := ValidateSchema(&Schema{Types: map[string]Type{
		"__Bad":  &Scalar{TypeName: "__Bad"},
		"Empty":  &Object{TypeName: "Empty"},
		"Field":  &Object{TypeName: "Field", Fields: map[string]*Field{"__bad": {Name: "__bad"}}},
		"Iface":  &Interface{TypeName: "Iface"},
		"Input":  &InputObject{TypeName: "Input"},
		"Union":  &Union{TypeName: "Union"},
		"Enum":   &Enum{TypeName: "Enum"},
		"Values": &Enum{TypeName: "Values", Values: []EnumValue{{Name: "true"}, {Name: "OK"}}},
		"__Type": &Object{TypeName: "__Type"},
	}})
	if len(errs) < 7 {
		t.Fatalf("expected validation errors, got %#v", errs)
	}
	if errs[0].Error() == "" {
		t.Fatal("empty schema error string")
	}
	if ValidateSchema(nil) != nil {
		t.Fatal("nil schema should have no errors")
	}
}
