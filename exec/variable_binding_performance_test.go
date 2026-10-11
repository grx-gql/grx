package exec

import "testing"

func TestVariableBindingReusesArgumentFreeSelections(t *testing.T) {
	bundle, err := parseDocumentBundle(`query Q($id:String!){ item(id:$id){ id nested { name } ...Row } } fragment Row on Item { title }`, map[string]any{"id": "value"}, 0)
	if err != nil {
		t.Fatal(err)
	}
	original := bundle.operations[0]
	bound, err := selectOperation(bundle, "Q")
	if err != nil {
		t.Fatal(err)
	}
	if &bound.Selections[0] == &original.Selections[0] {
		t.Fatal("variable-bearing root must be copied")
	}
	if &bound.Selections[0].Selections[0] != &original.Selections[0].Selections[0] {
		t.Fatal("argument-free descendants should retain their immutable syntax")
	}
	if bound.Fragments["Row"] != bundle.fragments["Row"] {
		t.Fatal("argument-free fragment should retain its immutable syntax")
	}
	if got := bound.Selections[0].Arguments["id"].(variableRef).Value; got != "value" {
		t.Fatalf("bound value = %v", got)
	}
	if original.Selections[0].Arguments["id"].(variableRef).HasValue {
		t.Fatal("binding mutated original syntax")
	}
}

func TestParserBindingKeepsVariableContainersIsolated(t *testing.T) {
	type namedInput map[string]any
	type namedList []any
	original := namedInput{"items": namedList{"original"}}
	provided := map[string]any{"value": original}
	doc, err := parseDocument(`query Q($value:JSON){ first:echo(value:$value) second:echo(value:$value) }`, provided)
	if err != nil {
		t.Fatal(err)
	}
	first := doc.Selections[0].Arguments["value"].(namedInput)
	second := doc.Selections[1].Arguments["value"].(namedInput)
	first["items"].(namedList)[0] = "changed"
	if original["items"].(namedList)[0] != "original" || second["items"].(namedList)[0] != "original" {
		t.Fatal("parser binding aliases caller or another argument container")
	}
}
