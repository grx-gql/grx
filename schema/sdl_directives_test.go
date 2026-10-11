package schema_test

import (
	"github.com/grx-gql/grx/exec"
	"github.com/grx-gql/grx/schema"
	"strings"
	"testing"
)

func TestRoadmapAuditSDLPrinterRoundTrip(t *testing.T) {
	s, err := exec.ParseSDL(`directive @tag(name: String) repeatable on OBJECT | FIELD_DEFINITION
type Query @tag(name: "root") { ok: String @tag(name: "field") }`)
	if err != nil {
		t.Fatal(err)
	}
	printed := schema.PrintSDL(s)
	if !strings.Contains(printed, "directive @tag") || !strings.Contains(printed, `@tag(name: "root")`) || !strings.Contains(printed, `@tag(name: "field")`) {
		t.Fatal("SDL printer discarded custom directive definitions or applications")
	}
}

func TestSDLDirectiveRoundTripAcrossSchemaElements(t *testing.T) {
	source := `
 "Tag description" directive @tag("Argument description" value: Options = {colors: [RED], note: null}, label: String) repeatable on SCHEMA | SCALAR | ENUM | ENUM_VALUE | OBJECT | FIELD_DEFINITION | ARGUMENT_DEFINITION | INPUT_OBJECT | INPUT_FIELD_DEFINITION | INTERFACE | UNION
 input Options { colors: [Color!] note: String }
 schema @tag(value: {colors: [RED, BLUE], note: null}, label: "schema") { query: Query }
 scalar Date @specifiedBy(url: "https://example.com/date") @tag
 enum Color @tag { RED @tag BLUE @deprecated(reason: "old") }
 interface Node @tag { name(old: String @deprecated @tag): String @tag }
 type Query implements Node @tag(label: "one") @tag(label: "two") { name(old: String @deprecated @tag): String @tag now: Date }
 input Filter @oneOf @tag { value: String @tag }
 union Result @tag = Query
 `
	original, err := exec.ParseSDL(source)
	if err != nil {
		t.Fatal(err)
	}
	printed := schema.PrintSDL(original)
	restored, err := exec.ParseSDL(printed)
	if err != nil {
		t.Fatalf("reparse: %v\n%s", err, printed)
	}
	if printed != schema.PrintSDL(restored) {
		t.Fatalf("SDL metadata did not survive a round trip\n%s\n%s", printed, schema.PrintSDL(restored))
	}
	if len(restored.Directives) != 1 || len(restored.Query.Directives) != 2 || len(restored.Types["Node"].(*schema.Interface).Fields["name"].Directives) != 1 || len(restored.Types["Filter"].(*schema.InputObject).Fields["value"].Directives) != 1 {
		t.Fatal("applied directives were lost")
	}
	if !restored.DirectiveDefinitions["tag"].IsRepeatable || restored.DirectiveDefinitions["tag"].Args[0].Description != "Argument description" {
		t.Fatal("directive definition metadata was lost")
	}
}

func TestSDLArgumentMetadataWithoutApplications(t *testing.T) {
	reason := "use replacement"
	s := &schema.Schema{Types: map[string]schema.Type{"Query": &schema.Object{TypeName: "Query", Fields: map[string]*schema.Field{
		"value": {Name: "value", Type: &schema.Scalar{TypeName: "String"}, Args: []schema.InputValue{{Name: "old", Description: "old argument", Type: &schema.Scalar{TypeName: "String"}, IsDeprecated: true, DeprecationReason: &reason}}},
	}}}}
	printed := schema.PrintSDL(s)
	if !strings.Contains(printed, `old: String @deprecated(reason: "use replacement")`) {
		t.Fatalf("argument deprecation missing: %s", printed)
	}
}
