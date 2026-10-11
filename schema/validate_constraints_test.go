package schema

import "testing"

func TestSchemaInterfaceContracts(t *testing.T) {
	scalar := &Scalar{TypeName: "String"}
	integer := &Scalar{TypeName: "Int"}
	for _, tc := range []struct {
		name                     string
		actual, expected         Type
		actualArgs, expectedArgs []InputValue
		valid                    bool
	}{
		{name: "equal", actual: scalar, expected: scalar, valid: true},
		{name: "nullable strengthening", actual: &NonNull{OfType: scalar}, expected: scalar, valid: true},
		{name: "nullable weakening", actual: scalar, expected: &NonNull{OfType: scalar}},
		{name: "list strengthening", actual: &List{OfType: &NonNull{OfType: scalar}}, expected: &List{OfType: scalar}, valid: true},
		{name: "nonnull covariance", actual: &NonNull{OfType: &List{OfType: &NonNull{OfType: scalar}}}, expected: &NonNull{OfType: &List{OfType: scalar}}, valid: true},
		{name: "list mismatch", actual: scalar, expected: &List{OfType: scalar}},
		{name: "type mismatch", actual: integer, expected: scalar},
		{name: "argument mismatch", actual: scalar, expected: scalar, actualArgs: []InputValue{{Name: "id", Type: integer}}, expectedArgs: []InputValue{{Name: "id", Type: scalar}}},
		{name: "argument missing", actual: scalar, expected: scalar, expectedArgs: []InputValue{{Name: "id", Type: scalar}}},
		{name: "argument equal", actual: scalar, expected: scalar, actualArgs: []InputValue{{Name: "id", Type: scalar}}, expectedArgs: []InputValue{{Name: "id", Type: scalar}}, valid: true},
		{name: "extra optional argument", actual: scalar, expected: scalar, actualArgs: []InputValue{{Name: "extra", Type: scalar}}, valid: true},
		{name: "extra required argument", actual: scalar, expected: scalar, actualArgs: []InputValue{{Name: "extra", Type: &NonNull{OfType: scalar}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			iface := &Interface{TypeName: "Node", Fields: map[string]*Field{"field": {Name: "field", Type: tc.expected, Args: tc.expectedArgs}}}
			obj := &Object{TypeName: "Query", Fields: map[string]*Field{"field": {Name: "field", Type: tc.actual, Args: tc.actualArgs}}, Interfaces: []*Interface{iface}}
			errs := ValidateSchema(&Schema{Query: obj, Types: map[string]Type{"Node": iface, "Query": obj}})
			if (len(errs) == 0) != tc.valid {
				t.Fatalf("valid=%v errors=%v", tc.valid, errs)
			}
		})
	}
}

func TestSchemaAbstractReturnCovariance(t *testing.T) {
	scalar := &Scalar{TypeName: "String"}
	node := &Interface{TypeName: "Node"}
	resource := &Interface{TypeName: "Resource", Interfaces: []*Interface{node}}
	object := &Object{TypeName: "User", Interfaces: []*Interface{node}}
	union := &Union{TypeName: "Result", Types: []*Object{object}}
	for _, tc := range []struct {
		actual, expected Type
		valid            bool
	}{
		{object, node, true}, {resource, node, true}, {object, union, true}, {scalar, node, false}, {scalar, union, false}, {nil, scalar, false},
	} {
		if got := isOutputSubtype(tc.actual, tc.expected); got != tc.valid {
			t.Fatalf("subtype(%T,%T)=%v", tc.actual, tc.expected, got)
		}
	}
}

func TestSchemaValidationConstraintBoundaries(t *testing.T) {
	scalar := &Scalar{TypeName: "String"}
	node := &Interface{TypeName: "Node", Fields: map[string]*Field{"name": {Type: scalar}}}
	inherited := &Interface{TypeName: "Inherited", Fields: node.Fields, Interfaces: []*Interface{node}}
	for _, tc := range []struct {
		name string
		typ  Type
	}{
		{"duplicate interfaces", &Object{TypeName: "Query", Fields: node.Fields, Interfaces: []*Interface{node, node}}},
		{"missing inherited", &Object{TypeName: "Query", Fields: node.Fields, Interfaces: []*Interface{inherited}}},
		{"self interface", &Interface{TypeName: "Node", Fields: node.Fields, Interfaces: []*Interface{node}}},
		{"duplicate arguments", &Object{TypeName: "Query", Fields: map[string]*Field{"name": {Type: scalar, Args: []InputValue{{Name: "id", Type: scalar}, {Name: "id", Type: scalar}}}}}},
		{"double output nonnull", &Object{TypeName: "Query", Fields: map[string]*Field{"name": {Type: &NonNull{OfType: &NonNull{OfType: scalar}}}}}},
		{"double input nonnull", &InputObject{TypeName: "Input", Fields: map[string]*Field{"name": {Type: &NonNull{OfType: &NonNull{OfType: scalar}}}}}},
		{"deprecated required input", &InputObject{TypeName: "Input", Fields: map[string]*Field{"name": {Type: &NonNull{OfType: scalar}, IsDeprecated: true}}}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if errs := ValidateSchema(&Schema{Types: map[string]Type{tc.typ.Name(): tc.typ}}); len(errs) == 0 {
				t.Fatal("invalid constraint accepted")
			}
		})
	}
	d := &DirectiveDefinition{Name: "__private", Args: []InputValue{{Name: "__arg", Type: node}}}
	if errs := ValidateSchema(&Schema{DirectiveDefinitions: map[string]*DirectiveDefinition{d.Name: d}}); len(errs) != 3 {
		t.Fatalf("directive errors=%v", errs)
	}
}

func TestBuiltIntrospectionTypesRemainValid(t *testing.T) {
	s, err := Build(Config{Query: coverQuery{}})
	if err != nil {
		t.Fatal(err)
	}
	if errs := ValidateSchema(s); len(errs) != 0 {
		t.Fatal(errs)
	}
	s.Types["__Type"] = &Scalar{TypeName: "__Type"}
	if errs := ValidateSchema(s); len(errs) == 0 {
		t.Fatal("replaced system type accepted")
	}
}
