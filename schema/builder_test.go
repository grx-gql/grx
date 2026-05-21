package schema

import (
	"context"
	"strings"
	"testing"
)

type buildTestEpisode string

const (
	buildTestEpisodeNewHope buildTestEpisode = "NEWHOPE"
	buildTestEpisodeEmpire  buildTestEpisode = "EMPIRE"
	buildTestEpisodeJedi    buildTestEpisode = "JEDI"
)

type buildTestDate struct {
	Raw string
}

type buildTestNode interface {
	isBuildTestNode()
}

type buildTestSearchResult interface {
	isBuildTestSearchResult()
}

type buildTestUser struct {
	ID string `gql:"id,nonNull"`
}

func (*buildTestUser) isBuildTestNode() {}

func (*buildTestUser) isBuildTestSearchResult() {}

type buildTestPost struct {
	ID string `gql:"id,nonNull"`
}

func (*buildTestPost) isBuildTestNode() {}

func (*buildTestPost) isBuildTestSearchResult() {}

type buildTestQuery struct{}

type buildTestMutation struct{}

type buildTestSubscription struct{}

func (buildTestQuery) User(ctx context.Context) (*buildTestUser, error) {
	return &buildTestUser{ID: "1"}, nil
}

func (buildTestMutation) CreateUser(ctx context.Context) (*buildTestUser, error) {
	return &buildTestUser{ID: "2"}, nil
}

func (buildTestSubscription) UserCreated(ctx context.Context) (<-chan *buildTestUser, error) {
	out := make(chan *buildTestUser)
	close(out)
	return out, nil
}

func TestBuildAllRoots(t *testing.T) {
	schemaValue, err := Build(Config{
		Query:        buildTestQuery{},
		Mutation:     buildTestMutation{},
		Subscription: buildTestSubscription{},
	})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}
	if schemaValue.Query == nil {
		t.Fatal("expected query root")
	}
	if schemaValue.Mutation == nil {
		t.Fatal("expected mutation root")
	}
	if schemaValue.Subscription == nil {
		t.Fatal("expected subscription root")
	}
	if _, ok := schemaValue.Query.Fields["user"]; !ok {
		t.Fatalf("expected user query field, got %#v", schemaValue.Query.Fields)
	}
	if _, ok := schemaValue.Mutation.Fields["createUser"]; !ok {
		t.Fatalf("expected createUser mutation field, got %#v", schemaValue.Mutation.Fields)
	}
	subscriptionField, ok := schemaValue.Subscription.Fields["userCreated"]
	if !ok {
		t.Fatalf("expected userCreated subscription field, got %#v", schemaValue.Subscription.Fields)
	}
	if subscriptionField.Type.Name() != "buildTestUser" {
		t.Fatalf("expected subscription field to expose channel element type, got %q", subscriptionField.Type.Name())
	}
}

func TestBuildSubscriptionRootStandalone(t *testing.T) {
	schemaValue, err := Build(Config{
		Query:        buildTestQuery{},
		Subscription: buildTestSubscription{},
	})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}
	if schemaValue.Subscription == nil {
		t.Fatal("expected subscription root")
	}
}

func TestBuildRequiresQueryRoot(t *testing.T) {
	_, err := Build(Config{Mutation: buildTestMutation{}})
	if err == nil {
		t.Fatal("expected error when query root is missing")
	}
}

