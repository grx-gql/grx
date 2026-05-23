package server

import (
	"testing"

	grxclient "github.com/patrickkabwe/grx/pkg/client"
)

func TestServeHTTPExecutesQuery(t *testing.T) {
	h := newTestHarness(t)
	body := responseToMap(t, execGraphQL(t, h, &grxclient.Request{
		Query: "query GetUser($id: String!) { __typename user(id: $id) { __typename id name email } }",
		Variables: map[string]any{
			"id": "user_42",
		},
	}))
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

func TestServeHTTPReturnsQueryValidationErrorForUnknownField(t *testing.T) {
	h := newTestHarness(t)
	body := responseToMap(t, execGraphQL(t, h, &grxclient.Request{
		Query: "query GetUser($id: String!) { __typename missing user(id: $id) { id name } }",
		Variables: map[string]any{
			"id": "user_42",
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
	if errorValue["message"] != `Cannot query field "missing" on type "Query".` {
		t.Fatalf("unexpected error message: %#v", errorValue["message"])
	}
	code, _ := errorValue["extensions"].(map[string]any)["code"].(string)
	if code != "GRAPHQL_VALIDATION_FAILED" {
		t.Fatalf("expected validation error code, got %#v", errorValue["extensions"])
	}
	assertErrorLocations(t, errorValue, 1, 42)
}

func TestServeHTTPReturnsExampleFieldErrorFromBasicSchema(t *testing.T) {
	h := newTestHarness(t)
	body := responseToMap(t, execGraphQL(t, h, &grxclient.Request{
		Query: "query ExampleError { errorExample }",
	}))
	dataRaw, ok := body["data"]
	if !ok {
		t.Fatalf("expected data key with null value, got %#v", body)
	}
	if dataRaw != nil {
		t.Fatalf("expected top-level data null, got %#v", dataRaw)
	}

	errors := graphQLErrors(t, body)
	if len(errors) != 1 {
		t.Fatalf("expected one error, got %#v", errors)
	}

	errorValue := graphQLError(t, errors, 0)
	if errorValue["message"] != "example error from basic server" {
		t.Fatalf("unexpected error message: %#v", errorValue["message"])
	}
	assertErrorClassification(t, errorValue, "field")
	assertErrorLocations(t, errorValue, 1, 22)
	path, ok := errorValue["path"].([]any)
	if !ok || len(path) != 1 || path[0] != "errorExample" {
		t.Fatalf("expected errorExample path, got %#v", errorValue["path"])
	}
}
