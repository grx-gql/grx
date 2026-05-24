package exec

import (
	"context"
	"strings"
	"testing"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/schema"
)

func testSchemaForValidation(t *testing.T) *schema.Schema {
	t.Helper()
	s, err := schema.Build(schema.Config{Query: testQuery{}, Mutation: testMutation{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}
	return s
}

func TestValidateFieldsOnCorrectType(t *testing.T) {
	s := testSchemaForValidation(t)
	bundle, err := parseDocumentBundle(`{ missing }`, nil, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	doc, err := selectOperation(bundle, "")
	if err != nil {
		t.Fatalf("select: %v", err)
	}

	errs := ValidateDocument(s, bundle, doc)
	if len(errs) != 1 {
		t.Fatalf("expected one error, got %#v", errs)
	}
	want := `Cannot query field "missing" on type "Query".`
	if errs[0].Error() != want {
		t.Fatalf("message = %q, want %q", errs[0].Error(), want)
	}
}

func TestValidateUnknownFragment(t *testing.T) {
	s := testSchemaForValidation(t)
	bundle, err := parseDocumentBundle(`{ ...Missing }`, nil, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	doc, err := selectOperation(bundle, "")
	if err != nil {
		t.Fatalf("select: %v", err)
	}

	errs := ValidateDocument(s, bundle, doc)
	if len(errs) != 1 || errs[0].Error() != `Unknown fragment "Missing".` {
		t.Fatalf("unexpected errors: %#v", errs)
	}
}

func TestValidateUniqueOperationNames(t *testing.T) {
	s := testSchemaForValidation(t)
	bundle, err := parseDocumentBundle(`query A { __typename } query A { user(id: "1") { id } }`, nil, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	doc, err := selectOperation(bundle, "A")
	if err != nil {
		t.Fatalf("select: %v", err)
	}

	errs := ValidateDocument(s, bundle, doc)
	if len(errs) == 0 {
		t.Fatal("expected duplicate operation name error")
	}
	if !strings.Contains(errs[0].Error(), `There can be only one operation named "A".`) {
		t.Fatalf("unexpected error: %q", errs[0].Error())
	}
}

func TestValidateScalarLeafsRequiresSubselection(t *testing.T) {
	s := testSchemaForValidation(t)
	bundle, err := parseDocumentBundle(`{ user(id: "1") }`, nil, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	doc, err := selectOperation(bundle, "")
	if err != nil {
		t.Fatalf("select: %v", err)
	}

	errs := ValidateDocument(s, bundle, doc)
	if len(errs) != 1 {
		t.Fatalf("expected one error, got %#v", errs)
	}
	if !strings.Contains(errs[0].Error(), `must have a selection of subfields`) {
		t.Fatalf("unexpected error: %q", errs[0].Error())
	}
}

func TestValidateUnusedFragment(t *testing.T) {
	s := testSchemaForValidation(t)
	q := `
		fragment F on Query { user(id: "1") { id } }
		{ __typename }
	`
	bundle, err := parseDocumentBundle(q, nil, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	doc, err := selectOperation(bundle, "")
	if err != nil {
		t.Fatalf("select: %v", err)
	}

	errs := ValidateDocument(s, bundle, doc)
	found := false
	for _, err := range errs {
		if err.Error() == `Fragment "F" is never used.` {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected unused fragment error, got %#v", errs)
	}
}

func TestValidateUnknownDirective(t *testing.T) {
	s := testSchemaForValidation(t)
	bundle, err := parseDocumentBundle(`{ user(id: "1") @nope { id } }`, nil, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	doc, err := selectOperation(bundle, "")
	if err != nil {
		t.Fatalf("select: %v", err)
	}

	errs := ValidateDocument(s, bundle, doc)
	if len(errs) != 1 || errs[0].Error() != `Unknown directive "@nope".` {
		t.Fatalf("unexpected errors: %#v", errs)
	}
}

func TestValidateUndefinedVariable(t *testing.T) {
	s := testSchemaForValidation(t)
	bundle, err := parseDocumentBundle(`{ user(id: $id) { id } }`, nil, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	doc, err := selectOperation(bundle, "")
	if err != nil {
		t.Fatalf("select: %v", err)
	}

	errs := ValidateDocument(s, bundle, doc)
	if len(errs) == 0 || !strings.Contains(errs[0].Error(), `Variable "$id" is not defined`) {
		t.Fatalf("expected undefined variable error, got %#v", errs)
	}
}

func TestValidateUnusedVariable(t *testing.T) {
	s := testSchemaForValidation(t)
	bundle, err := parseDocumentBundle(`query GetUser($id: String!) { __typename }`, map[string]any{"id": "1"}, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	doc, err := selectOperation(bundle, "GetUser")
	if err != nil {
		t.Fatalf("select: %v", err)
	}

	errs := ValidateDocument(s, bundle, doc)
	if len(errs) == 0 || !strings.Contains(errs[0].Error(), `Variable "$id" is never used`) {
		t.Fatalf("expected unused variable error, got %#v", errs)
	}
}

func TestValidateVariableInAllowedPosition(t *testing.T) {
	s := testSchemaForValidation(t)
	bundle, err := parseDocumentBundle(`query GetUser($id: Int!) { user(id: $id) { id } }`, map[string]any{"id": 1}, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	doc, err := selectOperation(bundle, "GetUser")
	if err != nil {
		t.Fatalf("select: %v", err)
	}

	errs := ValidateDocument(s, bundle, doc)
	if len(errs) == 0 || !strings.Contains(errs[0].Error(), `used in position expecting type "String!"`) {
		t.Fatalf("expected variable position error, got %#v", errs)
	}
}

func TestValidateNonNullVariableInNullablePosition(t *testing.T) {
	s := testSchemaForValidation(t)
	bundle, err := parseDocumentBundle(`query GetUser($id: String!) { user(id: $id) { id } }`, map[string]any{"id": "1"}, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	doc, err := selectOperation(bundle, "GetUser")
	if err != nil {
		t.Fatalf("select: %v", err)
	}

	errs := ValidateDocument(s, bundle, doc)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %#v", errs)
	}
}

func TestValidateFieldMergeConflict(t *testing.T) {
	s := testSchemaForValidation(t)
	bundle, err := parseDocumentBundle(`{ first: user(id: "1") { id } first: user(id: "2") { id } }`, nil, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	doc, err := selectOperation(bundle, "")
	if err != nil {
		t.Fatalf("select: %v", err)
	}

	errs := ValidateDocument(s, bundle, doc)
	if len(errs) == 0 || !strings.Contains(errs[0].Error(), `Fields "first" conflict`) {
		t.Fatalf("expected field merge conflict, got %#v", errs)
	}
}

func TestValidateDirectiveArguments(t *testing.T) {
	s := testSchemaForValidation(t)
	bundle, err := parseDocumentBundle(`{ user(id: "1") @skip { id } }`, nil, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	doc, err := selectOperation(bundle, "")
	if err != nil {
		t.Fatalf("select: %v", err)
	}

	errs := ValidateDocument(s, bundle, doc)
	if len(errs) == 0 || !strings.Contains(errs[0].Error(), `Directive "@skip" argument "if"`) {
		t.Fatalf("expected directive argument error, got %#v", errs)
	}
}

func TestExecutorReturnsValidationErrorsWithCode(t *testing.T) {
	s := testSchemaForValidation(t)
	executor := New(s, nil)
	response := executor.Execute(context.Background(), core.Request{Query: `{ missing }`})
	if len(response.Errors) != 1 {
		t.Fatalf("expected one error, got %#v", response.Errors)
	}
	if response.Errors[0].Message != `Cannot query field "missing" on type "Query".` {
		t.Fatalf("message = %q", response.Errors[0].Message)
	}
	code, _ := response.Errors[0].Extensions["code"].(string)
	if code != core.ErrorCodeValidationFailed {
		t.Fatalf("code = %q", code)
	}
	if response.Data != nil || response.DataNull {
		t.Fatalf("expected no execution data envelope, got Data=%#v DataNull=%v", response.Data, response.DataNull)
	}
}

func TestParseDocumentNamedUsesGraphQLErrorMessages(t *testing.T) {
	query := `query A { __typename } query B { __typename }`
	_, err := parseDocumentNamed(query, nil, "", 0)
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "Must provide operation name if query contains multiple operations.") {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = parseDocumentNamed(query, nil, "Missing", 0)
	if err == nil || !strings.Contains(err.Error(), `Unknown operation named "Missing".`) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidationHelperBranches(t *testing.T) {
	loc := core.Location{Line: 1, Column: 2}
	stringType := &schema.Scalar{TypeName: "String"}
	intType := &schema.Scalar{TypeName: "Int"}
	floatType := &schema.Scalar{TypeName: "Float"}
	boolType := &schema.Scalar{TypeName: "Boolean"}
	idType := &schema.Scalar{TypeName: "ID"}

	for _, tc := range []struct {
		typ   schema.Type
		value any
	}{
		{stringType, 1},
		{intType, "x"},
		{floatType, "x"},
		{boolType, "x"},
		{idType, struct{}{}},
		{&schema.NonNull{OfType: stringType}, nil},
		{&schema.List{OfType: intType}, []any{1, "x"}},
		{&schema.InputObject{TypeName: "Input", Fields: map[string]*schema.Field{"id": {Name: "id", Type: &schema.NonNull{OfType: idType}}}}, map[string]any{}},
		{&schema.InputObject{TypeName: "Input", Fields: map[string]*schema.Field{}}, "x"},
		{&schema.Enum{TypeName: "Role", Values: []schema.EnumValue{{Name: "ADMIN"}}}, "USER"},
		{&schema.Enum{TypeName: "Role", Values: []schema.EnumValue{{Name: "ADMIN"}}}, 1},
	} {
		if errs := validateInputValue(tc.typ, tc.value, loc, "value"); len(errs) == 0 {
			t.Fatalf("expected validation error for %T %#v", tc.typ, tc.value)
		}
	}
	if errs := validateInputValue(&schema.List{OfType: intType}, int64(1), loc, "value"); len(errs) != 0 {
		t.Fatalf("single list item should validate: %#v", errs)
	}
	oneOf := &schema.InputObject{TypeName: "One", IsOneOf: true, Fields: map[string]*schema.Field{
		"a": {Name: "a", Type: stringType},
		"b": {Name: "b", Type: stringType},
	}}
	if errs := validateInputObjectValue(oneOf, map[string]any{"a": "x", "b": "y"}, loc); len(errs) == 0 {
		t.Fatal("expected oneOf validation error")
	}

	s := &schema.Schema{
		Query:        &schema.Object{TypeName: "Query"},
		Mutation:     &schema.Object{TypeName: "Mutation"},
		Subscription: &schema.Object{TypeName: "Subscription"},
		Types: map[string]schema.Type{
			"String": stringType,
			"Query":  &schema.Object{TypeName: "Query"},
			"Input":  &schema.InputObject{TypeName: "Input"},
		},
	}
	for _, kind := range []operationKind{operationQuery, operationMutation, operationSubscription} {
		if root, err := rootObjectForKind(s, kind); err != nil || root == nil {
			t.Fatalf("root %s = %#v %v", kind, root, err)
		}
	}
	if _, err := rootObjectForKind(&schema.Schema{}, operationMutation); err == nil {
		t.Fatal("expected missing mutation root")
	}
	if !isInputType(&schema.InputObject{TypeName: "Input"}) || isInputType(&schema.Object{TypeName: "Obj"}) {
		t.Fatal("input type classification mismatch")
	}
	if !isCompositeType(&schema.Union{TypeName: "Union"}) || isCompositeType(stringType) {
		t.Fatal("composite type classification mismatch")
	}
	if !isLeafType(&schema.Enum{TypeName: "Enum"}) || isLeafType(&schema.Object{TypeName: "Obj"}) {
		t.Fatal("leaf type classification mismatch")
	}
	if typeString(&schema.NonNull{OfType: stringType}) != "String!" {
		t.Fatal("type string mismatch")
	}
	if objectTypeForSelection(&schema.Object{TypeName: "Obj"}) == nil {
		t.Fatal("expected object type for field selection")
	}
	if objectTypeForFieldType(s, &schema.List{OfType: &schema.NonNull{OfType: &schema.Object{TypeName: "Obj"}}}) == nil {
		t.Fatal("expected object type through wrappers")
	}
}

func TestValidateDocumentErrorBranches(t *testing.T) {
	s, err := ParseSDL(`
		input Input { id: ID! }
		type User { id: ID! name: String }
		type Query { user(id: ID!, input: Input): User scalar: String }
		type Subscription { changed: User }
	`)
	if err != nil {
		t.Fatalf("schema: %v", err)
	}
	queries := []string{
		`query A { scalar } query A { scalar }`,
		`{ ...Missing }`,
		`fragment F on Missing { id } query Q { scalar }`,
		`fragment F on User { missing } query Q { user(id: "1") { ...F } }`,
		`query Q { user(id: "1") { ... on Missing { id } } }`,
		`query Q { missing }`,
		`query Q { user }`,
		`query Q { user(id: "1", bad: true) { id } }`,
		`query Q($id: Missing) { user(id: $id) { id } }`,
		`query Q { user(id: "1", input: {bad: true}) { id } }`,
		`query Q { scalar @include }`,
		`query Q { scalar @skip(if: "bad") }`,
		`subscription S { changed { id } extra: changed { id } }`,
	}
	for _, query := range queries {
		bundle, parseErr := parseDocumentBundle(query, nil, 0)
		if parseErr != nil {
			t.Fatalf("parse %q: %v", query, parseErr)
		}
		doc, selectErr := selectOperation(bundle, "")
		if selectErr != nil && query != `query A { scalar } query A { scalar }` {
			t.Fatalf("select %q: %v", query, selectErr)
		}
		if selectErr == nil {
			if errs := ValidateDocument(s, bundle, doc); len(errs) == 0 {
				t.Fatalf("expected validation error for %q", query)
			}
		}
	}
}

type valRole string

const (
	valRoleAdmin valRole = "ADMIN"
	valRoleUser  valRole = "USER"
)

type valFilter struct {
	Term  string `gql:"term,nonNull"`
	Limit int    `gql:"limit"`
}

type valQuery struct{}

func (valQuery) Search(ctx context.Context, args struct {
	Term  string  `gql:"term,nonNull"`
	Count int     `gql:"count"`
	Role  valRole `gql:"role"`
}) (string, error) {
	return args.Term, nil
}

func (valQuery) Find(ctx context.Context, args struct {
	Filter valFilter `gql:"filter,nonNull"`
}) (string, error) {
	return args.Filter.Term, nil
}

func newValExecutor(t *testing.T) *Executor {
	t.Helper()
	s, err := schema.Build(schema.Config{
		Query: valQuery{},
		Enums: []schema.EnumConfig{{
			Type:   valRole(""),
			Name:   "valRole",
			Values: []schema.EnumValueConfig{{Name: "ADMIN", Value: valRoleAdmin}, {Name: "USER", Value: valRoleUser}},
		}},
	})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}
	return New(s, nil)
}

func valErrors(t *testing.T, e *Executor, query string) []core.Error {
	t.Helper()
	return e.Execute(context.Background(), core.Request{Query: query}).Errors
}

func hasErrContaining(errs []core.Error, substr string) bool {
	for _, e := range errs {
		if strings.Contains(e.Message, substr) {
			return true
		}
	}
	return false
}

func TestValidationScalarLiteralTypes(t *testing.T) {
	e := newValExecutor(t)
	cases := []struct {
		name  string
		query string
		want  string
	}{
		{"string for int", `{ search(term: "x", count: "nope") }`, "Int cannot represent"},
		{"bool for string", `{ search(term: true) }`, "String cannot represent"},
		{"null for non-null", `{ search(term: null) }`, "found null"},
		{"bad enum value", `{ search(term: "x", role: MAYBE) }`, "does not exist in"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			errs := valErrors(t, e, tc.query)
			if !hasErrContaining(errs, tc.want) {
				t.Fatalf("expected error containing %q, got %#v", tc.want, errs)
			}
		})
	}
}

func TestValidationScalarLiteralAccepts(t *testing.T) {
	e := newValExecutor(t)
	// Valid literals must not produce validation errors.
	if errs := valErrors(t, e, `{ search(term: "x", count: 3, role: ADMIN) }`); len(errs) != 0 {
		t.Fatalf("unexpected errors for valid literals: %#v", errs)
	}
}

func TestValidationInputObjectRules(t *testing.T) {
	e := newValExecutor(t)

	if errs := valErrors(t, e, `{ find(filter: {limit: 2}) }`); !hasErrContaining(errs, "required type") {
		t.Fatalf("expected missing-required-field error, got %#v", errs)
	}
	if errs := valErrors(t, e, `{ find(filter: {term: "x", bogus: 1}) }`); !hasErrContaining(errs, "is not defined by type") {
		t.Fatalf("expected unknown-input-field error, got %#v", errs)
	}
	if errs := valErrors(t, e, `{ find(filter: {term: 5}) }`); !hasErrContaining(errs, "String cannot represent") {
		t.Fatalf("expected nested field type error, got %#v", errs)
	}
	if errs := valErrors(t, e, `{ find(filter: {term: "ok"}) }`); len(errs) != 0 {
		t.Fatalf("valid input object should pass, got %#v", errs)
	}
}

func TestValidationVariableUniqueness(t *testing.T) {
	e := newValExecutor(t)
	errs := valErrors(t, e, `query ($x: String!, $x: String!) { search(term: $x) }`)
	if !hasErrContaining(errs, `only one variable named "$x"`) {
		t.Fatalf("expected duplicate variable error, got %#v", errs)
	}
}

func TestValidationVariablesMustBeInputTypes(t *testing.T) {
	e := newValExecutor(t)
	// String is a valid input type — the variable definition must not be
	// rejected as a non-input type.
	resp := e.Execute(context.Background(), core.Request{
		Query:     `query ($t: String!) { search(term: $t) }`,
		Variables: map[string]any{"t": "hello"},
	})
	if hasErrContaining(resp.Errors, "non-input type") {
		t.Fatalf("String! variable wrongly flagged as non-input: %#v", resp.Errors)
	}
	if len(resp.Errors) != 0 {
		t.Fatalf("valid input-typed variable should pass, got %#v", resp.Errors)
	}
}

func TestValidationDeferLabelUniqueness(t *testing.T) {
	e := newValExecutor(t)
	query := `{
		a: search(term: "x")
		... on valQuery @defer(label: "dup") { b: search(term: "y") }
		... on valQuery @defer(label: "dup") { c: search(term: "z") }
	}`
	errs := valErrors(t, e, query)
	if !hasErrContaining(errs, `label "dup" must be unique`) {
		t.Fatalf("expected duplicate defer label error, got %#v", errs)
	}
}

// Null in a non-null position is rejected by validation with a located error.

func TestNullValueRejectedInNonNullContext(t *testing.T) {
	e := newValExecutor(t)
	resp := e.Execute(context.Background(), core.Request{Query: `{ search(term: null) }`})
	if len(resp.Errors) == 0 {
		t.Fatal("expected error for null in non-null position")
	}
	if !hasErrContaining(resp.Errors, "found null") {
		t.Fatalf("expected null-in-non-null error, got %#v", resp.Errors)
	}
	if len(resp.Errors[0].Locations) == 0 {
		t.Fatalf("expected a source location on the error: %#v", resp.Errors[0])
	}
}

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

func TestValidateDocumentUnionConditionalFragmentSelections(t *testing.T) {
	s, err := ParseSDL(`
		union Thing = Foo | Bar
		type Foo { name: ID }
		type Bar { title: ID }
		type Query { item: Thing }
	`)
	if err != nil {
		t.Fatalf("ParseSDL: %v", err)
	}
	doc := `{ item { __typename ... on Thing { ... on Foo { name } ... on Bar { title } } } }`
	bundle, err := parseDocumentBundle(doc, nil, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	op, err := selectOperation(bundle, "")
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	errs := ValidateDocument(s, bundle, op)
	if len(errs) != 0 {
		var buf strings.Builder
		for _, validationErr := range errs {
			buf.WriteString("\n ")
			buf.WriteString(validationErr.Error())
		}
		t.Fatalf("unexpected validation errors:%s", buf.String())
	}
}

func TestValidateExecutableDirectiveDuplicatesAndDeferStreamArgs(t *testing.T) {
	s := testSchemaForValidation(t)
	cases := []struct {
		doc string
		msg string
	}{
		{`query { __typename @unknown }`, `Unknown directive`},
		{`query { __typename @skip(if: false) @skip(if: true) }`, `may not be used more than once`},
		{`query { __typename @defer(if: "x") }`, `must be Boolean`},
		{`query { __typename @defer(label: false) }`, `must be String`},
		{`query { __typename @stream(label: false) }`, `must be String`},
		{`query { __typename @stream(initialCount: "nope") }`, `must be Int`},
	}
	for _, tc := range cases {
		bundle, err := parseDocumentBundle(tc.doc, nil, 0)
		if err != nil {
			t.Fatalf("%q bundle: %v", tc.doc, err)
		}
		op, err := selectOperation(bundle, "")
		if err != nil {
			t.Fatalf("%q select: %v", tc.doc, err)
		}
		errs := ValidateDocument(s, bundle, op)
		if len(errs) != 1 {
			t.Fatalf("%q errs = %#v", tc.doc, errs)
		}
		if !strings.Contains(errs[0].Error(), tc.msg) {
			t.Fatalf("%q msg = %q want substring %q", tc.doc, errs[0].Error(), tc.msg)
		}
	}
}

func TestValidateSkipIncludeAllowsVariableLiteralsBoundOrRejectsUnset(t *testing.T) {
	s := testSchemaForValidation(t)
	doc := `query Q($missing: Boolean) { __typename @skip(if: $missing) }`
	bundle, err := parseDocumentBundle(doc, nil, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	op, err := selectOperation(bundle, "")
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	errs := ValidateDocument(s, bundle, op)
	found := false
	for _, validationErr := range errs {
		if strings.Contains(validationErr.Error(), "missing") && strings.Contains(validationErr.Error(), "$missing") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected skip directive unset variable error among %#v", errs)
	}
}

func TestTypeStringNil(t *testing.T) {
	if got := typeString(nil); got != "" {
		t.Fatalf("typeString(nil) = %q, want empty", got)
	}
	if got := typeString(&schema.Scalar{TypeName: "Int"}); got != "Int" {
		t.Fatalf("typeString(Int) = %q", got)
	}
}
