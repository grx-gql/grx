package exec

import (
	"github.com/grx-gql/grx/schema"
	"testing"
)

func TestRoadmapAuditSDLExtensions(t *testing.T) {
	cases := []struct{ name, source string }{
		{"enum", `type Query { ok: Color } enum Color { RED } extend enum Color { BLUE }`},
		{"interface", `type Query { ok: String } interface Node { id: ID } extend interface Node { name: String }`},
		{"union", `type Query { ok: Result } type A { a: String } type B { b: String } union Result = A extend union Result = B`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, err := ParseSDL(tc.source)
			if err != nil {
				t.Fatal(err)
			}
			switch tc.name {
			case "enum":
				if len(s.Types["Color"].(*schema.Enum).Values) != 2 {
					t.Fatal("enum extension was discarded")
				}
			case "interface":
				if s.Types["Node"].(*schema.Interface).Fields["name"] == nil {
					t.Fatal("interface extension was discarded")
				}
			case "union":
				if len(s.Types["Result"].(*schema.Union).Types) != 2 {
					t.Fatal("union extension was discarded")
				}
			}
		})
	}
}

func TestSDLExtensionDirectivesAndMetadata(t *testing.T) {
	s, err := ParseSDL(`
 directive @tag on OBJECT | INTERFACE | INPUT_OBJECT | ENUM | UNION
 scalar Date
 extend scalar Date @specifiedBy(url: "https://example.com/date")
 type Query { now: Date }
 extend type Query @tag
 interface Node { id: ID }
 extend interface Node @tag
 input Filter { id: ID }
 extend input Filter @tag
 enum Color { RED }
 extend enum Color @tag
 union Result = Query
 extend union Result @tag
 `)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Types["Date"].(*schema.Scalar).SpecifiedByURL; got != "https://example.com/date" {
		t.Fatalf("specifiedByURL = %q", got)
	}
	for name, dirs := range map[string][]schema.AppliedDirective{
		"Query":  s.Query.Directives,
		"Node":   s.Types["Node"].(*schema.Interface).Directives,
		"Filter": s.Types["Filter"].(*schema.InputObject).Directives,
		"Color":  s.Types["Color"].(*schema.Enum).Directives,
		"Result": s.Types["Result"].(*schema.Union).Directives,
	} {
		if len(dirs) != 1 || dirs[0].Name != "tag" {
			t.Fatalf("%s directives = %#v", name, dirs)
		}
	}
}

func TestSDLInvalidExtensionsReturnErrors(t *testing.T) {
	for _, source := range []string{
		`extend scalar Missing @specifiedBy(url: "url")`,
		`scalar Date extend scalar Date`,
		`type Query { ok: String } extend enum Query { VALUE }`,
		`type Query { ok: String } extend type Query { ok: String }`,
		`interface Node { id: ID } extend interface Node { id: ID }`,
		`input Filter { id: ID } extend input Filter { id: ID }`,
		`enum Color { RED } extend enum Color { RED }`,
		`type Query { ok: String } union Result = Query extend union Result = Query`,
		`type Query { ok: String } union Result = Query extend union Result = String`,
		`type Query { ok: String } extend type Query { missing: Unknown }`,
		`interface Node { id: ID } extend interface Node { missing: Unknown }`,
		`input Filter { id: ID } extend input Filter { missing: Unknown }`,
	} {
		t.Run(source, func(t *testing.T) {
			if _, err := ParseSDL(source); err == nil {
				t.Fatal("invalid extension accepted")
			}
		})
	}
}
