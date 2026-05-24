package exec

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseDocumentAnonymousQuery(t *testing.T) {
	doc, err := parseDocument(`{ user { id name } }`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if doc.Kind != operationQuery {
		t.Fatalf("expected query operation, got %q", doc.Kind)
	}
	if len(doc.Selections) != 1 {
		t.Fatalf("expected one top-level selection, got %d", len(doc.Selections))
	}

	user := doc.Selections[0]
	if user.Name != "user" {
		t.Fatalf("expected selection name user, got %q", user.Name)
	}
	if len(user.Arguments) != 0 {
		t.Fatalf("expected no arguments, got %#v", user.Arguments)
	}
	if len(user.Selections) != 2 {
		t.Fatalf("expected two nested selections, got %d", len(user.Selections))
	}
	if user.Selections[0].Name != "id" || user.Selections[1].Name != "name" {
		t.Fatalf("unexpected nested selection names: %#v", user.Selections)
	}
}

func TestParseDocumentNamedQueryWithVariables(t *testing.T) {
	query := `query GetUser($id: String!, $verbose: Boolean) {
		user(id: $id, verbose: $verbose) {
			id
			name
		}
	}`

	doc, err := parseDocument(query, map[string]any{
		"id":      "user_42",
		"verbose": true,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if doc.Kind != operationQuery {
		t.Fatalf("expected query operation, got %q", doc.Kind)
	}

	user := doc.Selections[0]
	if user.Name != "user" {
		t.Fatalf("expected user selection, got %q", user.Name)
	}
	if user.Arguments["id"] != "user_42" {
		t.Fatalf("expected id user_42, got %#v", user.Arguments["id"])
	}
	if user.Arguments["verbose"] != true {
		t.Fatalf("expected verbose true, got %#v", user.Arguments["verbose"])
	}
}

func TestParseDocumentMutation(t *testing.T) {
	doc, err := parseDocument(`mutation CreateUser { createUser { id } }`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if doc.Kind != operationMutation {
		t.Fatalf("expected mutation operation, got %q", doc.Kind)
	}
	if doc.Selections[0].Name != "createUser" {
		t.Fatalf("expected createUser, got %q", doc.Selections[0].Name)
	}
}

func TestParseDocumentRejectsDuplicateArguments(t *testing.T) {
	_, err := parseDocument(`{ user(id: "1", id: "2") { id } }`, nil)
	if err == nil {
		t.Fatal("expected duplicate argument error")
	}
	if !strings.Contains(err.Error(), `There can be only one argument named "id".`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDocumentInlineFragmentWithoutTypeCondition(t *testing.T) {
	doc, err := parseDocument(`{ user(id: "1") { ... { id } } }`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	fragment := doc.Selections[0].Selections[0]
	if !fragment.isInlineFragment() {
		t.Fatalf("expected inline fragment, got %#v", fragment)
	}
	if fragment.InlineFragmentOn != "" {
		t.Fatalf("expected empty type condition, got %q", fragment.InlineFragmentOn)
	}
}

func TestParseDocumentMutationWithoutName(t *testing.T) {
	doc, err := parseDocument(`mutation { createUser { id } }`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Kind != operationMutation {
		t.Fatalf("expected mutation operation, got %q", doc.Kind)
	}
}

func TestParseDocumentSubscription(t *testing.T) {
	doc, err := parseDocument(`subscription OnUser { userCreated { id name } }`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Kind != operationSubscription {
		t.Fatalf("expected subscription operation, got %q", doc.Kind)
	}
	if len(doc.Selections) != 1 {
		t.Fatalf("expected one root selection, got %d", len(doc.Selections))
	}
	if doc.Selections[0].Name != "userCreated" {
		t.Fatalf("expected userCreated, got %q", doc.Selections[0].Name)
	}
}

func TestParseDocumentSubscriptionWithoutName(t *testing.T) {
	doc, err := parseDocument(`subscription { userCreated { id } }`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Kind != operationSubscription {
		t.Fatalf("expected subscription operation, got %q", doc.Kind)
	}
}

func TestParseDocumentMultipleOperationsSelectsByName(t *testing.T) {
	query := `mutation MyMutation {
		__typename
		createUser(input: {email: "test@gmail.com", name: "test"}) { user { id email } }
	}

	query MyQuery { __typename }

	subscription MySubscription { userCreated { email } }`

	mutation, err := parseDocumentNamed(query, nil, "MyMutation", 0)
	if err != nil {
		t.Fatalf("unexpected error selecting MyMutation: %v", err)
	}
	if mutation.Kind != operationMutation || mutation.Name != "MyMutation" {
		t.Fatalf("expected mutation MyMutation, got %s %q", mutation.Kind, mutation.Name)
	}
	if len(mutation.Selections) != 2 {
		t.Fatalf("expected 2 selections in mutation, got %d", len(mutation.Selections))
	}

	queryDoc, err := parseDocumentNamed(query, nil, "MyQuery", 0)
	if err != nil {
		t.Fatalf("unexpected error selecting MyQuery: %v", err)
	}
	if queryDoc.Kind != operationQuery || queryDoc.Name != "MyQuery" {
		t.Fatalf("expected query MyQuery, got %s %q", queryDoc.Kind, queryDoc.Name)
	}

	sub, err := parseDocumentNamed(query, nil, "MySubscription", 0)
	if err != nil {
		t.Fatalf("unexpected error selecting MySubscription: %v", err)
	}
	if sub.Kind != operationSubscription || sub.Name != "MySubscription" {
		t.Fatalf("expected subscription MySubscription, got %s %q", sub.Kind, sub.Name)
	}
}

func TestParseDocumentMultipleOperationsRequiresName(t *testing.T) {
	query := `query A { __typename } query B { __typename }`
	if _, err := parseDocumentNamed(query, nil, "", 0); err == nil {
		t.Fatalf("expected error when operationName is missing for multi-op document")
	} else if !strings.Contains(err.Error(), "Must provide operation name if query contains multiple operations") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDocumentUnknownOperationName(t *testing.T) {
	query := `query A { __typename } query B { __typename }`
	if _, err := parseDocumentNamed(query, nil, "C", 0); err == nil {
		t.Fatalf("expected error when operationName does not match any operation")
	} else if !strings.Contains(err.Error(), `"C"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDocumentArgumentScalarValues(t *testing.T) {
	query := `{
		search(
			term: "hello",
			limit: 10,
			score: 1.5,
			active: true,
			disabled: false,
			cursor: null,
			sort: ASC
		) {
			id
		}
	}`

	doc, err := parseDocument(query, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	args := doc.Selections[0].Arguments
	expected := map[string]any{
		"term":     "hello",
		"limit":    10,
		"score":    1.5,
		"active":   true,
		"disabled": false,
		"cursor":   nil,
		"sort":     "ASC",
	}
	if !reflect.DeepEqual(args, expected) {
		t.Fatalf("unexpected arguments.\nexpected: %#v\n     got: %#v", expected, args)
	}
}

func TestParseDocumentArgumentNegativeNumbers(t *testing.T) {
	doc, err := parseDocument(`{ items(offset: -5, ratio: -1.25) { id } }`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	args := doc.Selections[0].Arguments
	if args["offset"] != -5 {
		t.Fatalf("expected offset -5, got %#v", args["offset"])
	}
	if args["ratio"] != -1.25 {
		t.Fatalf("expected ratio -1.25, got %#v", args["ratio"])
	}
}

func TestParseDocumentVariableSubstitution(t *testing.T) {
	doc, err := parseDocument(
		`{ user(filter: $filter) { id } }`,
		map[string]any{"filter": map[string]any{"name": "Ada"}},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, ok := doc.Selections[0].Arguments["filter"].(map[string]any)
	if !ok {
		t.Fatalf("expected filter map, got %T", doc.Selections[0].Arguments["filter"])
	}
	if got["name"] != "Ada" {
		t.Fatalf("expected filter.name Ada, got %#v", got["name"])
	}
}

func TestParseDocumentInlineInputObjectArgument(t *testing.T) {
	doc, err := parseDocument(
		`mutation { createUser(input: {email: "test@gmail.com", name: "test"}) { user { email } } }`,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	createUser := doc.Selections[0]
	input, ok := createUser.Arguments["input"].(map[string]any)
	if !ok {
		t.Fatalf("expected input to be a map, got %T", createUser.Arguments["input"])
	}
	if input["email"] != "test@gmail.com" {
		t.Fatalf("expected input.email test@gmail.com, got %#v", input["email"])
	}
	if input["name"] != "test" {
		t.Fatalf("expected input.name test, got %#v", input["name"])
	}
}

func TestParseDocumentInlineNestedAndListLiterals(t *testing.T) {
	doc, err := parseDocument(
		`{ search(filter: {tags: ["a", "b"], page: {limit: 5, offset: 0}}) { id } }`,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	filter, ok := doc.Selections[0].Arguments["filter"].(map[string]any)
	if !ok {
		t.Fatalf("expected filter map, got %T", doc.Selections[0].Arguments["filter"])
	}
	tags, ok := filter["tags"].([]any)
	if !ok || len(tags) != 2 || tags[0] != "a" || tags[1] != "b" {
		t.Fatalf("unexpected tags: %#v", filter["tags"])
	}
	page, ok := filter["page"].(map[string]any)
	if !ok || page["limit"] != 5 || page["offset"] != 0 {
		t.Fatalf("unexpected page: %#v", filter["page"])
	}
}

func TestParseDocumentNestedSelections(t *testing.T) {
	query := `{
		viewer {
			id
			account {
				balance
				owner { id name }
			}
		}
	}`

	doc, err := parseDocument(query, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	viewer := doc.Selections[0]
	if viewer.Name != "viewer" || len(viewer.Selections) != 2 {
		t.Fatalf("unexpected viewer selection: %#v", viewer)
	}
	account := viewer.Selections[1]
	if account.Name != "account" || len(account.Selections) != 2 {
		t.Fatalf("unexpected account selection: %#v", account)
	}
	owner := account.Selections[1]
	if owner.Name != "owner" || len(owner.Selections) != 2 {
		t.Fatalf("unexpected owner selection: %#v", owner)
	}
	if owner.Selections[0].Name != "id" || owner.Selections[1].Name != "name" {
		t.Fatalf("unexpected nested owner fields: %#v", owner.Selections)
	}
}

func TestParseDocumentCommasAreTreatedAsWhitespace(t *testing.T) {
	doc, err := parseDocument(`{ user(id: 1, limit: 5) { id, name } }`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	user := doc.Selections[0]
	if len(user.Selections) != 2 {
		t.Fatalf("expected two nested selections, got %d", len(user.Selections))
	}
	if user.Arguments["id"] != 1 || user.Arguments["limit"] != 5 {
		t.Fatalf("unexpected arguments: %#v", user.Arguments)
	}
}

func TestParseDocumentErrors(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		variables map[string]any
		contains  string
	}{
		{
			name:     "fragment definitions without operation",
			query:    `fragment Foo on User { id }`,
			contains: `document contains no operations`,
		},
		{
			name:     "missing selection set",
			query:    `query GetUser`,
			contains: `expected token kind`,
		},
		{
			name:     "unterminated selection set",
			query:    `{ user { id `,
			contains: `unexpected end of query inside selection set`,
		},
		{
			name:     "trailing tokens after document",
			query:    `{ user { id } } extra`,
			contains: `unexpected token`,
		},
		{
			name:     "field name expected",
			query:    `{ 123 }`,
			contains: `expected field name`,
		},
		{
			name:     "argument name expected",
			query:    `{ user("id": 1) { id } }`,
			contains: `expected argument name`,
		},
		{
			name:     "missing colon between argument and value",
			query:    `{ user(id 1) { id } }`,
			contains: `expected token kind`,
		},
		{
			name:     "unknown value token",
			query:    `{ user(id: }) { id } }`,
			contains: `unexpected value token`,
		},
		{
			name:     "variable without name",
			query:    `{ user(id: $) { id } }`,
			contains: `expected variable name after $`,
		},
		{
			name:     "unterminated operation variables",
			query:    `query Foo(`,
			contains: `unexpected end of query inside operation variables`,
		},
		{
			name:     "unterminated string literal",
			query:    `{ user(name: "ada) { id } }`,
			contains: `unterminated string literal`,
		},
		{
			name:     "invalid string escape",
			query:    `{ user(name: "a\xb") { id } }`,
			contains: `invalid string escape`,
		},
		{
			name:     "unexpected character",
			query:    `{ user(id: ?) { id } }`,
			contains: `unexpected character`,
		},
		{
			name:     "leading zero number",
			query:    `{ user(id: 0123) { id } }`,
			contains: `leading zeros are not allowed`,
		},
		{
			name:     "number with trailing name",
			query:    `{ user(id: 123abc) { id } }`,
			contains: `invalid number literal`,
		},
		{
			name:     "fraction without digits",
			query:    `{ user(id: 1.) { id } }`,
			contains: `expected digits after '.'`,
		},
		{
			name:     "exponent without digits",
			query:    `{ user(id: 1e) { id } }`,
			contains: `expected digits in exponent`,
		},
		{
			name:     "lone dot",
			query:    `{ user(id: .) { id } }`,
			contains: `unexpected character`,
		},
		{
			name:     "unterminated block string",
			query:    `{ user(name: """oops) { id } }`,
			contains: `unterminated block string`,
		},
		{
			name:     "invalid unicode escape body",
			query:    `{ user(name: "\uZZZZ") { id } }`,
			contains: `invalid unicode escape`,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, err := parseDocument(test.query, test.variables)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", test.contains)
			}
			if !strings.Contains(err.Error(), test.contains) {
				t.Fatalf("expected error containing %q, got %q", test.contains, err.Error())
			}
		})
	}
}

func TestLexProducesExpectedTokens(t *testing.T) {
	tokens, err := lex(`query Foo($id: ID!) { user(id: $id) { name } }`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	expected := []tokenKind{
		tokenName,       // query
		tokenName,       // Foo
		tokenParenOpen,  // (
		tokenDollar,     // $
		tokenName,       // id
		tokenColon,      // :
		tokenName,       // ID
		tokenBang,       // !
		tokenParenClose, // )
		tokenBraceOpen,  // {
		tokenName,       // user
		tokenParenOpen,  // (
		tokenName,       // id
		tokenColon,      // :
		tokenDollar,     // $
		tokenName,       // id
		tokenParenClose, // )
		tokenBraceOpen,  // {
		tokenName,       // name
		tokenBraceClose, // }
		tokenBraceClose, // }
		tokenEOF,
	}

	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d (%#v)", len(expected), len(tokens), tokens)
	}
	for i, kind := range expected {
		if tokens[i].kind != kind {
			t.Fatalf("token %d: expected kind %d, got %d (value %q)", i, kind, tokens[i].kind, tokens[i].value)
		}
	}
}

func TestLexBracketTokens(t *testing.T) {
	tokens, err := lex(`[]`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tokens) != 3 {
		t.Fatalf("expected 3 tokens, got %d", len(tokens))
	}
	if tokens[0].kind != tokenBracketOpen || tokens[1].kind != tokenBracketClose {
		t.Fatalf("unexpected token kinds: %#v", tokens)
	}
}

func TestLexStringLiteral(t *testing.T) {
	tokens, err := lex(`"hello world"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tokens[0].kind != tokenString || tokens[0].value != "hello world" {
		t.Fatalf("unexpected string token: %#v", tokens[0])
	}
}

func TestLexSkipsLineComments(t *testing.T) {
	tokens, err := lex("# leading comment\n{ user # trailing\n  { id } }")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []tokenKind{
		tokenBraceOpen, tokenName, tokenBraceOpen, tokenName, tokenBraceClose, tokenBraceClose, tokenEOF,
	}
	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d (%#v)", len(expected), len(tokens), tokens)
	}
	for i, kind := range expected {
		if tokens[i].kind != kind {
			t.Fatalf("token %d: expected %s, got %s", i, kind, tokens[i].kind)
		}
	}
}

func TestLexStripsBOM(t *testing.T) {
	tokens, err := lex("\uFEFF{ user { id } }")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tokens[0].kind != tokenBraceOpen {
		t.Fatalf("expected BOM to be stripped, got %s", tokens[0].kind)
	}
}

func TestNormalizeSourceLineTerminators(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
	}{
		{"crlf", "a\r\nb", "a\nb"},
		{"lone cr", "a\rb", "a\nb"},
		{"mixed", "a\r\nb\rc\nd", "a\nb\nc\nd"},
		{"trailing cr", "a\r", "a\n"},
		{"bom and crlf", "\uFEFFa\r\nb", "a\nb"},
		{"no cr fast path", "a\nb", "a\nb"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := normalizeSource(tc.in); got != tc.want {
				t.Fatalf("normalizeSource(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestLocationTrackingAfterCRLFNormalization(t *testing.T) {
	// A CRLF-delimited document must report the same line/column as its
	// LF-delimited equivalent, since both normalize to LF before lexing.
	crlf := "query {\r\n  bad!\r\n}"
	lf := "query {\n  bad!\n}"
	_, errCRLF := parseDocument(crlf, nil)
	_, errLF := parseDocument(lf, nil)
	if errCRLF == nil || errLF == nil {
		t.Fatalf("expected parse errors for both forms")
	}
	locCRLF := errCRLF.(parseError).locations
	locLF := errLF.(parseError).locations
	if len(locCRLF) == 0 || len(locLF) == 0 {
		t.Fatalf("expected locations on both errors")
	}
	if locCRLF[0] != locLF[0] {
		t.Fatalf("CRLF location %+v != LF location %+v", locCRLF[0], locLF[0])
	}
	if locCRLF[0].Line != 2 {
		t.Fatalf("expected error on line 2, got %d", locCRLF[0].Line)
	}
}

func TestLexAdditionalPunctuators(t *testing.T) {
	tokens, err := lex(`... = @ & |`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := []tokenKind{tokenSpread, tokenEquals, tokenAt, tokenAmp, tokenPipe, tokenEOF}
	if len(tokens) != len(expected) {
		t.Fatalf("expected %d tokens, got %d (%#v)", len(expected), len(tokens), tokens)
	}
	for i, kind := range expected {
		if tokens[i].kind != kind {
			t.Fatalf("token %d: expected %s, got %s", i, kind, tokens[i].kind)
		}
	}
}

func TestLexStringEscapes(t *testing.T) {
	tests := []struct {
		name   string
		source string
		want   string
	}{
		{name: "simple escapes", source: `"a\"b\\c\/d\b\f\n\r\te"`, want: "a\"b\\c/d\b\f\n\r\te"},
		{name: "fixed unicode escape", source: `"\u00e9"`, want: "é"},
		{name: "variable-width unicode escape", source: `"\u{1F600}"`, want: "😀"},
		{name: "surrogate pair", source: `"\uD83D\uDE00"`, want: "😀"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tokens, err := lex(tt.source)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if tokens[0].kind != tokenString {
				t.Fatalf("expected string token, got %s", tokens[0].kind)
			}
			if tokens[0].value != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, tokens[0].value)
			}
		})
	}
}

func TestLexStringFastPathDoesNotCopy(t *testing.T) {
	source := `"hello world"`
	tokens, err := lex(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tokens[0].value != "hello world" {
		t.Fatalf("unexpected string value: %q", tokens[0].value)
	}
	// The escape-free fast path returns a substring of the source, which means
	// the underlying byte data is shared. Verify with unsafe-free string header
	// inspection using strings.Index on the source as a sanity check that we
	// did not allocate a separate buffer.
	if !strings.Contains(source, tokens[0].value) {
		t.Fatalf("expected token value to be a substring view of the source")
	}
}

func TestLexUnicodeInString(t *testing.T) {
	tokens, err := lex(`"héllo, 世界"`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tokens[0].value != "héllo, 世界" {
		t.Fatalf("unexpected string token: %q", tokens[0].value)
	}
}

func TestLexBlockString(t *testing.T) {
	source := "\"\"\"\n    Hello\n      indented\n    World\n    \"\"\""
	tokens, err := lex(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tokens[0].kind != tokenString {
		t.Fatalf("expected string token, got %s", tokens[0].kind)
	}
	want := "Hello\n  indented\nWorld"
	if tokens[0].value != want {
		t.Fatalf("expected %q, got %q", want, tokens[0].value)
	}
}

func TestLexBlockStringEscapedTripleQuote(t *testing.T) {
	source := `"""contains \""" inside"""`
	tokens, err := lex(source)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := `contains """ inside`
	if tokens[0].value != want {
		t.Fatalf("expected %q, got %q", want, tokens[0].value)
	}
}

func TestLexNumberGrammar(t *testing.T) {
	tests := []struct {
		source string
		kinds  []tokenKind
		values []string
	}{
		{source: `0`, kinds: []tokenKind{tokenNumber, tokenEOF}, values: []string{"0"}},
		{source: `-0`, kinds: []tokenKind{tokenNumber, tokenEOF}, values: []string{"-0"}},
		{source: `123`, kinds: []tokenKind{tokenNumber, tokenEOF}, values: []string{"123"}},
		{source: `-1.25`, kinds: []tokenKind{tokenNumber, tokenEOF}, values: []string{"-1.25"}},
		{source: `1.5e10`, kinds: []tokenKind{tokenNumber, tokenEOF}, values: []string{"1.5e10"}},
		{source: `2E-3`, kinds: []tokenKind{tokenNumber, tokenEOF}, values: []string{"2E-3"}},
		{source: `1e+5`, kinds: []tokenKind{tokenNumber, tokenEOF}, values: []string{"1e+5"}},
	}
	for _, tt := range tests {
		t.Run(tt.source, func(t *testing.T) {
			tokens, err := lex(tt.source)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(tokens) != len(tt.kinds) {
				t.Fatalf("expected %d tokens, got %d", len(tt.kinds), len(tokens))
			}
			for i, kind := range tt.kinds {
				if tokens[i].kind != kind {
					t.Fatalf("token %d: expected %s, got %s", i, kind, tokens[i].kind)
				}
			}
			for i, value := range tt.values {
				if tokens[i].value != value {
					t.Fatalf("token %d: expected value %q, got %q", i, value, tokens[i].value)
				}
			}
		})
	}
}

func TestParseDocumentTolleratesDirectives(t *testing.T) {
	doc, err := parseDocument(
		`query GetUser @cached(ttl: 30) { user(id: "u_1") @include(if: true) { id @lowercase } }`,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(doc.Selections) != 1 {
		t.Fatalf("expected 1 top-level selection, got %d", len(doc.Selections))
	}
	user := doc.Selections[0]
	if user.Name != "user" {
		t.Fatalf("expected user, got %q", user.Name)
	}
	if user.Arguments["id"] != "u_1" {
		t.Fatalf("expected user.id u_1, got %#v", user.Arguments["id"])
	}
	if len(user.Selections) != 1 || user.Selections[0].Name != "id" {
		t.Fatalf("expected single id child, got %#v", user.Selections)
	}
}

func TestParseDocumentNumberValueCoercion(t *testing.T) {
	doc, err := parseDocument(`{ search(limit: 10, score: 1.5e2, ratio: 2E-1) { id } }`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	args := doc.Selections[0].Arguments
	if args["limit"] != 10 {
		t.Fatalf("expected limit int 10, got %#v", args["limit"])
	}
	if args["score"] != 150.0 {
		t.Fatalf("expected score 150.0, got %#v", args["score"])
	}
	if args["ratio"] != 0.2 {
		t.Fatalf("expected ratio 0.2, got %#v", args["ratio"])
	}
}

var benchmarkQuery = `query GetUser($id: ID!) {
	user(id: $id) {
		id
		name
		email
		account {
			balance
			owner {
				id
				name
			}
		}
		posts(first: 10, filter: {tags: ["go", "graphql"], published: true}) {
			id
			title
			body
		}
	}
}`

func BenchmarkParseDocument(b *testing.B) {
	vars := map[string]any{"id": "user_42"}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := parseDocument(benchmarkQuery, vars); err != nil {
			b.Fatal(err)
		}
	}
}

func TestParseFragmentDefinitionFailureDiagnostics(t *testing.T) {
	cases := []struct {
		doc       string
		substring string
	}{
		{`fragment`, `expected fragment name`},
		{`fragment Foo off Query { x } query Q { __typename }`, `expected "on" in fragment definition`},
		{`fragment Foo on 123 { x } query Q { __typename }`, `expected type condition name`},
		{`fragment @bad on Query { x } query Q { __typename }`, `expected fragment name`},
	}
	for _, tc := range cases {
		_, err := parseDocumentBundle(tc.doc, nil, 0)
		if err == nil || !strings.Contains(err.Error(), tc.substring) {
			t.Fatalf("doc %q: err=%v want substring %q", tc.doc, err, tc.substring)
		}
	}
	if _, err := parseDocumentBundle(`fragments only here`, nil, 0); err == nil {
		t.Fatal("expected top-level fragment keyword error")
	}
}

func TestLocationForOffsetCoversMixedLineEndings(t *testing.T) {
	// Covers \r branch without advancing past an immediately following \n.
	src := "\r#\r#\r"
	if loc := locationForOffset(src, 5); loc.Line != 4 || loc.Column != 1 {
		t.Fatalf("want line 4 col 1, got %#v", loc)
	}
}

func BenchmarkLex(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		if _, err := lex(benchmarkQuery); err != nil {
			b.Fatal(err)
		}
	}
}

func TestParseDocumentEmptySelectionAllocations(t *testing.T) {
	doc, err := parseDocument(`{ user { id } }`, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	user := doc.Selections[0]
	// No arguments => map should be left nil so we don't pay for an empty hmap.
	if user.Arguments != nil {
		t.Fatalf("expected nil Arguments for arg-less field, got %#v", user.Arguments)
	}
}

func TestParseDocumentNamedRejectsDuplicateFragments(t *testing.T) {
	q := `
		fragment dup on Query { __typename }
		fragment dup on Query { __typename }
		query Q { __typename }
	`
	_, err := parseDocumentNamed(q, nil, "Q", 0)
	if err == nil {
		t.Fatal("expected error for duplicate fragment name")
	}
	if !strings.Contains(err.Error(), `There can be only one fragment named "dup"`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDocumentNamedEnforcesMaxSelectionDepth(t *testing.T) {
	q := `{ __schema { queryType { name } } }`
	_, err := parseDocumentNamed(q, nil, "", 2)
	if err == nil {
		t.Fatal("expected depth error")
	}
	if !strings.Contains(err.Error(), "selection depth exceeds limit") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestParseDocumentParsesAliasAndDirectives(t *testing.T) {
	doc, err := parseDocument(`query Q($s: Boolean!) { u: user(id: "1") @skip(if: $s) { n: name } }`, map[string]any{"s": false})
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(doc.Selections) != 1 {
		t.Fatalf("expected one root selection, got %d", len(doc.Selections))
	}
	sel := doc.Selections[0]
	if sel.Alias != "u" || sel.Name != "user" {
		t.Fatalf("unexpected alias/name: %#v", sel)
	}
	if len(sel.Directives) != 1 || sel.Directives[0].Name != "skip" {
		t.Fatalf("unexpected directives: %#v", sel.Directives)
	}
	if sel.Directives[0].Args["if"] != false {
		t.Fatalf("expected skip if=false, got %#v", sel.Directives[0].Args["if"])
	}
	if len(sel.Selections) != 1 {
		t.Fatalf("expected nested selections")
	}
	inner := sel.Selections[0]
	if inner.Alias != "n" || inner.Name != "name" {
		t.Fatalf("unexpected nested alias: %#v", inner)
	}
}

func TestParseDocumentParsesFragmentSpread(t *testing.T) {
	q := `
		fragment userFrag on Query {
			user(id: "1") { id }
		}
		query {
			...userFrag
		}
	`
	doc, err := parseDocumentNamed(q, nil, "", 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(doc.Fragments) != 1 {
		t.Fatalf("expected one fragment def, got %d", len(doc.Fragments))
	}
	fd := doc.Fragments["userFrag"]
	if fd == nil || fd.TypeCondition != "Query" {
		t.Fatalf("unexpected fragment: %#v", fd)
	}
	if len(doc.Selections) != 1 || doc.Selections[0].FragmentSpread != "userFrag" {
		t.Fatalf("unexpected operation selections: %#v", doc.Selections)
	}
}

// --- Issue #5: Variable definition default values ---

func TestParseDocumentVariableDefaultScalar(t *testing.T) {
	// Variable not supplied by client; default from definition must be used.
	doc, err := parseDocument(
		`query List($limit: Int = 10) { items(limit: $limit) { id } }`,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Selections[0].Arguments["limit"] != 10 {
		t.Fatalf("expected limit 10 from default, got %#v", doc.Selections[0].Arguments["limit"])
	}
}

func TestParseDocumentVariableDefaultOverriddenByClient(t *testing.T) {
	doc, err := parseDocument(
		`query List($limit: Int = 10) { items(limit: $limit) { id } }`,
		map[string]any{"limit": 20},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Selections[0].Arguments["limit"] != 20 {
		t.Fatalf("expected limit 20 from variables, got %#v", doc.Selections[0].Arguments["limit"])
	}
}

func TestParseDocumentVariableDefaultString(t *testing.T) {
	doc, err := parseDocument(
		`query Q($name: String = "World") { greet(name: $name) { message } }`,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Selections[0].Arguments["name"] != "World" {
		t.Fatalf("expected name World from default, got %#v", doc.Selections[0].Arguments["name"])
	}
}

func TestParseDocumentVariableDefaultBool(t *testing.T) {
	doc, err := parseDocument(
		`query Q($verbose: Boolean = true) { items(verbose: $verbose) { id } }`,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Selections[0].Arguments["verbose"] != true {
		t.Fatalf("expected verbose true from default, got %#v", doc.Selections[0].Arguments["verbose"])
	}
}

func TestParseDocumentVariableDefaultNull(t *testing.T) {
	doc, err := parseDocument(
		`query Q($cursor: String = null) { items(cursor: $cursor) { id } }`,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := doc.Selections[0].Arguments["cursor"]; !ok {
		t.Fatalf("expected cursor key to be present in arguments")
	}
	if doc.Selections[0].Arguments["cursor"] != nil {
		t.Fatalf("expected cursor nil from default, got %#v", doc.Selections[0].Arguments["cursor"])
	}
}

func TestParseDocumentVariableDefaultListType(t *testing.T) {
	// Default on a list-typed variable
	doc, err := parseDocument(
		`query Q($ids: [ID!]! = ["a", "b"]) { users(ids: $ids) { id } }`,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	ids, ok := doc.Selections[0].Arguments["ids"].([]any)
	if !ok || len(ids) != 2 {
		t.Fatalf("expected ids list of length 2, got %#v", doc.Selections[0].Arguments["ids"])
	}
}

func TestParseDocumentVariableDefaultObjectType(t *testing.T) {
	doc, err := parseDocument(
		`query Q($filter: FilterInput = {active: true}) { items(filter: $filter) { id } }`,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	filter, ok := doc.Selections[0].Arguments["filter"].(map[string]any)
	if !ok {
		t.Fatalf("expected filter map, got %#v", doc.Selections[0].Arguments["filter"])
	}
	if filter["active"] != true {
		t.Fatalf("expected filter.active true, got %#v", filter["active"])
	}
}

func TestParseDocumentMultipleVariableDefaults(t *testing.T) {
	doc, err := parseDocument(
		`query Q($limit: Int = 5, $offset: Int = 0, $q: String) {
			search(limit: $limit, offset: $offset, q: $q) { id }
		}`,
		map[string]any{"q": "hello"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	args := doc.Selections[0].Arguments
	if args["limit"] != 5 {
		t.Fatalf("expected limit 5, got %#v", args["limit"])
	}
	if args["offset"] != 0 {
		t.Fatalf("expected offset 0, got %#v", args["offset"])
	}
	if args["q"] != "hello" {
		t.Fatalf("expected q hello, got %#v", args["q"])
	}
}

func TestParseDocumentVariableDefaultWithDirectiveOnVar(t *testing.T) {
	// Directives on variable definitions should be parsed without error.
	doc, err := parseDocument(
		`query Q($id: ID! @deprecated) { user(id: $id) { id } }`,
		map[string]any{"id": "u1"},
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if doc.Selections[0].Arguments["id"] != "u1" {
		t.Fatalf("expected id u1, got %#v", doc.Selections[0].Arguments["id"])
	}
}

// --- Issue #5: Directives on fragment definitions ---

func TestParseDocumentDirectivesOnFragmentDefinition(t *testing.T) {
	q := `
		fragment UserFields on User @deprecated(reason: "Use new format") {
			id
			name
		}
		query { ...UserFields }
	`
	_, err := parseDocumentNamed(q, nil, "", 0)
	if err != nil {
		t.Fatalf("unexpected error parsing fragment with directives: %v", err)
	}
}

func TestParseDocumentDirectivesOnInlineFragment(t *testing.T) {
	doc, err := parseDocument(
		`{ user { ... on User @skip(if: false) { id } } }`,
		nil,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_ = doc
}

func TestParserLiteralAndFragmentVariants(t *testing.T) {
	query := `
		query Q(
			$int: Int = 1
			$float: Float = 1.5
			$bool: Boolean = true
			$string: String = "Ada"
			$list: [Int] = [1, 2, 3]
			$obj: Input = {term: "x", nested: {ok: true}}
		) {
			alias: user(id: $string, input: {term: "inline", nums: [1, 2]}) @include(if: $bool) {
				...Frag
				... on User @skip(if: false) {
					name
				}
			}
		}
		fragment Frag on User { id }
	`
	bundle, err := parseDocumentBundle(query, map[string]any{"bool": true, "string": "1"}, 0)
	if err != nil {
		t.Fatalf("parse bundle: %v", err)
	}
	doc, err := selectOperation(bundle, "Q")
	if err != nil {
		t.Fatalf("select op: %v", err)
	}
	if doc.Name != "Q" || len(doc.Variables) != 6 || len(doc.Selections) != 1 {
		t.Fatalf("doc = %#v", doc)
	}

	for _, bad := range []string{
		`query Q($bad: [) { x }`,
		`query Q { field(arg: {a:}) }`,
		`query Q { field(arg: [1,) }`,
		`fragment on User { id }`,
		`query Q { ... on { id } }`,
	} {
		if _, err := parseDocumentBundle(bad, nil, 0); err == nil {
			t.Fatalf("expected parse error for %q", bad)
		}
	}
}

func TestParserTokenAndLocationBranches(t *testing.T) {
	for _, kind := range []tokenKind{
		tokenEOF, tokenName, tokenString, tokenNumber, tokenBraceOpen, tokenBraceClose,
		tokenParenOpen, tokenParenClose, tokenColon, tokenDollar, tokenBracketOpen,
		tokenBracketClose, tokenBang, tokenSpread, tokenEquals, tokenAt, tokenAmp, tokenPipe,
	} {
		if kind.String() == "" {
			t.Fatalf("empty token string for %d", kind)
		}
	}
	if tokenKind(255).String() != "<unknown>" {
		t.Fatal("expected unknown token string")
	}
	source := "one\ntwo\nthree"
	if loc := locationForOffset(source, 0); loc.Line != 1 || loc.Column != 1 {
		t.Fatalf("start location = %#v", loc)
	}
	if loc := locationForOffset(source, len(source)+10); loc.Line != 3 || loc.Column != 6 {
		t.Fatalf("end location = %#v", loc)
	}
	if lines := splitBlockStringLines("a\r\nb\rc"); !reflect.DeepEqual(lines, []string{"a", "b", "c"}) {
		t.Fatalf("split lines = %#v", lines)
	}
	for _, raw := range []string{
		`query Q { field(arg: """line
			value
		""") }`,
		`query Q { field(arg: "\u0041") }`,
	} {
		if _, err := parseDocumentBundle(raw, nil, 0); err != nil {
			t.Fatalf("parse string branch: %v", err)
		}
	}
	if _, err := parseDocumentBundle(`query Q { field(arg: "\u00ZZ") }`, nil, 0); err == nil {
		t.Fatal("expected unicode escape parse error")
	}
}

func TestParseConstValueFloatIntegerAndInvalidLiteralBranches(t *testing.T) {
	// Covers parseConstValue number branches: floats, ints outside strconv.Atoi range, enums, lists/objects.
	doc, err := parseDocument(`
		query Q(
		 $f: Float = 12.25e3
		 $li: Mixed = 3000000000
		 $truth: Mixed = ENUMVAL
		 $nested: Mixed = [{ x: ENUMVAL }, { flag: FALSEVAL }]
		) {
			sample(f: $f, li: $li, truth: $truth, nested: $nested)
		}`, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	args := doc.Selections[0].Arguments
	if got := args["f"].(float64); got != 12250 {
		t.Fatalf("scientific float default = %#v", got)
	}
	if got, ok := args["li"].(int64); ok {
		if got != 3000000000 {
			t.Fatalf("large int literal int64 = %d", got)
		}
	} else if got, ok := args["li"].(int); ok {
		if got != 3000000000 {
			t.Fatalf("large int literal int = %d", got)
		}
	} else {
		t.Fatalf("large int literal type = %T %#v", args["li"], args["li"])
	}
	if got := args["truth"].(string); got != "ENUMVAL" {
		t.Fatalf("name token default = %#v", got)
	}
	nested, ok := args["nested"].([]any)
	if !ok || len(nested) != 2 {
		t.Fatalf("nested const list = %#v", args["nested"])
	}
	m0 := nested[0].(map[string]any)
	if m0["x"].(string) != "ENUMVAL" {
		t.Fatalf("nested enum-ish = %#v", m0["x"])
	}

	for _, bad := range []string{
		// Variable references are not allowed in constant (default-value) positions.
		`query Q($x: Mixed = $y) { o }`,
		// Integer literals must fit in int64 for parseConstValue.
		`query Q($n: Mixed = 9999999999999999999999999999999) { o }`,
	} {
		if _, err := parseDocument(bad, nil); err == nil {
			t.Fatalf("expected const value error for %q", bad)
		}
	}
}

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

func TestParseErrorsCarryLocations(t *testing.T) {
	cases := []struct {
		name      string
		query     string
		wantLine  int
		substring string
	}{
		{"unclosed selection", "{\n  user {\n    id\n", 4, "end of query"},
		{"bad value token", "{ user(id: @) { id } }", 1, "value token"},
		{"missing fragment name", "{ ... }", 1, "fragment name"},
		{"leading zero", "{ f(n: 01) }", 1, "leading zeros"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseDocument(tc.query, nil)
			if err == nil {
				t.Fatalf("expected parse error for %q", tc.query)
			}
			pe, ok := err.(parseError)
			if !ok {
				t.Fatalf("expected parseError, got %T: %v", err, err)
			}
			if !strings.Contains(pe.Error(), tc.substring) {
				t.Fatalf("error %q does not contain %q", pe.Error(), tc.substring)
			}
			locs := pe.GraphQLLocations()
			if len(locs) == 0 {
				t.Fatalf("expected source location on parse error %q", pe.Error())
			}
			if locs[0].Line != tc.wantLine {
				t.Fatalf("error line = %d, want %d (%+v)", locs[0].Line, tc.wantLine, locs[0])
			}
			if locs[0].Column < 1 {
				t.Fatalf("expected 1-based column, got %d", locs[0].Column)
			}
		})
	}
}

func TestParseDocumentFragmentSpreadAndUnicodeEscapeString(t *testing.T) {
	q := `
fragment F on Query {
	otherUser: user(id: "99") {
		id
	}
}
{
	u: user(id: "\u0041BB") {
		id name
	}
	...F
}`
	doc, err := parseDocument(q, nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	fr, ok := doc.Fragments["F"]
	if !ok || fr.TypeCondition != "Query" {
		t.Fatalf("fragment metadata = %#v (ok=%v)", fr, ok)
	}
	if len(doc.Selections) != 2 {
		t.Fatalf("expected two selections (field + fragment spread), got %d (%#v)",
			len(doc.Selections), doc.Selections)
	}
}

func TestParseVariableDefinitionDefaultsCoverConstBranches(t *testing.T) {
	cases := []struct {
		name         string
		query        string
		errSubstring string
		fail         bool // any error acceptable
	}{
		{name: "scientific-float-default", query: `query Q($f: Float = 12.25e3) { __typename }`},
		{name: "large-integer-token", query: `query Q($n: Mixed = 3000000000) { __typename }`},
		{
			name:  "const-object-with-list-and-enum-ish-name",
			query: `query Q($cfg: Mixed = { tags: [{ flag: false }], value: ENUMVAL }) { __typename }`,
		},
		{
			name:         "duplicate-const-fields",
			query:        `query Q($bad: Mixed = { a: 1 a: 2 }) { __typename }`,
			errSubstring: `only one input field`,
		},
		{name: "unterminated-const-list", query: `query Q($z: Mixed = [[1,) { __typename }`, fail: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseDocument(tc.query, nil)
			switch {
			case tc.errSubstring != "":
				if err == nil || !strings.Contains(err.Error(), tc.errSubstring) {
					t.Fatalf("expected error containing %q, got %v", tc.errSubstring, err)
				}
			case tc.fail:
				if err == nil {
					t.Fatal("expected parse failure")
				}
			default:
				if err != nil {
					t.Fatalf("unexpected parse error: %v", err)
				}
			}
		})
	}
}

func TestParseFragmentDefinitionDirectivesAndBlockStringDefault(t *testing.T) {
	fragSrc := `
		fragment Pieces on Query @include(if:true) @skip(if:false) {
		  user(id:"1"){ id }
		}
		query { v: __typename ...Pieces }
	`
	if _, err := parseDocument(fragSrc, nil); err != nil {
		t.Fatalf("fragment dirs: %v", err)
	}
	blockDefault := `
		query Q($s: String = """multi
line""" ) {
		  __typename
		}
	`
	if _, err := parseDocument(blockDefault, nil); err != nil {
		t.Fatalf("block string variable default: %v", err)
	}
}
