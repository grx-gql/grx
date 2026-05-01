package schema

import (
	"context"
	"testing"
)

type buildTestUser struct {
	ID string `gql:"id,nonNull"`
}

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