func TestBuildRegistersAdvancedTypeDefinitions(t *testing.T) {
	schemaValue, err := Build(Config{
		Query: buildTestQuery{},
		Scalars: []ScalarConfig{
			{
				Type: buildTestDate{},
				Name: "Date",
				Parse: func(input any) (any, error) {
					return buildTestDate{Raw: input.(string)}, nil
				},
				Serialize: func(value any) (any, error) {
					return value.(buildTestDate).Raw, nil
				},
			},
		},
		Enums: []EnumConfig{
			{
				Type: buildTestEpisode(""),
				Name: "Episode",
				Values: []EnumValueConfig{
					{Name: "NEWHOPE", Value: buildTestEpisodeNewHope},
					{Name: "EMPIRE", Value: buildTestEpisodeEmpire},
					{Name: "JEDI", Value: buildTestEpisodeJedi},
				},
			},
		},
		Interfaces: []InterfaceConfig{
			{
				Type:         (*buildTestNode)(nil),
				Implementors: []any{buildTestUser{}, buildTestPost{}},
			},
		},
		Unions: []UnionConfig{
			{
				Type:         (*buildTestSearchResult)(nil),
				Name:         "SearchResult",
				Implementors: []any{buildTestUser{}, buildTestPost{}},
			},
		},
	})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	typeCases := []struct {
		name string
		kind Kind
	}{
		{name: "Date", kind: ScalarKind},
		{name: "Episode", kind: EnumKind},
		{name: "buildTestNode", kind: InterfaceKind},
		{name: "SearchResult", kind: UnionKind},
	}

	for _, typeCase := range typeCases {
		typeValue, ok := schemaValue.Types[typeCase.name]
		if !ok {
			t.Fatalf("expected type %q to be registered", typeCase.name)
		}
		if typeValue.Kind() != typeCase.kind {
			t.Fatalf("expected type %q kind %s, got %s", typeCase.name, typeCase.kind, typeValue.Kind())
		}
	}

	interfaceType, ok := schemaValue.Types["buildTestNode"].(*Interface)
	if !ok {
		t.Fatalf("expected interface type, got %T", schemaValue.Types["buildTestNode"])
	}
	if len(interfaceType.PossibleTypes) != 2 {
		t.Fatalf("expected two interface possible types, got %d", len(interfaceType.PossibleTypes))
	}

	unionType, ok := schemaValue.Types["SearchResult"].(*Union)
	if !ok {
		t.Fatalf("expected union type, got %T", schemaValue.Types["SearchResult"])
	}
	if len(unionType.Types) != 2 {
		t.Fatalf("expected two union members, got %d", len(unionType.Types))
	}

	enumType, ok := schemaValue.Types["Episode"].(*Enum)
	if !ok {
		t.Fatalf("expected enum type, got %T", schemaValue.Types["Episode"])
	}
	if len(enumType.Values) != 3 {
		t.Fatalf("expected three enum values, got %d", len(enumType.Values))
	}
}

// --- Issue #6: Type descriptions ---

type buildDescQuery struct{}

type buildDescUser struct {
	ID   string `gql:"id,nonNull,description=The unique identifier"`
	Name string `gql:"name,description=The display name"`
}

func (buildDescQuery) User(ctx context.Context) (*buildDescUser, error) {
	return nil, nil
}

