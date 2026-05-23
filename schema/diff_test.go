package schema_test

import (
	"strings"
	"testing"

	"github.com/patrickkabwe/grx/exec"
	"github.com/patrickkabwe/grx/schema"
)

func mustParseSDL(t *testing.T, sdl string) *schema.Schema {
	t.Helper()
	s, err := exec.ParseSDL(sdl)
	if err != nil {
		t.Fatalf("ParseSDL(%q): %v", sdl, err)
	}
	return s
}

// findChange returns the first change whose message contains substr.
func findChange(changes []schema.Change, substr string) (schema.Change, bool) {
	for _, c := range changes {
		if strings.Contains(c.Message, substr) {
			return c, true
		}
	}
	return schema.Change{}, false
}

func TestDiffDetectsBreakingChanges(t *testing.T) {
	oldSchema := mustParseSDL(t, `
		type Query { user(id: ID!): User account: Account }
		type User { id: ID! name: String email: String }
		type Account { id: ID! }
		enum Role { ADMIN USER GUEST }
	`)
	newSchema := mustParseSDL(t, `
		type Query { user(id: ID!, region: String!): User }
		type User { id: ID! name: Int }
		enum Role { ADMIN USER }
	`)

	changes := schema.Diff(oldSchema, newSchema)
	if !schema.HasBreaking(changes) {
		t.Fatal("expected breaking changes")
	}

	wants := []string{
		`Type "Account" was removed`,
		`Field "User.email" was removed`,
		`Field "User.name" changed type from String to Int`,
		`Required argument "region" was added to "Query.user"`,
		`Enum value "GUEST" was removed from "Role"`,
	}
	for _, want := range wants {
		c, ok := findChange(changes, want)
		if !ok {
			t.Fatalf("expected change %q; got %v", want, changes)
		}
		if c.Severity != schema.Breaking {
			t.Fatalf("change %q should be breaking, got %s", want, c.Severity)
		}
	}
}

func TestDiffDetectsNonBreakingAndDangerous(t *testing.T) {
	oldSchema := mustParseSDL(t, `
		type Query { user: User }
		type User { id: ID! }
		union Content = User
	`)
	newSchema := mustParseSDL(t, `
		type Query { user: User health: String }
		type User { id: ID! nickname: String }
		type Post { id: ID! }
		union Content = User | Post
	`)

	changes := schema.Diff(oldSchema, newSchema)
	if schema.HasBreaking(changes) {
		t.Fatalf("did not expect breaking changes, got %v", changes)
	}

	if c, ok := findChange(changes, `Field "Query.health" was added`); !ok || c.Severity != schema.NonBreaking {
		t.Fatalf("expected non-breaking added field, got %v", changes)
	}
	if c, ok := findChange(changes, `Type "Post" was added`); !ok || c.Severity != schema.NonBreaking {
		t.Fatalf("expected non-breaking added type, got %v", changes)
	}
	if c, ok := findChange(changes, `Member "Post" was added to union "Content"`); !ok || c.Severity != schema.Dangerous {
		t.Fatalf("expected dangerous union member addition, got %v", changes)
	}
}

func TestDiffIdenticalSchemasHaveNoChanges(t *testing.T) {
	sdl := `type Query { user: User } type User { id: ID! name: String }`
	changes := schema.Diff(mustParseSDL(t, sdl), mustParseSDL(t, sdl))
	if len(changes) != 0 {
		t.Fatalf("expected no changes for identical schemas, got %v", changes)
	}
}

func TestDiffOptionalArgAdditionIsSafe(t *testing.T) {
	oldSchema := mustParseSDL(t, `type Query { search(q: String!): String }`)
	newSchema := mustParseSDL(t, `type Query { search(q: String!, limit: Int): String }`)
	changes := schema.Diff(oldSchema, newSchema)
	if schema.HasBreaking(changes) {
		t.Fatalf("optional arg addition must be non-breaking, got %v", changes)
	}
	if _, ok := findChange(changes, `Optional argument "limit" was added`); !ok {
		t.Fatalf("expected optional arg addition change, got %v", changes)
	}
}
