package exec

import (
	"context"
	"strings"
	"testing"

	"github.com/patrickkabwe/grx/core"
	"github.com/patrickkabwe/grx/schema"
)

func TestFlattenSelectionsDeferAndFragments(t *testing.T) {
	object := &schema.Object{TypeName: "User"}
	e := &Executor{}
	fragments := map[string]*fragmentDef{
		"UserFields": {
			Name:          "UserFields",
			TypeCondition: "User",
			Selections: []selection{
				{Name: "id"},
				{Name: "name"},
			},
		},
	}
	flat, defers, errs := e.flattenSelections(true, object, []selection{
		{FragmentSpread: "UserFields", Directives: []directive{{Name: "defer", Args: map[string]any{"label": "later"}}}},
		{InlineFragmentOn: "Other", Selections: []selection{{Name: "skipMe"}}},
		{InlineFragmentOn: "User", Directives: []directive{{Name: "include", Args: map[string]any{"if": true}}}, Selections: []selection{{Name: "email"}}},
	}, fragments)
	if len(errs) != 0 {
		t.Fatalf("flatten errors: %#v", errs)
	}
	if len(flat) != 1 || len(defers) != 1 {
		t.Fatalf("flat=%#v defers=%#v", flat, defers)
	}
	if active, label := deferDirective([]directive{{Name: "defer", Args: map[string]any{"if": variableRef{Name: "missing"}}}}); !active || label != "" {
		t.Fatalf("missing defer variable should remain active without label: %v %q", active, label)
	}
	if active, count, label := streamDirective([]directive{{Name: "stream", Args: map[string]any{"initialCount": int64(2), "label": "s"}}}); !active || count != 2 || label != "s" {
		t.Fatalf("stream directive = %v %d %q", active, count, label)
	}
}

func builtinDirSchema(t *testing.T) *schema.Schema {
	t.Helper()
	type builtinQuery struct{}
	type builtinUser struct {
		ID   string `gql:"id,nonNull"`
		Name string `gql:"name"`
	}
	s, err := schema.Build(schema.Config{
		Query: struct {
			builtinQuery
		}{},
	})
	// Use a fresh schema via SDL so we get all the types.
	s2, err2 := ParseSDL(`
		type Query {
			user(id: ID!): User
			items: [String]
		}
		type User {
			id: ID!
			name: String
		}
	`)
	_ = err
	_ = s
	if err2 != nil {
		t.Fatalf("build schema: %v", err2)
	}
	return s2
}

func findBuiltinDirective(name string) bool {
	for _, d := range introspectionBuiltinDirectives() {
		obj, ok := d.(*core.OrderedObject)
		if !ok {
			continue
		}
		for _, f := range obj.Fields() {
			if f.Name == "name" && f.Value == name {
				return true
			}
		}
	}
	return false
}

// @deprecated should appear in introspection built-in directives.

func TestIntrospectionIncludesDeprecatedDirective(t *testing.T) {
	if !findBuiltinDirective("deprecated") {
		t.Fatal("expected @deprecated in introspection built-in directives")
	}
}

// @specifiedBy should appear in introspection built-in directives.

func TestIntrospectionIncludesSpecifiedByDirective(t *testing.T) {
	if !findBuiltinDirective("specifiedBy") {
		t.Fatal("expected @specifiedBy in introspection built-in directives")
	}
}

// @oneOf should appear in introspection built-in directives.

func TestIntrospectionIncludesOneOfDirective(t *testing.T) {
	if !findBuiltinDirective("oneOf") {
		t.Fatal("expected @oneOf in introspection built-in directives")
	}
}

// @defer should appear in introspection built-in directives.

func TestIntrospectionIncludesDeferDirective(t *testing.T) {
	if !findBuiltinDirective("defer") {
		t.Fatal("expected @defer in introspection built-in directives")
	}
}

// @stream should appear in introspection built-in directives.

func TestIntrospectionIncludesStreamDirective(t *testing.T) {
	if !findBuiltinDirective("stream") {
		t.Fatal("expected @stream in introspection built-in directives")
	}
}

// @defer is a valid directive on FIELD — validation should not reject it.

func TestValidationAllowsDeferOnField(t *testing.T) {
	s := builtinDirSchema(t)
	bundle, err := parseDocumentBundle(`{ user(id: "1") @defer { id } }`, nil, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	doc, err := selectOperation(bundle, "")
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	errs := ValidateDocument(s, bundle, doc)
	for _, e := range errs {
		if strings.Contains(e.Error(), `Unknown directive "@defer"`) {
			t.Fatalf("unexpected validation error: %v", e)
		}
	}
}

// @stream is a valid directive on FIELD — validation should not reject it.

func TestValidationAllowsStreamOnField(t *testing.T) {
	s := builtinDirSchema(t)
	bundle, err := parseDocumentBundle(`{ items @stream(initialCount: 0) }`, nil, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	doc, err := selectOperation(bundle, "")
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	errs := ValidateDocument(s, bundle, doc)
	for _, e := range errs {
		if strings.Contains(e.Error(), `Unknown directive "@stream"`) {
			t.Fatalf("unexpected validation error: %v", e)
		}
	}
}

// @deprecated is a schema-side directive and should be rejected on a field selection.

func TestValidationRejectsDeprecatedOnFieldSelection(t *testing.T) {
	s := builtinDirSchema(t)
	bundle, err := parseDocumentBundle(`{ user(id: "1") @deprecated { id } }`, nil, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	doc, err := selectOperation(bundle, "")
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	errs := ValidateDocument(s, bundle, doc)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "@deprecated") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected validation error for @deprecated on field selection")
	}
}

// @oneOf is a schema-side directive and should be rejected on a field selection.

func TestValidationRejectsOneOfOnFieldSelection(t *testing.T) {
	s := builtinDirSchema(t)
	bundle, err := parseDocumentBundle(`{ user(id: "1") @oneOf { id } }`, nil, 0)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	doc, err := selectOperation(bundle, "")
	if err != nil {
		t.Fatalf("select: %v", err)
	}
	errs := ValidateDocument(s, bundle, doc)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "@oneOf") {
			found = true
		}
	}
	if !found {
		t.Fatal("expected validation error for @oneOf on field selection")
	}
}

// Introspection of a schema built from SDL should surface @deprecated on fields.

func TestIntrospectionShowsDeprecatedFieldInfo(t *testing.T) {
	s, err := ParseSDL(`
		type Query {
			user: String
			legacy: String @deprecated(reason: "old")
		}
	`)
	if err != nil {
		t.Fatalf("SDL: %v", err)
	}
	executor := New(s, nil)
	resp := executor.Execute(context.Background(), core.Request{
		Query: `{ __type(name: "Query") { fields(includeDeprecated: true) { name isDeprecated deprecationReason } } }`,
	})
	if len(resp.Errors) != 0 {
		t.Fatalf("unexpected errors: %v", resp.Errors)
	}
}
