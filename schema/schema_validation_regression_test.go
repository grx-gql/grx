package schema_test

import (
	"github.com/grx-gql/grx/exec"
	"github.com/grx-gql/grx/schema"
	"testing"
)

func TestRoadmapAuditBuildRejectsEmptyQuery(t *testing.T) {
	if _, err := schema.Build(schema.Config{Query: struct{}{}}); err == nil {
		t.Fatal("Build accepted an empty query object despite schema validation requirements")
	}
}

func TestRoadmapAuditSchemaValidation(t *testing.T) {
	cases := []struct{ name, source string }{
		{"reserved input field", `type Query { ok: String } input Filter { __hidden: String }`},
		{"reserved interface field", `type Query { ok: String } interface Node { __hidden: String }`},
		{"reserved argument", `type Query { ok(__hidden: String): String }`},
		{"reserved enum value", `type Query { ok: Status } enum Status { __HIDDEN }`},
		{"output input object", `type Query { bad: Filter } input Filter { value: String }`},
		{"input output object", `type Query { bad(arg: Query): String }`},
		{"incompatible interface", `interface Node { id: ID! } type Query implements Node { name: String }`},
		{"oneof nonnull field", `type Query { ok: String } input Filter @oneOf { value: String! }`},
		{"oneof default", `type Query { ok: String } input Filter @oneOf { value: String = "x" }`},
		{"required input cycle", `type Query { ok: String } input Filter { next: Filter! }`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, err := exec.ParseSDL(tc.source)
			if err != nil {
				return
			}
			if errs := schema.ValidateSchema(s); len(errs) == 0 {
				t.Fatalf("invalid schema accepted: %s", tc.source)
			}
		})
	}
}

func TestSchemaValidationAllowsPartialSDLAndFiniteInputs(t *testing.T) {
	for _, source := range []string{
		`input Filter { next: Filter }`,
		`input Filter { next: [Filter!]! }`,
		`input Filter { next: Other! } input Other { next: Filter }`,
		`input Choice @oneOf { text: String code: Int }`,
		`interface Node { id: ID } type Query implements Node { id: ID! }`,
		`interface Node { id: ID } interface Resource implements Node { id: ID } type Query implements Node & Resource { id: ID! }`,
	} {
		t.Run(source, func(t *testing.T) {
			s, err := exec.ParseSDL(source)
			if err != nil {
				t.Fatal(err)
			}
			if errs := schema.ValidateSchema(s); len(errs) != 0 {
				t.Fatal(errs)
			}
		})
	}
}

func TestSchemaValidationRejectsReservedBuiltinTypeName(t *testing.T) {
	s := &schema.Schema{Types: map[string]schema.Type{"__Type": &schema.Scalar{TypeName: "__Type"}}}
	if errs := schema.ValidateSchema(s); len(errs) == 0 {
		t.Fatal("user-defined reserved introspection type accepted")
	}
}