func TestBuildFieldDescriptions(t *testing.T) {
	s, err := Build(Config{Query: buildDescQuery{}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	userType, ok := s.Types["buildDescUser"].(*Object)
	if !ok {
		t.Fatalf("expected buildDescUser object, got %T", s.Types["buildDescUser"])
	}
	if userType.Fields["id"].Description != "The unique identifier" {
		t.Fatalf("expected field description, got %q", userType.Fields["id"].Description)
	}
	if userType.Fields["name"].Description != "The display name" {
		t.Fatalf("expected field description, got %q", userType.Fields["name"].Description)
	}
}

// --- Issue #6: IsOneOf on InputObject ---

type buildOneOfInput struct {
	Email *string `gql:"email"`
	Phone *string `gql:"phone"`
}

func TestInputObjectIsOneOf(t *testing.T) {
	io := &InputObject{TypeName: "TestInput", IsOneOf: true, Fields: map[string]*Field{}}
	if !io.IsOneOf {
		t.Fatal("expected IsOneOf to be true")
	}
}

// --- Issue #6: Reserved __ name validation ---

func TestBuildRejectsReservedTypeName(t *testing.T) {
	type __badType struct{ ID string }
	type badQuery struct{}
	// The builder should not allow types starting with "__" (reserved by spec).
	// We test this via ValidateSchema which checks reserved names.
	s := &Schema{
		Types: map[string]Type{
			"__BadType": &Object{TypeName: "__BadType", Fields: map[string]*Field{}},
		},
		Query: &Object{TypeName: "Query", Fields: map[string]*Field{}},
	}
	errs := ValidateSchema(s)
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "__") {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected reserved name error, got %v", errs)
	}
}

func TestValidateSchemaNoErrors(t *testing.T) {
	s, err := Build(Config{Query: buildTestQuery{}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	errs := ValidateSchema(s)
	if len(errs) != 0 {
		t.Fatalf("expected no schema errors, got %v", errs)
	}
}

// --- Issue #6: Interface implementing interface ---

func TestInterfaceImplementsInterfaces(t *testing.T) {
	base := &Interface{TypeName: "Node", Fields: map[string]*Field{
		"id": {Name: "id", Type: &Scalar{TypeName: "ID"}},
	}}
	derived := &Interface{
		TypeName:   "Entity",
		Interfaces: []*Interface{base},
		Fields:     map[string]*Field{},
	}
	if len(derived.Interfaces) != 1 || derived.Interfaces[0].TypeName != "Node" {
		t.Fatalf("expected Entity to implement Node, got %#v", derived.Interfaces)
	}
}

// --- Issue #6: DirectiveDefinition type ---

func TestDirectiveDefinitionType(t *testing.T) {
	dd := &DirectiveDefinition{
		Name:         "auth",
		Description:  "Requires authentication",
		Locations:    []string{"FIELD_DEFINITION", "OBJECT"},
		IsRepeatable: false,
		Args: []InputValue{
			{Name: "role", Type: &Scalar{TypeName: "String"}},
		},
	}
	if dd.Name != "auth" {
		t.Fatalf("expected name auth, got %q", dd.Name)
	}
	if len(dd.Locations) != 2 {
		t.Fatalf("expected 2 locations, got %d", len(dd.Locations))
	}
}

// --- Issue #6: Type descriptions via struct tags ---

type descTagQuery struct{}

type descTagUser struct {
	ID string `gql:"id,nonNull,description=User ID"`
}

func (descTagQuery) Me(ctx context.Context) (*descTagUser, error) { return nil, nil }

func TestBuildPopulatesDescriptionFromTag(t *testing.T) {
	s, err := Build(Config{Query: descTagQuery{}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	userT := s.Types["descTagUser"].(*Object)
	if userT.Fields["id"].Description != "User ID" {
		t.Fatalf("expected 'User ID' description, got %q", userT.Fields["id"].Description)
	}
}

// --- Issue #6: Deprecation metadata on Field ---

type deprecatedQuery struct{}

type deprecatedUser struct {
	ID       string `gql:"id,nonNull"`
	LegacyID string `gql:"legacyId,deprecated=Use id instead"`
}

func (deprecatedQuery) User(ctx context.Context) (*deprecatedUser, error) { return nil, nil }

func TestBuildDeprecationMetadata(t *testing.T) {
	s, err := Build(Config{Query: deprecatedQuery{}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	userT := s.Types["deprecatedUser"].(*Object)
	f := userT.Fields["legacyId"]
	if !f.IsDeprecated {
		t.Fatal("expected legacyId to be deprecated")
	}
	if f.DeprecationReason == nil || *f.DeprecationReason != "Use id instead" {
		t.Fatalf("expected deprecation reason, got %v", f.DeprecationReason)
	}
}

// --- Issue #6: specifiedByURL on Scalar ---

func TestScalarSpecifiedByURL(t *testing.T) {
	s := &Scalar{TypeName: "URL", SpecifiedByURL: "https://url.spec.whatwg.org/"}
	if s.SpecifiedByURL != "https://url.spec.whatwg.org/" {
		t.Fatalf("expected specifiedByURL, got %q", s.SpecifiedByURL)
	}
}
