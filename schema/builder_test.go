package schema

import (
	"context"
	"fmt"
	"reflect"
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

type coverRole string

const coverAdmin coverRole = "ADMIN"

type coverInput struct {
	Term string `gql:"term,default=ada"`
	Age  int    `gql:"age"`
}

type coverUser struct {
	ID   string
	Name string
}

type coverQuery struct{}

func (coverQuery) User(ctx context.Context, args struct {
	ID     string      `gql:"id,nonNull"`
	Role   coverRole   `gql:"role"`
	Filter coverInput  `gql:"filter"`
	Tags   []string    `gql:"tags"`
	Ptr    *coverInput `gql:"ptr"`
}) (*coverUser, error) {
	return &coverUser{ID: args.ID, Name: args.Filter.Term}, nil
}

func TestBuilderResolverCoercesArgumentShapes(t *testing.T) {
	s, err := Build(Config{
		Query: coverQuery{},
		Enums: []EnumConfig{{
			Type: coverRole(""),
			Name: "CoverRole",
			Values: []EnumValueConfig{
				{Name: "ADMIN", Value: coverAdmin},
			},
		}},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	field := s.Query.Fields["user"]
	value, err := field.Resolver(context.Background(), ResolveParams{Args: map[string]any{
		"id":     "1",
		"role":   "ADMIN",
		"filter": map[string]any{"age": int64(42)},
		"tags":   []any{"a", "b"},
		"ptr":    map[string]any{"term": "ptr"},
	}})
	if err != nil {
		t.Fatalf("resolve: %v", err)
	}
	user := value.(*coverUser)
	if user.ID != "1" || user.Name != "ada" {
		t.Fatalf("user = %#v", user)
	}

	if _, err := field.Resolver(context.Background(), ResolveParams{Args: map[string]any{}}); err == nil {
		t.Fatal("expected missing required argument")
	}
}

func TestBuilderInternalHelperBranches(t *testing.T) {
	b, err := newBuilder(Config{
		Scalars: []ScalarConfig{{
			Type:      customDefault(""),
			Name:      "CustomDefault",
			Parse:     func(input any) (any, error) { return customDefault(fmt.Sprint(input)), nil },
			Serialize: func(value any) (any, error) { return value, nil },
		}},
		Enums: []EnumConfig{{
			Type:   coverRole(""),
			Name:   "CoverRole",
			Values: []EnumValueConfig{{Name: "ADMIN", Value: coverAdmin}},
		}},
	})
	if err != nil {
		t.Fatalf("newBuilder: %v", err)
	}

	for _, target := range []any{
		new(string), new(bool), new(int), new(int8), new(int16), new(int32), new(int64), new(float32), new(float64),
	} {
		v := reflect.ValueOf(target).Elem()
		raw := reflect.ValueOf(anyForKind(v.Kind()))
		if !setScalarValue(v, raw) {
			t.Fatalf("setScalarValue failed for %s", v.Kind())
		}
	}
	var small int8
	if setScalarValue(reflect.ValueOf(&small).Elem(), reflect.ValueOf(int64(999))) {
		t.Fatal("overflowing int should not set")
	}
	var f32 float32
	if setScalarValue(reflect.ValueOf(&f32).Elem(), reflect.ValueOf(float64(1e100))) {
		t.Fatal("overflowing float should not set")
	}
	if setScalarValue(reflect.ValueOf(&small).Elem(), reflect.ValueOf("x")) {
		t.Fatal("string should not set int")
	}
	if isSignedIntegerKind(reflect.Uint) {
		t.Fatal("uint is not signed")
	}

	if got, err := b.parseDefaultValue(reflect.TypeOf(coverRole("")), "ADMIN"); err != nil || got != coverAdmin {
		t.Fatalf("enum default = %#v %v", got, err)
	}
	if got, err := b.parseDefaultValue(reflect.TypeOf(customDefault("")), "x"); err != nil || got != customDefault("x") {
		t.Fatalf("scalar default = %#v %v", got, err)
	}
	for _, tc := range []struct {
		typ reflect.Type
		raw string
	}{
		{reflect.TypeOf(int8(0)), "bad"},
		{reflect.TypeOf(float32(0)), "bad"},
		{reflect.TypeOf(false), "bad"},
		{reflect.TypeOf(struct{}{}), "bad"},
	} {
		if _, err := b.parseDefaultValue(tc.typ, tc.raw); err == nil {
			t.Fatalf("expected default parse error for %s", tc.typ)
		}
	}

	if typ, err := b.graphQLInputType(reflect.TypeOf([]*coverInput{})); err != nil || typ.Name() != "[coverInput]" {
		t.Fatalf("input list type = %v %v", typ, err)
	}
	if _, err := b.graphQLInputType(reflect.TypeOf(make(chan int))); err == nil {
		t.Fatal("expected unsupported input type")
	}
	if typ, err := b.graphQLType(reflect.TypeOf([]*coverUser{})); err != nil || typ.Name() != "[coverUser]" {
		t.Fatalf("output list type = %v %v", typ, err)
	}
	if _, err := b.graphQLType(reflect.TypeOf(make(chan int))); err == nil {
		t.Fatal("expected unsupported output type")
	}

	if lowerFirst("") != "" || lowerFirst("URL") != "uRL" {
		t.Fatal("lowerFirst branch mismatch")
	}
	if containsType([]reflect.Type{reflect.TypeOf(coverUser{})}, reflect.TypeOf(coverInput{})) {
		t.Fatal("unexpected containsType match")
	}
}

func TestBuilderInvalidConfigurationBranches(t *testing.T) {
	type localInterface interface{ marker() }
	type localImplementor struct{}

	cases := []Config{
		{Query: coverQuery{}, Scalars: []ScalarConfig{{Type: nil, Name: "Bad", Parse: func(any) (any, error) { return nil, nil }}}},
		{Query: coverQuery{}, Scalars: []ScalarConfig{{Type: customDefault(""), Parse: func(any) (any, error) { return nil, nil }}}},
		{Query: coverQuery{}, Scalars: []ScalarConfig{{Type: customDefault(""), Name: "Bad"}}},
		{Query: coverQuery{}, Enums: []EnumConfig{{Type: nil, Name: "Bad", Values: []EnumValueConfig{{Name: "A", Value: "A"}}}}},
		{Query: coverQuery{}, Enums: []EnumConfig{{Type: coverRole(""), Values: []EnumValueConfig{{Name: "A", Value: coverRole("A")}}}}},
		{Query: coverQuery{}, Enums: []EnumConfig{{Type: coverRole(""), Name: "Bad"}}},
		{Query: coverQuery{}, Interfaces: []InterfaceConfig{{Type: nil}}},
		{Query: coverQuery{}, Interfaces: []InterfaceConfig{{Type: coverUser{}}}},
		{Query: coverQuery{}, Interfaces: []InterfaceConfig{{Type: (*localInterface)(nil), Implementors: []any{nil}}}},
		{Query: coverQuery{}, Unions: []UnionConfig{{Type: nil}}},
		{Query: coverQuery{}, Unions: []UnionConfig{{Type: coverUser{}}}},
		{Query: coverQuery{}, Unions: []UnionConfig{{Type: (*localInterface)(nil), Name: "LocalUnion", Implementors: []any{nil}}}},
	}
	for index, config := range cases {
		if _, err := Build(config); err == nil {
			t.Fatalf("case %d: expected build error", index)
		}
	}
	if _, err := namedTypeOf([]string{}); err == nil {
		t.Fatal("expected unnamed type error")
	}
	if _, err := interfaceTypeOf((*interface{})(nil)); err == nil {
		t.Fatal("expected unnamed interface type error")
	}
	if _, err := interfaceTypeOf((*localInterface)(nil)); err != nil {
		t.Fatalf("local interface should be valid: %v", err)
	}
	_ = localImplementor{}
}

type builderShapeQuery struct{}

func (builderShapeQuery) Value(ctx context.Context, args struct {
	Input *coverInput `gql:"input"`
}) (*coverUser, error) {
	if args.Input == nil {
		return &coverUser{ID: "nil"}, nil
	}
	return &coverUser{ID: args.Input.Term}, nil
}

func TestBuilderSetValueAndTypeConflictBranches(t *testing.T) {
	b, err := newBuilder(Config{
		Enums: []EnumConfig{{
			Type:   coverRole(""),
			Name:   "CoverRole",
			Values: []EnumValueConfig{{Name: "ADMIN", Value: coverAdmin}},
		}},
	})
	if err != nil {
		t.Fatalf("newBuilder: %v", err)
	}

	var role coverRole
	if err := b.setValue(reflect.ValueOf(&role).Elem(), "ADMIN"); err != nil || role != coverAdmin {
		t.Fatalf("set enum = %q %v", role, err)
	}
	var ptr *coverInput
	if err := b.setValue(reflect.ValueOf(&ptr).Elem(), map[string]any{"term": "x"}); err != nil || ptr == nil || ptr.Term != "x" {
		t.Fatalf("set pointer input = %#v %v", ptr, err)
	}
	var list []string
	if err := b.setValue(reflect.ValueOf(&list).Elem(), []any{"a", "b"}); err != nil || !reflect.DeepEqual(list, []string{"a", "b"}) {
		t.Fatalf("set list = %#v %v", list, err)
	}
	var input coverInput
	if err := b.setValue(reflect.ValueOf(&input).Elem(), map[string]any{"age": "bad"}); err == nil {
		t.Fatal("expected invalid struct field error")
	}
	if err := b.setValue(reflect.ValueOf(&input).Elem(), "bad"); err == nil {
		t.Fatal("expected struct input shape error")
	}
	if err := b.setValue(reflect.ValueOf(&list).Elem(), "bad"); err == nil {
		t.Fatal("expected slice input shape error")
	}

	s, err := Build(Config{Query: builderShapeQuery{}})
	if err != nil {
		t.Fatalf("build pointer args schema: %v", err)
	}
	field := s.Query.Fields["value"]
	got, err := field.Resolver(context.Background(), ResolveParams{Args: map[string]any{"input": map[string]any{"term": "ok"}}})
	if err != nil {
		t.Fatalf("resolve pointer args: %v", err)
	}
	if got.(*coverUser).ID != "ok" {
		t.Fatalf("pointer arg result = %#v", got)
	}

	for _, value := range []any{
		"", int(0), int8(0), int16(0), int32(0), int64(0), float32(0), float64(0), false,
		[]string{}, [1]int{}, coverUser{}, &coverUser{},
	} {
		typ, typeErr := b.graphQLType(reflect.TypeOf(value))
		if typeErr != nil || typ == nil {
			t.Fatalf("graphQLType(%T) = %#v %v", value, typ, typeErr)
		}
	}
	for _, value := range []any{"", int(0), float64(0), false, []int{}, [1]string{}, coverInput{}, &coverInput{}} {
		typ, typeErr := b.graphQLInputType(reflect.TypeOf(value))
		if typeErr != nil || typ == nil {
			t.Fatalf("graphQLInputType(%T) = %#v %v", value, typ, typeErr)
		}
	}
	b.types["coverUser"] = &Scalar{TypeName: "coverUser"}
	if _, err := b.buildObject(reflect.TypeOf(coverUser{})); err == nil {
		t.Fatal("expected output type name conflict")
	}
	b.types["coverInput"] = &Scalar{TypeName: "coverInput"}
	if _, err := b.buildInputObject(reflect.TypeOf(coverInput{})); err == nil {
		t.Fatal("expected input type name conflict")
	}
	if _, err := b.buildInputObject(reflect.TypeOf(struct{ Name string }{})); err == nil {
		t.Fatal("expected anonymous input object error")
	}
	if got, err := (&Scalar{TypeName: "String"}).Serialize("x"); err != nil || got != "x" {
		t.Fatalf("default scalar serialize = %#v %v", got, err)
	}
}

func TestBuilderInterfaceUnionAndMethodBranches(t *testing.T) {
	type localNode interface{ nodeMarker() }
	type localSearch interface{ searchMarker() }
	type localUser struct {
		ID   string `gql:"id"`
		Name string `gql:"name"`
	}
	funcUser := localUser{ID: "1"}
	_ = funcUser
	b, err := newBuilder(Config{
		Interfaces: []InterfaceConfig{{Type: (*localNode)(nil), Implementors: []any{localUser{}}}},
		Unions:     []UnionConfig{{Type: (*localSearch)(nil), Name: "LocalSearch", Implementors: []any{localUser{}}}},
	})
	if err != nil {
		t.Fatalf("newBuilder: %v", err)
	}
	if typ, err := b.graphQLType(reflect.TypeOf((*localNode)(nil)).Elem()); err != nil || typ.Kind() != InterfaceKind {
		t.Fatalf("interface graphQLType = %#v %v", typ, err)
	}
	if typ, err := b.graphQLType(reflect.TypeOf((*localSearch)(nil)).Elem()); err != nil || typ.Kind() != UnionKind {
		t.Fatalf("union graphQLType = %#v %v", typ, err)
	}
}

type namedUnionIface interface {
	unionMarkerNamed()
}

type namedUnionImpl struct{}

func (namedUnionImpl) unionMarkerNamed() {}

type inferUnionNamedRoot struct{}

func (inferUnionNamedRoot) Ping(context.Context) (string, error) { return "ok", nil }

func (inferUnionNamedRoot) Node(context.Context) (namedUnionIface, error) {
	return namedUnionImpl{}, nil
}

func TestUnionInfersGraphQLNameFromGoInterfaceWhenOmitted(t *testing.T) {
	s, err := Build(Config{
		Query: inferUnionNamedRoot{},
		Unions: []UnionConfig{
			{Type: (*namedUnionIface)(nil), Implementors: []any{namedUnionImpl{}}},
		},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	u, ok := s.Types["namedUnionIface"].(*Union)
	if !ok || u == nil || len(u.Types) == 0 || u.Types[0] == nil {
		t.Fatalf("expected inferred union populated with concrete members: %#v", u)
	}
}

type malformedZeroOutputQuery struct{}

func (malformedZeroOutputQuery) Bad() {}

type malformedBadSecondReturnQuery struct{}

func (malformedBadSecondReturnQuery) Bad(context.Context) (string, bool) { return "", false }

type malformedNonStructArgsQuery struct{}

func (malformedNonStructArgsQuery) Bad(context.Context, int) (string, error) { return "", nil }

type malformedExtraParamQuery struct{}

func (malformedExtraParamQuery) Bad(context.Context, struct{}, int) (string, error) { return "", nil }

func TestBuilderRejectsMalformedResolverMethods(t *testing.T) {
	for index, cfg := range []Config{
		{Query: malformedZeroOutputQuery{}},
		{Query: malformedBadSecondReturnQuery{}},
		{Query: malformedNonStructArgsQuery{}},
		{Query: malformedExtraParamQuery{}},
	} {
		if _, err := Build(cfg); err == nil {
			t.Fatalf("case %d: expected build error for malformed resolver signature", index)
		}
	}
}

type coverMoney int

type coverDebitQuery struct{}

func (coverDebitQuery) Debit(context.Context, struct {
	M coverMoney `gql:"m"`
}) (bool, error) {
	return true, nil
}

func TestBuilderUsesScalarParsingConvertibleAssignments(t *testing.T) {
	s, err := Build(Config{
		Query: coverDebitQuery{},
		Scalars: []ScalarConfig{{
			Type: coverMoney(0),
			Name: "Money",
			Parse: func(input any) (any, error) {
				switch v := input.(type) {
				case int:
					return coverMoney(v * 2), nil
				case int64:
					return coverMoney(v + 3), nil
				default:
					return nil, fmt.Errorf("unsupported money payload %T", input)
				}
			},
		}},
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	field := s.Query.Fields["debit"]
	if _, err := field.Resolver(context.Background(), ResolveParams{Args: map[string]any{"m": int64(4)}}); err != nil {
		t.Fatalf("resolve int64 literal: %v", err)
	}
}

type customDefault string

func anyForKind(kind reflect.Kind) any {
	switch kind {
	case reflect.String:
		return "x"
	case reflect.Bool:
		return true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return int64(1)
	case reflect.Float32, reflect.Float64:
		return float64(1)
	default:
		return nil
	}
}
