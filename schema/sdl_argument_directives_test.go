package schema_test

import (
	"github.com/grx-gql/grx/exec"
	"github.com/grx-gql/grx/schema"
	"testing"
)

func TestRoadmapAuditArgumentDeprecationPreserved(t *testing.T) {
	s, err := exec.ParseSDL(`type Query { ok(old: String @deprecated(reason: "use new")): String }`)
	if err != nil {
		t.Fatal(err)
	}
	arg := s.Query.Fields["ok"].Args[0]
	if !arg.IsDeprecated || arg.DeprecationReason == nil || *arg.DeprecationReason != "use new" {
		t.Fatalf("argument directive metadata lost: %#v", arg)
	}
}

func TestSDLArgumentMetadataPreservesDefaultsAndDirectives(t *testing.T) {
	s, err := exec.ParseSDL(`
 directive @tag(value: String) repeatable on ARGUMENT_DEFINITION
 directive @optional(value: Int! = 1) on FIELD
 interface Node { find(old: String @deprecated @tag(value: "first") @tag(value: "second")): String }
 type Query { find(old: String @deprecated): String }
 `)
	if err != nil {
		t.Fatal(err)
	}
	if got := s.DirectiveDefinitions["optional"].Args[0].DefaultValue; got != 1 {
		t.Fatalf("directive argument default = %#v, want 1", got)
	}
	arg := s.Types["Node"].(*schema.Interface).Fields["find"].Args[0]
	if !arg.IsDeprecated || arg.DeprecationReason == nil || *arg.DeprecationReason != "No longer supported" {
		t.Fatalf("default deprecation metadata = %#v", arg)
	}
	if len(arg.Directives) != 3 || arg.Directives[1].Args["value"] != "first" || arg.Directives[2].Args["value"] != "second" {
		t.Fatalf("ordered directives = %#v", arg.Directives)
	}
}
