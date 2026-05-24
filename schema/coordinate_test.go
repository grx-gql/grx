package schema_test

import (
	"testing"

	"github.com/grx-gql/grx/exec"
	"github.com/grx-gql/grx/schema"
)

func coordSchema(t *testing.T) *schema.Schema {
	t.Helper()
	s, err := exec.ParseSDL(`
		type Query { user(id: ID!, role: Role): User }
		type User implements Node { id: ID! name: String }
		interface Node { id: ID! }
		enum Role { ADMIN USER }
		input Filter { term: String }
		directive @auth(role: String!) on FIELD_DEFINITION
	`)
	if err != nil {
		t.Fatalf("ParseSDL: %v", err)
	}
	return s
}

func TestResolveCoordinateForms(t *testing.T) {
	s := coordSchema(t)

	if v, err := s.ResolveCoordinate("User"); err != nil {
		t.Fatalf("User: %v", err)
	} else if _, ok := v.(*schema.Object); !ok {
		t.Fatalf("User = %T, want *schema.Object", v)
	}

	if v, err := s.ResolveCoordinate("User.name"); err != nil {
		t.Fatalf("User.name: %v", err)
	} else if f, ok := v.(*schema.Field); !ok || f.Name != "name" {
		t.Fatalf("User.name = %T %v", v, v)
	}

	if v, err := s.ResolveCoordinate("Query.user(id:)"); err != nil {
		t.Fatalf("Query.user(id:): %v", err)
	} else if a, ok := v.(schema.InputValue); !ok || a.Name != "id" {
		t.Fatalf("Query.user(id:) = %T %v", v, v)
	}

	// trailing colon optional
	if _, err := s.ResolveCoordinate("Query.user(role)"); err != nil {
		t.Fatalf("Query.user(role): %v", err)
	}

	if v, err := s.ResolveCoordinate("Role.ADMIN"); err != nil {
		t.Fatalf("Role.ADMIN: %v", err)
	} else if ev, ok := v.(schema.EnumValue); !ok || ev.Name != "ADMIN" {
		t.Fatalf("Role.ADMIN = %T %v", v, v)
	}

	if v, err := s.ResolveCoordinate("Node.id"); err != nil {
		t.Fatalf("Node.id: %v", err)
	} else if f, ok := v.(*schema.Field); !ok || f.Name != "id" {
		t.Fatalf("Node.id = %T", v)
	}

	if v, err := s.ResolveCoordinate("@auth"); err != nil {
		t.Fatalf("@auth: %v", err)
	} else if d, ok := v.(*schema.DirectiveDefinition); !ok || d.Name != "auth" {
		t.Fatalf("@auth = %T %v", v, v)
	}

	if v, err := s.ResolveCoordinate("@auth(role:)"); err != nil {
		t.Fatalf("@auth(role:): %v", err)
	} else if a, ok := v.(schema.InputValue); !ok || a.Name != "role" {
		t.Fatalf("@auth(role:) = %T %v", v, v)
	}
}

func TestResolveCoordinateErrors(t *testing.T) {
	s := coordSchema(t)
	cases := []string{
		"Nope",
		"User.missing",
		"Query.user(bogus:)",
		"@nope",
		"@auth(bogus:)",
		"Role.MISSING",
		"",
		"User.name(x:)",
	}
	for _, c := range cases {
		if _, err := s.ResolveCoordinate(c); err == nil {
			t.Fatalf("expected error for coordinate %q", c)
		}
	}
}

func TestParseCoordinateBranches(t *testing.T) {
	if _, err := schema.ParseCoordinate(""); err == nil {
		t.Fatal("empty coordinate should error")
	}
	if _, err := schema.ParseCoordinate("Foo.bar(noClose"); err == nil {
		t.Fatal("missing closing paren should error")
	}
	if _, err := schema.ParseCoordinate("Foo.field("); err == nil {
		t.Fatal("empty argument name inside parens should error")
	}
	if _, err := schema.ParseCoordinate("Foo.field ()"); err == nil {
		t.Fatal("whitespace-only arg should error")
	}
	if _, err := schema.ParseCoordinate("@"); err == nil {
		t.Fatal("empty directive coordinate should error")
	}
	if _, err := schema.ParseCoordinate("@dir.name"); err == nil {
		t.Fatal("dotted directive name should error")
	}
	if _, err := schema.ParseCoordinate("Foo(a:)"); err == nil {
		t.Fatal("arg on bare type coordinate should error")
	}
	c, err := schema.ParseCoordinate("Foo.field(arg)")
	if err != nil || c.ArgName != "arg" || c.MemberName != "field" {
		t.Fatalf("ParseCoordinate = %#v %v", c, err)
	}
	if _, err := schema.ParseCoordinate("Foo.bar.baz"); err == nil {
		t.Fatal("three-part coordinate should error")
	}
}
