package schema

import (
	"reflect"
	"testing"
)

func TestSchemaTypeHelpersAndResolution(t *testing.T) {
	scalar := &Scalar{TypeName: "Custom", serializeFn: func(v any) (any, error) { return "x", nil }}
	if scalar.Kind() != ScalarKind || scalar.Name() != "Custom" {
		t.Fatalf("scalar metadata mismatch")
	}
	if got, err := scalar.Serialize(1); err != nil || got != "x" {
		t.Fatalf("scalar serialize = %#v %v", got, err)
	}

	list := &List{OfType: scalar}
	nonNull := &NonNull{OfType: list}
	if list.Name() != "[Custom]" || list.Kind() != ListKind || nonNull.Name() != "[Custom]!" || nonNull.Kind() != NonNullKind {
		t.Fatalf("wrapped type names: %s %s", list.Name(), nonNull.Name())
	}

	obj := &Object{TypeName: "CoverUser", goType: reflect.TypeOf(coverUser{})}
	if obj.Kind() != ObjectKind || obj.ReflectType() != reflect.TypeOf(coverUser{}) {
		t.Fatalf("object metadata mismatch")
	}
	iface := &Interface{TypeName: "Node", PossibleTypes: []*Object{obj}}
	union := &Union{TypeName: "Search", Types: []*Object{obj}}
	if iface.Name() != "Node" || iface.Kind() != InterfaceKind || union.Name() != "Search" || union.Kind() != UnionKind {
		t.Fatal("abstract metadata mismatch")
	}
	if resolved, err := iface.Resolve(coverUser{}); err != nil || resolved != obj {
		t.Fatalf("interface resolve = %#v %v", resolved, err)
	}
	if resolved, err := union.Resolve(&coverUser{}); err != nil || resolved != obj {
		t.Fatalf("union resolve = %#v %v", resolved, err)
	}
	if _, err := union.Resolve(struct{}{}); err == nil {
		t.Fatal("expected unknown possible type")
	}

	input := &InputObject{TypeName: "CoverInput"}
	if input.Name() != "CoverInput" || input.Kind() != InputObjectKind {
		t.Fatal("input metadata mismatch")
	}
}

func TestEnumParseSerializeBranches(t *testing.T) {
	enum := &Enum{
		TypeName: "Role",
		valueByName: map[string]any{
			"ADMIN": coverAdmin,
		},
		nameByValue: map[string]string{
			enumValueKey(coverAdmin): "ADMIN",
		},
	}
	if parsed, err := enum.Parse("ADMIN"); err != nil || parsed != coverAdmin {
		t.Fatalf("parse string = %#v %v", parsed, err)
	}
	if parsed, err := enum.Parse(coverAdmin); err != nil || parsed != coverAdmin {
		t.Fatalf("parse value = %#v %v", parsed, err)
	}
	if _, err := enum.Parse("USER"); err == nil {
		t.Fatal("expected invalid enum input")
	}
	if serialized, err := enum.Serialize(coverAdmin); err != nil || serialized != "ADMIN" {
		t.Fatalf("serialize = %#v %v", serialized, err)
	}
	if _, err := enum.Serialize("USER"); err == nil {
		t.Fatal("expected invalid enum output")
	}
}
