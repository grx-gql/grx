package exec

import "testing"

// Fixtures derived from the GraphQL October 2021 specification
// (https://spec.graphql.org/October2021/). Each case asserts that a source
// document either parses or is rejected, exercising lexical and grammatical
// rules in one table so spec coverage is visible in a single place.

func TestSpecFixturesParseable(t *testing.T) {
	valid := []struct {
		name  string
		query string
	}{
		{"anonymous shorthand", `{ field }`},
		{"named query", `query Named { field }`},
		{"named mutation", `mutation M { do }`},
		{"named subscription", `subscription S { ev }`},
		{"variable definitions", `query ($x: Int = 3, $y: [String!]!) { f(a: $x) }`},
		{"field alias", `{ alias: field }`},
		{"nested selection", `{ a { b { c } } }`},
		{"fragment spread and def", `{ user { ...F } } fragment F on User { id }`},
		{"inline fragment", `{ user { ... on User { id } } }`},
		{"inline fragment no type", `{ user { ... { id } } }`},
		{"directives", `{ field @skip(if: true) @include(if: false) }`},
		{"list and object values", `{ f(list: [1, 2, 3], obj: {a: 1, b: "x"}) }`},
		{"enum and bool and null", `{ f(e: ACTIVE, b: true, n: null) }`},
		{"block string argument", "{ f(s: \"\"\"\n  hello\n  world\n\"\"\") }"},
		{"string escapes", `{ f(s: "tab\tnewline\nquote\"unicodeé") }`},
		{"variable-width unicode escape", `{ f(s: "\u{1F600}") }`},
		{"negative and float numbers", `{ f(i: -5, big: 1.5e10, small: -2.0E-3) }`},
		{"comments ignored", "# leading\n{ field # trailing\n}"},
		{"commas as whitespace", `{ f(a: 1, b: 2,, c: 3) }`},
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseDocument(tc.query, nil); err != nil {
				t.Fatalf("expected %q to parse, got error: %v", tc.query, err)
			}
		})
	}
}

func TestSpecFixturesRejected(t *testing.T) {
	invalid := []struct {
		name  string
		query string
	}{
		{"unterminated selection", `{ field`},
		{"unterminated string", `{ f(s: "open) }`},
		{"leading zero int", `{ f(i: 01) }`},
		{"float missing fraction digits", `{ f(x: 1.) }`},
		{"exponent missing digits", `{ f(x: 1e) }`},
		{"name immediately after number", `{ f(x: 1abc) }`},
		{"lone dot is not spread", `{ f(x: .) }`},
		{"duplicate argument", `{ f(a: 1, a: 2) }`},
		{"duplicate input field", `{ f(o: {k: 1, k: 2}) }`},
		{"empty document", ``},
		{"invalid escape", `{ f(s: "\q") }`},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := parseDocument(tc.query, nil); err == nil {
				t.Fatalf("expected %q to be rejected, but it parsed", tc.query)
			}
		})
	}
}

func TestSpecFixturesSDLParseable(t *testing.T) {
	valid := []struct {
		name string
		sdl  string
	}{
		{"object with args", `type Query { user(id: ID!): User } type User { id: ID! }`},
		{"description and implements", `"""Doc""" type User implements Node & Timestamped { id: ID! } interface Node { id: ID! } interface Timestamped { id: ID! }`},
		{"interface", `interface Node { id: ID! } type User implements Node { id: ID! }`},
		{"union", `union SearchResult = User | Post type User { id: ID! } type Post { id: ID! }`},
		{"enum", `enum Episode { NEWHOPE EMPIRE JEDI }`},
		{"custom scalar specifiedBy", `scalar DateTime @specifiedBy(url: "https://example.com/datetime")`},
		{"oneOf input", `input Filter @oneOf { byId: ID byName: String }`},
		{"repeatable directive def", `directive @auth(role: String!) repeatable on FIELD_DEFINITION`},
		{"type extension", `type Query { health: String } extend type Query { version: String }`},
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := ParseSDL(tc.sdl); err != nil {
				t.Fatalf("expected SDL %q to parse, got: %v", tc.sdl, err)
			}
		})
	}
}
