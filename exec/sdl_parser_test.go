package exec

import (
	"strings"
	"testing"

	"github.com/grx-gql/grx/schema"
)

// --- Issue #5 & #6: SDL Parser ---

func TestParseSDLScalarType(t *testing.T) {
	s, err := ParseSDL(`
		scalar DateTime
		type Query { now: DateTime }
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, ok := s.Types["DateTime"].(*schema.Scalar); !ok {
		t.Fatalf("expected DateTime scalar, got %T", s.Types["DateTime"])
	}
}

func TestParseSDLScalarWithSpecifiedBy(t *testing.T) {
	s, err := ParseSDL(`
		scalar URL @specifiedBy(url: "https://url.spec.whatwg.org/")
		type Query { link: URL }
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	sc, ok := s.Types["URL"].(*schema.Scalar)
	if !ok {
		t.Fatalf("expected URL scalar, got %T", s.Types["URL"])
	}
	if sc.SpecifiedByURL != "https://url.spec.whatwg.org/" {
		t.Fatalf("expected specifiedByURL, got %q", sc.SpecifiedByURL)
	}
}

func TestParseSDLObjectType(t *testing.T) {
	s, err := ParseSDL(`
		type User {
			id: ID!
			name: String
			age: Int
		}
		type Query { user: User }
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	userT, ok := s.Types["User"].(*schema.Object)
	if !ok {
		t.Fatalf("expected User object, got %T", s.Types["User"])
	}
	if len(userT.Fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(userT.Fields))
	}
	if _, ok := userT.Fields["id"]; !ok {
		t.Fatal("expected id field")
	}
}

func TestParseSDLObjectWithNonNullAndList(t *testing.T) {
	s, err := ParseSDL(`
		type Post {
			id: ID!
			tags: [String!]!
			comments: [String]
		}
		type Query { post: Post }
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	postT := s.Types["Post"].(*schema.Object)

	idField := postT.Fields["id"]
	if _, ok := idField.Type.(*schema.NonNull); !ok {
		t.Fatalf("expected id to be NonNull, got %T", idField.Type)
	}

	tagsField := postT.Fields["tags"]
	nn, ok := tagsField.Type.(*schema.NonNull)
	if !ok {
		t.Fatalf("expected tags to be NonNull, got %T", tagsField.Type)
	}
	if _, ok := nn.OfType.(*schema.List); !ok {
		t.Fatalf("expected tags inner to be List, got %T", nn.OfType)
	}

	commentsField := postT.Fields["comments"]
	if _, ok := commentsField.Type.(*schema.List); !ok {
		t.Fatalf("expected comments to be List, got %T", commentsField.Type)
	}
}

func TestParseSDLObjectWithArguments(t *testing.T) {
	s, err := ParseSDL(`
		type Query {
			user(id: ID!, verbose: Boolean = false): String
		}
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	q := s.Types["Query"].(*schema.Object)
	userField := q.Fields["user"]
	if len(userField.Args) != 2 {
		t.Fatalf("expected 2 args, got %d", len(userField.Args))
	}
}

func TestParseSDLInterfaceType(t *testing.T) {
	s, err := ParseSDL(`
		interface Node {
			id: ID!
		}
		type User implements Node {
			id: ID!
			name: String
		}
		type Query { node: Node }
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	iface, ok := s.Types["Node"].(*schema.Interface)
	if !ok {
		t.Fatalf("expected Node interface, got %T", s.Types["Node"])
	}
	if _, ok := iface.Fields["id"]; !ok {
		t.Fatal("expected id field on interface")
	}
	userT := s.Types["User"].(*schema.Object)
	if len(userT.Interfaces) != 1 || userT.Interfaces[0].TypeName != "Node" {
		t.Fatalf("expected User to implement Node, got %#v", userT.Interfaces)
	}
}

func TestParseSDLInterfaceImplementsInterface(t *testing.T) {
	// Issue #5: Interface implements lists including multi-interface inheritance
	s, err := ParseSDL(`
		interface Node {
			id: ID!
		}
		interface Entity implements Node {
			id: ID!
			name: String!
		}
		type Query { node: Node }
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	entity, ok := s.Types["Entity"].(*schema.Interface)
	if !ok {
		t.Fatalf("expected Entity interface, got %T", s.Types["Entity"])
	}
	if len(entity.Interfaces) != 1 || entity.Interfaces[0].TypeName != "Node" {
		t.Fatalf("expected Entity to implement Node, got %#v", entity.Interfaces)
	}
}

func TestParseSDLMultipleImplements(t *testing.T) {
	s, err := ParseSDL(`
		interface Node { id: ID! }
		interface Auditable { createdAt: String! }
		type User implements Node & Auditable {
			id: ID!
			createdAt: String!
		}
		type Query { user: User }
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	userT := s.Types["User"].(*schema.Object)
	if len(userT.Interfaces) != 2 {
		t.Fatalf("expected User to implement 2 interfaces, got %d", len(userT.Interfaces))
	}
}

func TestParseSDLUnionType(t *testing.T) {
	// Issue #5: Union member type lists
	s, err := ParseSDL(`
		type Cat { name: String }
		type Dog { name: String }
		union Pet = Cat | Dog
		type Query { pet: Pet }
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	union, ok := s.Types["Pet"].(*schema.Union)
	if !ok {
		t.Fatalf("expected Pet union, got %T", s.Types["Pet"])
	}
	if len(union.Types) != 2 {
		t.Fatalf("expected 2 union members, got %d", len(union.Types))
	}
}

func TestParseSDLEnumType(t *testing.T) {
	s, err := ParseSDL(`
		enum Status {
			ACTIVE
			INACTIVE
			PENDING
		}
		type Query { status: Status }
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	enum, ok := s.Types["Status"].(*schema.Enum)
	if !ok {
		t.Fatalf("expected Status enum, got %T", s.Types["Status"])
	}
	if len(enum.Values) != 3 {
		t.Fatalf("expected 3 enum values, got %d", len(enum.Values))
	}
}

func TestParseSDLInputObjectType(t *testing.T) {
	s, err := ParseSDL(`
		input CreateUserInput {
			name: String!
			email: String!
			age: Int
		}
		type Query { create(input: CreateUserInput): String }
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	inputT, ok := s.Types["CreateUserInput"].(*schema.InputObject)
	if !ok {
		t.Fatalf("expected CreateUserInput input, got %T", s.Types["CreateUserInput"])
	}
	if len(inputT.Fields) != 3 {
		t.Fatalf("expected 3 fields, got %d", len(inputT.Fields))
	}
}

func TestParseSDLInputObjectWithOneOf(t *testing.T) {
	// Issue #5: Oneof input object definitions
	s, err := ParseSDL(`
		input ContactInput @oneOf {
			email: String
			phone: String
		}
		type Query { contact(input: ContactInput): String }
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	inputT, ok := s.Types["ContactInput"].(*schema.InputObject)
	if !ok {
		t.Fatalf("expected ContactInput input, got %T", s.Types["ContactInput"])
	}
	if !inputT.IsOneOf {
		t.Fatal("expected IsOneOf to be true")
	}
}

func TestParseSDLDescriptionStrings(t *testing.T) {
	// Issue #5: Description strings on definitions
	s, err := ParseSDL(`
		"""A user in the system."""
		type User {
			"""The unique ID."""
			id: ID!
			"Display name"
			name: String
		}
		type Query { user: User }
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	userT := s.Types["User"].(*schema.Object)
	if userT.Description != "A user in the system." {
		t.Fatalf("expected type description, got %q", userT.Description)
	}
	if userT.Fields["id"].Description != "The unique ID." {
		t.Fatalf("expected field description, got %q", userT.Fields["id"].Description)
	}
	if userT.Fields["name"].Description != "Display name" {
		t.Fatalf("expected field description, got %q", userT.Fields["name"].Description)
	}
}

func TestParseSDLDirectiveDefinition(t *testing.T) {
	// Issue #5: Directive definitions in SDL
	s, err := ParseSDL(`
		directive @auth(role: String!) on FIELD_DEFINITION | OBJECT
		type Query { secret: String @auth(role: "admin") }
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.DirectiveDefinitions == nil {
		t.Fatal("expected DirectiveDefinitions to be non-nil")
	}
	dd, ok := s.DirectiveDefinitions["auth"]
	if !ok {
		t.Fatal("expected @auth directive definition")
	}
	if len(dd.Locations) != 2 {
		t.Fatalf("expected 2 locations, got %d", len(dd.Locations))
	}
}

func TestParseSDLRepeatableDirective(t *testing.T) {
	// Issue #5: Repeatable directive declarations
	s, err := ParseSDL(`
		directive @tag(name: String!) repeatable on FIELD_DEFINITION
		type Query { id: String @tag(name: "public") @tag(name: "internal") }
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	dd, ok := s.DirectiveDefinitions["tag"]
	if !ok {
		t.Fatal("expected @tag directive definition")
	}
	if !dd.IsRepeatable {
		t.Fatal("expected @tag to be repeatable")
	}
}

func TestParseSDLSchemaDefinition(t *testing.T) {
	s, err := ParseSDL(`
		type MyQuery { hello: String }
		type MyMutation { greet: String }
		schema {
			query: MyQuery
			mutation: MyMutation
		}
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Query == nil || s.Query.TypeName != "MyQuery" {
		t.Fatalf("expected query root MyQuery, got %v", s.Query)
	}
	if s.Mutation == nil || s.Mutation.TypeName != "MyMutation" {
		t.Fatalf("expected mutation root MyMutation, got %v", s.Mutation)
	}
}

func TestParseSDLDefaultQueryRootName(t *testing.T) {
	// When no schema block, type named Query is the default root.
	s, err := ParseSDL(`type Query { hello: String }`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Query == nil || s.Query.TypeName != "Query" {
		t.Fatalf("expected Query root, got %v", s.Query)
	}
}

func TestParseSDLExtendType(t *testing.T) {
	// Issue #5: Schema and type extend definitions
	s, err := ParseSDL(`
		type User { id: ID! }
		extend type User { name: String }
		type Query { user: User }
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	userT := s.Types["User"].(*schema.Object)
	if _, ok := userT.Fields["name"]; !ok {
		t.Fatal("expected name field from extend")
	}
	if _, ok := userT.Fields["id"]; !ok {
		t.Fatal("expected id field from original definition")
	}
}

func TestParseSDLExtendSchema(t *testing.T) {
	// Issue #5: Schema extend definitions
	s, err := ParseSDL(`
		type Query { hello: String }
		type Mutation { greet: String }
		extend schema {
			mutation: Mutation
		}
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Mutation == nil || s.Mutation.TypeName != "Mutation" {
		t.Fatalf("expected mutation root from extend schema, got %v", s.Mutation)
	}
}

func TestParseSDLErrorOnUnknownTypeRef(t *testing.T) {
	_, err := ParseSDL(`
		type Query { user: NonExistentType }
	`)
	if err == nil {
		t.Fatal("expected error for unknown type reference")
	}
	if !strings.Contains(err.Error(), "NonExistentType") {
		t.Fatalf("expected NonExistentType in error, got %v", err)
	}
}

func TestParseSDLEnumWithDeprecatedValue(t *testing.T) {
	s, err := ParseSDL(`
		enum Status {
			ACTIVE
			OLD @deprecated(reason: "Use ACTIVE")
		}
		type Query { status: Status }
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	enum := s.Types["Status"].(*schema.Enum)
	var oldVal *schema.EnumValue
	for i := range enum.Values {
		if enum.Values[i].Name == "OLD" {
			oldVal = &enum.Values[i]
		}
	}
	if oldVal == nil {
		t.Fatal("expected OLD enum value")
	}
	if !oldVal.IsDeprecated {
		t.Fatal("expected OLD to be deprecated")
	}
}

func TestParseSDLFieldWithDeprecated(t *testing.T) {
	s, err := ParseSDL(`
		type User {
			id: ID!
			legacyId: String @deprecated(reason: "Use id")
		}
		type Query { user: User }
	`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	userT := s.Types["User"].(*schema.Object)
	f := userT.Fields["legacyId"]
	if !f.IsDeprecated {
		t.Fatal("expected legacyId to be deprecated")
	}
	if f.DeprecationReason == nil || *f.DeprecationReason != "Use id" {
		t.Fatalf("expected deprecation reason, got %v", f.DeprecationReason)
	}
}

func TestComplexSDLParsingBranches(t *testing.T) {
	s, err := ParseSDL(`
		scalar Date
		interface Node { id: ID! }
		type User implements Node { id: ID! name: String age: Int }
		type Post { id: ID! title: String }
		union Search = User | Post
		enum Role { ADMIN USER }
		input Filter { term: String limit: Int }
		type Query { search(filter: Filter): [Search] user(id: ID!): User }
		type Mutation { noop: Boolean }
		type Subscription { changed: User }
	`)
	if err != nil {
		t.Fatalf("ParseSDL: %v", err)
	}
	if s.Query == nil || s.Mutation == nil || s.Subscription == nil {
		t.Fatalf("missing roots: %#v", s)
	}
	if _, ok := s.Types["Search"].(*schema.Union); !ok {
		t.Fatalf("missing union: %#v", s.Types["Search"])
	}
	rich, err := ParseSDL(`
		scalar Date
		schema { query: Root mutation: Mut subscription: Sub }
		interface RichNode { id: ID! }
		type Root implements RichNode {
			id: ID!
			field(arg: String, nums: [Int!]): String
		}
		type Mut { noop: Boolean }
		type Sub { changed: String }
		enum RichRole { ADMIN @deprecated(reason: "old") USER }
		input RichInput { term: String nums: [Int!] }
	`)
	if err != nil {
		t.Fatalf("rich SDL: %v", err)
	}
	if rich.Query == nil || rich.Mutation == nil || rich.Subscription == nil {
		t.Fatalf("rich SDL roots missing: %#v", rich)
	}
	for _, bad := range []string{
		`type`,
		`type Bad { field: Missing }`,
		`schema { query: Missing }`,
		`directive @bad(`,
		`enum Bad {`,
		`input Bad { field: }`,
	} {
		if _, err := ParseSDL(bad); err == nil {
			t.Fatalf("expected SDL parse error for %q", bad)
		}
	}
}

func TestParseSDLInputDefaultsRoundTripDefaults(t *testing.T) {
	s, err := ParseSDL(`
		enum Unit { KG }
		input Payload {
		  v: Float = -1.25
		  tags: [String] = ["a", "b"]
		  active: Boolean! = false
		  unit: Unit = KG
		}
		type Query {
			pass(p: Payload = { v: 0, tags: ["x"], active: true, unit: KG }): Boolean!
		}
	`)
	if err != nil {
		t.Fatalf("ParseSDL: %v", err)
	}
	in, ok := s.Types["Payload"].(*schema.InputObject)
	if !ok {
		t.Fatalf("expected Payload input object, got %T", s.Types["Payload"])
	}
	if len(in.Fields) != 4 {
		t.Fatalf("field count = %d", len(in.Fields))
	}
	if in.Fields["v"].DefaultValue == nil || in.Fields["active"].DefaultValue == nil {
		t.Fatalf("expected input-field defaults populated, got %+v %+v",
			in.Fields["v"].DefaultValue, in.Fields["active"].DefaultValue)
	}
	q := s.Types["Query"].(*schema.Object)
	passArg := q.Fields["pass"].Args[0]
	if passArg.DefaultValue == nil {
		t.Fatal("expected field argument structured default")
	}
}

func TestParseSDLExplicitSchemaRootsAndExtendQuery(t *testing.T) {
	s, err := ParseSDL(`
		schema @tag {
			query: RootQuery
			mutation: Mut
			subscription: Sub
		}
		directive @tag on SCHEMA
		type RootQuery { ok: Boolean }
		type Mut { bump: Boolean }
		type Sub { tick: Boolean }
		union SearchUnion = RootQuery | Mut
		extend type RootQuery {
			extraFlag: Boolean
		}
	`)
	if err != nil {
		t.Fatalf("ParseSDL: %v", err)
	}
	if s.Query == nil || s.Mutation == nil || s.Subscription == nil {
		t.Fatalf("explicit roots wired incorrectly: q=%v m=%v s=%v", s.Query != nil, s.Mutation != nil, s.Subscription != nil)
	}
	root := s.Types["RootQuery"].(*schema.Object)
	if _, ok := root.Fields["extraFlag"]; !ok {
		t.Fatal("expected extend fields merged into RootQuery")
	}
	if _, ok := s.Types["SearchUnion"].(*schema.Union); !ok {
		t.Fatalf("expected union type, got %T", s.Types["SearchUnion"])
	}
}

func TestParseSDLDirectiveArgsDescriptionsAndNestedInterfaceImplements(t *testing.T) {
	sdl := `
		directive @audit(
		  "telemetry key"
		  key: String! = "default"
		  "retry budget"
		  retries: Int @deprecated(reason: "use budget")
		  channel: Boolean = false
		) repeatable on FIELD_DEFINITION

		interface Identifiable { id: ID! }
		interface Auditable implements Identifiable {
		  id: ID!
		  auditedAt: String
		}
		type Account implements Identifiable & Auditable {
		  id: ID!
		  auditedAt: String @audit(key: "acct")
		  label: String
		}

		enum Stage { ALPHA @deprecated BETA }

		input PatchOps {
		  legacyRatio: Float @deprecated
		  note: String! = ""
		}

		type Query {
		  account: Account
		  patch(
		   "applied changes"
		   ops: PatchOps = {}
		  ): Boolean!
		}
	`
	s, err := ParseSDL(sdl)
	if err != nil {
		t.Fatalf("ParseSDL: %v", err)
	}
	dir, ok := s.DirectiveDefinitions["audit"]
	if !ok || dir == nil || !dir.IsRepeatable || len(dir.Args) != 3 {
		t.Fatalf("audit directive malformed: repeatable=%v args=%d", dir != nil && dir.IsRepeatable, len(dir.Args))
	}
	foundKeyDesc := false
	for _, iv := range dir.Args {
		if iv.Name == "key" && iv.Description == "telemetry key" {
			foundKeyDesc = true
		}
	}
	if !foundKeyDesc {
		t.Fatal("expected string descriptions on directive args")
	}

	audIface := s.Types["Auditable"].(*schema.Interface)
	if len(audIface.Interfaces) != 1 || audIface.Interfaces[0].Name() != "Identifiable" {
		t.Fatalf("expected Auditable to implement Identifiable, got %+v", audIface.Interfaces)
	}

	ev := s.Types["Stage"].(*schema.Enum)
	evByName := map[string]schema.EnumValue{}
	for _, v := range ev.Values {
		evByName[v.Name] = v
	}
	a := evByName["ALPHA"]
	if !a.IsDeprecated || a.DeprecationReason != nil {
		t.Fatalf("ALPHA deprecation = deprecated=%v reason=%v", a.IsDeprecated, a.DeprecationReason)
	}

	patch := s.Types["PatchOps"].(*schema.InputObject)
	lf := patch.Fields["legacyRatio"]
	if !lf.IsDeprecated || lf.DeprecationReason != nil {
		t.Fatalf("input field deprecation without reason: %#v %#v", lf.IsDeprecated, lf.DeprecationReason)
	}

	qField := s.Query.Fields["patch"]
	if len(qField.Args) != 1 || qField.Args[0].Description != "applied changes" {
		t.Fatalf("field arg description mismatch: %+v", qField.Args)
	}
	if qField.Args[0].DefaultValue == nil {
		t.Fatal("expected default value for PatchOps argument")
	}
}

func TestParseSDLSchemaDirectiveWithoutSelectionsBlock(t *testing.T) {
	// Covers parseSchemaDef when no { ... } block follows (valid empty schema declaration).
	s, err := ParseSDL(`
		directive @marker on SCHEMA
		schema @marker
		type Query { ping: Boolean! }
	`)
	if err != nil {
		t.Fatalf("ParseSDL: %v", err)
	}
	if s.Query == nil {
		t.Fatal("expected default Query root from convention")
	}
}

func TestParseSDLFailureMatrixBranches(t *testing.T) {
	// Targets SDL parser/parser-builder error exits (eof, expect mismatches).
	docs := []string{
		`junk`,
		`scalar`,
		`scalar @`,
		`type`,
		`interface`,
		`union`,
		`enum`,
		`enum E { `,
		`input`,
		`input I { `,
		`schema {`,
		`schema { query: `,
		`directive @`,
		`directive @audit repeatable`,
		`extend type `,
		`extend input `,
		`type Query { x: TotallyMissingType }`,
		"type Q { _: [[ID!",
		`fragment Illegal on Query { x: ID }`,
		`type Bad { fld(: Int): Int }`,
		`union U = | Person`,
		`directive @dup ( ") on FIELD_DEFINITION`,
		`enum E { V @ `,
		`input Bad { foo: = 1 }`,
		"type Bad2 { _: ]ID }",
		`type Zap { _: @illegal }`,
		`enum Color { RED BLUE`,
		`interface Edge { `,
	}
	for _, doc := range docs {
		if _, err := ParseSDL(doc); err == nil {
			t.Fatalf("expected error for %#v", doc)
		}
	}
	if _, err := ParseSDL("\"unclosed"); err == nil {
		t.Fatal("expected lex/parse failure for dangling string literal")
	}
}
