package server

import (
	"net/http"
	"testing"
)

func TestServeHTTPExecutesQuery(t *testing.T) {
	server := newTestServer(t)
	response := executeGraphQL(t, server, `{"query":"query GetUser($id: String!) { __typename user(id: $id) { __typename id name email } }","variables":{"id":"user_42"}}`)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	body := graphQLResponseBody(t, response)
	assertNoErrors(t, body)

	data := nestedMap(t, body, "data")
	if data["__typename"] != "Query" {
		t.Fatalf("expected root typename Query, got %#v", data["__typename"])
	}

	user := nestedMap(t, data, "user")
	assertExactKeys(t, user, "__typename", "id", "name", "email")
	if user["__typename"] != "User" {
		t.Fatalf("expected user typename User, got %#v", user["__typename"])
	}
	if user["id"] != "user_42" {
		t.Fatalf("expected id user_42, got %#v", user["id"])
	}
	if user["name"] != "Ada Lovelace" {
		t.Fatalf("expected name Ada Lovelace, got %#v", user["name"])
	}
	if user["email"] != "ada@example.com" {
		t.Fatalf("expected email ada@example.com, got %#v", user["email"])
	}
}

func TestServeHTTPReturnsQueryFieldErrorsWithPartialData(t *testing.T) {
	server := newTestServer(t)
	response := executeGraphQL(t, server, `{"query":"query GetUser($id: String!) { __typename missing user(id: $id) { id name } }","variables":{"id":"user_42"}}`)

	if response.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, response.Code)
	}

	body := graphQLResponseBody(t, response)
	data := nestedMap(t, body, "data")
	if data["__typename"] != "Query" {
		t.Fatalf("expected root typename Query, got %#v", data["__typename"])
	}
	user := nestedMap(t, data, "user")
	if user["id"] != "user_42" {
		t.Fatalf("expected id user_42, got %#v", user["id"])
	}

	errors := graphQLErrors(t, body)
	if len(errors) != 1 {
		t.Fatalf("expected one error, got %#v", errors)
	}
	errorValue := graphQLError(t, errors, 0)
	if errorValue["message"] != `unknown field "missing" on Query` {
		t.Fatalf("unexpected error message: %#v", errorValue["message"])
	}
	path, ok := errorValue["path"].([]any)
	if !ok || len(path) != 1 || path[0] != "missing" {
		t.Fatalf("expected missing error path, got %#v", errorValue["path"])
	}
}
