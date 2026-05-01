package exec

import (
	"context"
	"testing"

	"github.com/patrickkabwe/grx/core"
	"github.com/patrickkabwe/grx/schema"
)

type testUser struct {
	ID   string `gql:"id,nonNull"`
	Name string `gql:"name,nonNull"`
}

type testQuery struct{}

type testMutation struct{}

type testUserCreateInput struct {
	Name string `gql:"name,nonNull"`
}

type testUserCreateArgs struct {
	Input testUserCreateInput `gql:"input,nonNull"`
}

type testUserCreatePayload struct {
	User *testUser `gql:"user,nonNull"`
}

func (testQuery) User(ctx context.Context, args struct {
	ID string `gql:"id,nonNull"`
}) (*testUser, error) {
	return &testUser{ID: args.ID, Name: "Ada"}, nil
}

func (testMutation) CreateUser(ctx context.Context, args testUserCreateArgs) (*testUserCreatePayload, error) {
	return &testUserCreatePayload{User: &testUser{ID: "user_1", Name: args.Input.Name}}, nil
}

func TestExecutorResolvesNestedSelection(t *testing.T) {
	schemaValue, err := schema.Build(schema.Config{Query: testQuery{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	executor := New(schemaValue, nil)
	response := executor.Execute(context.Background(), core.Request{
		Query:     `query GetUser($id: String!) { user(id: $id) { id name } }`,
		Variables: map[string]any{"id": "1"},
	})

	if len(response.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", response.Errors)
	}

	data, ok := response.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data, got %T", response.Data)
	}

	user, ok := data["user"].(map[string]any)
	if !ok {
		t.Fatalf("expected user map, got %T", data["user"])
	}
	if user["id"] != "1" {
		t.Fatalf("expected id 1, got %#v", user["id"])
	}
	if user["name"] != "Ada" {
		t.Fatalf("expected name Ada, got %#v", user["name"])
	}
}

func TestExecutorBindsNestedInputObject(t *testing.T) {
	schemaValue, err := schema.Build(schema.Config{Query: testQuery{}, Mutation: testMutation{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	executor := New(schemaValue, nil)
	response := executor.Execute(context.Background(), core.Request{
		Query: `mutation CreateUser($input: UserCreateInput!) {
			createUser(input: $input) {
				user {
					id
					name
				}
			}
		}`,
		Variables: map[string]any{
			"input": map[string]any{"name": "Grace"},
		},
	})

	if len(response.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", response.Errors)
	}

	data, ok := response.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data, got %T", response.Data)
	}

	payload, ok := data["createUser"].(map[string]any)
	if !ok {
		t.Fatalf("expected payload map, got %T", data["createUser"])
	}

	user, ok := payload["user"].(map[string]any)
	if !ok {
		t.Fatalf("expected user map, got %T", payload["user"])
	}
	if user["name"] != "Grace" {
		t.Fatalf("expected name Grace, got %#v", user["name"])
	}
}

func TestExecutorBindsInlineInputObjectLiteral(t *testing.T) {
	schemaValue, err := schema.Build(schema.Config{Query: testQuery{}, Mutation: testMutation{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	executor := New(schemaValue, nil)
	response := executor.Execute(context.Background(), core.Request{
		Query: `mutation MyMutation {
			__typename
			createUser(input: {name: "test"}) {
				user {
					id
					name
				}
			}
		}`,
	})

	if len(response.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", response.Errors)
	}

	data, ok := response.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data, got %T", response.Data)
	}
	payload, ok := data["createUser"].(map[string]any)
	if !ok {
		t.Fatalf("expected payload map, got %T", data["createUser"])
	}
	user, ok := payload["user"].(map[string]any)
	if !ok {
		t.Fatalf("expected user map, got %T", payload["user"])
	}
	if user["name"] != "test" {
		t.Fatalf("expected name test, got %#v", user["name"])
	}
}

func TestExecutorHandlesSchemaIntrospectionQueryWithFragments(t *testing.T) {
	schemaValue, err := schema.Build(schema.Config{Query: testQuery{}, Mutation: testMutation{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	executor := New(schemaValue, nil)
	response := executor.Execute(context.Background(), core.Request{
		Query: `query IntrospectionQuery {
			__schema {
				queryType { name }
				mutationType { name }
				types { ...FullType }
				directives { name }
			}
		}

		fragment FullType on __Type {
			kind
			name
			fields(includeDeprecated: true) {
				name
				args { name }
				type { kind name ofType { kind name } }
			}
		}`,
	})

	if len(response.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", response.Errors)
	}

	data, ok := response.Data.(map[string]any)
	if !ok {
		t.Fatalf("expected map data, got %T", response.Data)
	}

	schemaData, ok := data["__schema"].(map[string]any)
	if !ok {
		t.Fatalf("expected __schema map, got %T", data["__schema"])
	}

	queryType, ok := schemaData["queryType"].(map[string]any)
	if !ok {
		t.Fatalf("expected queryType map, got %T", schemaData["queryType"])
	}
	if queryType["name"] != "Query" {
		t.Fatalf("expected Query root, got %#v", queryType["name"])
	}

	types, ok := schemaData["types"].([]any)
	if !ok {
		t.Fatalf("expected types list, got %T", schemaData["types"])
	}

	userField := introspectionField(t, types, "Query", "user")
	args, ok := userField["args"].([]any)
	if !ok {
		t.Fatalf("expected user args list, got %T", userField["args"])
	}
	if len(args) != 1 {
		t.Fatalf("expected one user arg, got %#v", args)
	}

	idArg, ok := args[0].(map[string]any)
	if !ok {
		t.Fatalf("expected id arg map, got %T", args[0])
	}
	if idArg["name"] != "id" {
		t.Fatalf("expected id arg, got %#v", idArg["name"])
	}

	createUserField := introspectionField(t, types, "Mutation", "createUser")
	createUserArgs, ok := createUserField["args"].([]any)
	if !ok {
		t.Fatalf("expected createUser args list, got %T", createUserField["args"])
	}
	if len(createUserArgs) != 1 {
		t.Fatalf("expected one createUser arg, got %#v", createUserArgs)
	}
}

func introspectionField(t *testing.T, types []any, typeName string, fieldName string) map[string]any {
	t.Helper()

	for _, rawType := range types {
		typeValue, ok := rawType.(map[string]any)
		if !ok || typeValue["name"] != typeName {
			continue
		}

		fields, ok := typeValue["fields"].([]any)
		if !ok {
			t.Fatalf("expected fields list for %s, got %T", typeName, typeValue["fields"])
		}
		for _, rawField := range fields {
			field, ok := rawField.(map[string]any)
			if ok && field["name"] == fieldName {
				return field
			}
		}
	}

	t.Fatalf("expected field %s.%s", typeName, fieldName)
	return nil
}
