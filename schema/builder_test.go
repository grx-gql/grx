package schema

import (
	"context"
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
