package exec

import "testing"

var fuzzSeedQueries = []string{
	`{ user { id name } }`,
	`query Q($id: ID!) { user(id: $id) { id ... on User @defer { name } } }`,
	`mutation { createUser(input: {name: "Ada"}) { user { id } } }`,
	`{ items @stream(initialCount: 2) }`,
	`fragment F on User { id } { user { ...F } }`,
	`{ a(x: 1, y: 2.5, z: "s\nA", b: true, n: null, e: ENUM, l: [1,2], o: {k: 1}) }`,
	"{\r\n  user # comment\r\n  { id }\n}",
	`{ user { name(arg: """` + "\n  block\n  string\n" + `""") } }`,
	"\uFEFF{ a }",
	`subscription { onEvent { id } }`,
}

// FuzzParseDocument ensures the parser never panics on arbitrary input. Parse
// errors are an acceptable outcome; a panic is not.
func FuzzParseDocument(f *testing.F) {
	for _, seed := range fuzzSeedQueries {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, query string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("parseDocument panicked on %q: %v", query, r)
			}
		}()
		_, _ = parseDocument(query, nil)
	})
}

// FuzzLex ensures the lexer never panics and that successful tokenisation always
// terminates with an EOF token.
func FuzzLex(f *testing.F) {
	for _, seed := range fuzzSeedQueries {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, src string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("lex panicked on %q: %v", src, r)
			}
		}()
		tokens, err := lex(src)
		if err != nil {
			return
		}
		if len(tokens) == 0 || tokens[len(tokens)-1].kind != tokenEOF {
			t.Fatalf("expected trailing EOF token for %q", src)
		}
	})
}

// FuzzParseSDL ensures the SDL parser never panics on arbitrary input.
func FuzzParseSDL(f *testing.F) {
	f.Add(`type Query { user: User } type User { id: ID! name: String }`)
	f.Add(`scalar DateTime @specifiedBy(url: "https://example.com")`)
	f.Add(`interface Node { id: ID! } type User implements Node { id: ID! }`)
	f.Add(`enum Episode { NEWHOPE EMPIRE JEDI }`)
	f.Fuzz(func(t *testing.T, src string) {
		defer func() {
			if r := recover(); r != nil {
				t.Fatalf("ParseSDL panicked on %q: %v", src, r)
			}
		}()
		_, _ = ParseSDL(src)
	})
}
