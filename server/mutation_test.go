package server

import (
	"testing"

	grxclient "github.com/patrickkabwe/grx/pkg/client"
)

func TestServeHTTPExecutesMutation(t *testing.T) {
	h := newTestHarness(t)
	body := responseToMap(t, execGraphQL(t, h, &grxclient.Request{
		Query: "mutation CreateUser($input: UserCreateInput!) { __typename createUser(input: $input) { __typename user { __typename id name email } } }",
		Variables: map[string]any{
			"input": map[string]any{
				"name":  "Grace Hopper",
				"email": "grace@example.com",
			},
		},
	}))
	assertNoErrors(t, body)

	data := nestedMap(t, body, "data")
	if data["__typename"] != "Mutation" {
		t.Fatalf("expected root typename Mutation, got %#v", data["__typename"])
	}

	createUser := nestedMap(t, data, "createUser")
	assertExactKeys(t, createUser, "__typename", "user")
	if createUser["__typename"] != "UserCreatePayload" {
		t.Fatalf("expected payload typename UserCreatePayload, got %#v", createUser["__typename"])
	}

	user := nestedMap(t, createUser, "user")
	assertExactKeys(t, user, "__typename", "id", "name", "email")
	if user["__typename"] != "User" {
		t.Fatalf("expected user typename User, got %#v", user["__typename"])
	}
	if user["id"] != "user_1" {
		t.Fatalf("expected id user_1, got %#v", user["id"])
	}
	if user["name"] != "Grace Hopper" {
		t.Fatalf("expected name Grace Hopper, got %#v", user["name"])
	}
	if user["email"] != "grace@example.com" {
		t.Fatalf("expected email grace@example.com, got %#v", user["email"])
	}
}

func TestServeHTTPExecutesMutationWithInlineInputObject(t *testing.T) {
	h := newTestHarness(t)
	body := responseToMap(t, execGraphQL(t, h, &grxclient.Request{
		Query: `mutation MyMutation { __typename createUser(input: {email: "test@gmail.com", name: "test"}) { user { email } } }`,
	}))
	assertNoErrors(t, body)

	data := nestedMap(t, body, "data")
	if data["__typename"] != "Mutation" {
		t.Fatalf("expected root typename Mutation, got %#v", data["__typename"])
	}
	createUser := nestedMap(t, data, "createUser")
	user := nestedMap(t, createUser, "user")
	if user["email"] != "test@gmail.com" {
		t.Fatalf("expected email test@gmail.com, got %#v", user["email"])
	}
}

func TestServeHTTPReturnsMutationValidationErrorForUnknownField(t *testing.T) {
	h := newTestHarness(t)
	body := responseToMap(t, execGraphQL(t, h, &grxclient.Request{
		Query: "mutation CreateUser($input: UserCreateInput!) { __typename missing createUser(input: $input) { user { id name } } }",
		Variables: map[string]any{
			"input": map[string]any{
				"name": "Grace Hopper",
			},
		},
	}))
	if _, exists := body["data"]; exists {
		t.Fatalf("expected validation error to omit data, got %#v", body["data"])
	}

	errors := graphQLErrors(t, body)
	if len(errors) != 1 {
		t.Fatalf("expected one error, got %#v", errors)
	}
	errorValue := graphQLError(t, errors, 0)
	if errorValue["message"] != `Cannot query field "missing" on type "Mutation".` {
		t.Fatalf("unexpected error message: %#v", errorValue["message"])
	}
	assertErrorLocations(t, errorValue, 1, 60)
}
