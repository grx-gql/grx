package exec

import (
	"context"
	"strings"
	"testing"

	"github.com/patrickkabwe/grx/core"
	"github.com/patrickkabwe/grx/schema"
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
	if _, exists := response.Data.(any); exists && response.Data != nil {
		t.Fatalf("expected no data, got %#v", response.Data)
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
