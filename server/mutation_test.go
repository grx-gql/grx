package server

import (
	"net/http"
	"testing"
)

func TestServeHTTPExecutesMutation(t *testing.T) {
	server := newTestServer(t)
	response := executeGraphQL(t, server, `{"query":"mutation CreateUser($input: UserCreateInput!) { __typename createUser(input: $input) { __typename user { __typename id name email } } }","variables":{"input":{"name":"Grace Hopper","email":"grace@example.com"}}}`)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	body := graphQLResponseBody(t, response)
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
	server := newTestServer(t)
	response := executeGraphQL(t, server, `{"query":"mutation MyMutation { __typename createUser(input: {email: \"test@gmail.com\", name: \"test\"}) { user { email } } }"}`)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	body := graphQLResponseBody(t, response)
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

func TestServeHTTPReturnsMutationFieldErrorsWithPartialData(t *testing.T) {
	server := newTestServer(t)
	response := executeGraphQL(t, server, `{"query":"mutation CreateUser($input: UserCreateInput!) { __typename missing createUser(input: $input) { user { id name } } }","variables":{"input":{"name":"Grace Hopper"}}}`)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	body := graphQLResponseBody(t, response)
	data := nestedMap(t, body, "data")
	if data["__typename"] != "Mutation" {
		t.Fatalf("expected root typename Mutation, got %#v", data["__typename"])
	}
	createUser := nestedMap(t, data, "createUser")
	user := nestedMap(t, createUser, "user")
	if user["name"] != "Grace Hopper" {
		t.Fatalf("expected name Grace Hopper, got %#v", user["name"])
	}

	errors := graphQLErrors(t, body)
	if len(errors) != 1 {
		t.Fatalf("expected one error, got %#v", errors)
	}
	errorValue := graphQLError(t, errors, 0)
	if errorValue["message"] != `unknown field "missing" on Mutation` {
		t.Fatalf("unexpected error message: %#v", errorValue["message"])
	}
	path, ok := errorValue["path"].([]any)
	if !ok || len(path) != 1 || path[0] != "missing" {
		t.Fatalf("expected missing error path, got %#v", errorValue["path"])
	}
}
