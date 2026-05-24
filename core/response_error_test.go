package core

import (
	"errors"
	"testing"
)

type locationErr struct {
	msg string
	loc Location
}

func (e locationErr) Error() string { return e.msg }

func (e locationErr) GraphQLLocations() []Location { return []Location{e.loc} }

func TestNewRequestErrorIncludesClassification(t *testing.T) {
	err := NewRequestError(errors.New("bad request"))
	if err.Message != "bad request" {
		t.Fatalf("message = %q", err.Message)
	}
	if err.Extensions["classification"] != "request" {
		t.Fatalf("classification = %#v", err.Extensions["classification"])
	}
}

func TestNewRequestErrorIncludesLocations(t *testing.T) {
	err := NewRequestError(locationErr{
		msg: "syntax error",
		loc: Location{Line: 2, Column: 5},
	})
	if len(err.Locations) != 1 || err.Locations[0].Line != 2 || err.Locations[0].Column != 5 {
		t.Fatalf("locations = %#v", err.Locations)
	}
}

func TestNewValidationErrorIncludesCode(t *testing.T) {
	err := NewValidationError(locationErr{msg: "bad", loc: Location{Line: 1, Column: 1}})
	if err.Extensions["code"] != ErrorCodeValidationFailed {
		t.Fatalf("code = %#v", err.Extensions["code"])
	}
}

func TestNewFieldErrorIncludesClassificationAndLocations(t *testing.T) {
	err := NewFieldError("resolver failed", []any{"user", "email"}, Location{Line: 3, Column: 7})
	if err.Extensions["classification"] != "field" {
		t.Fatalf("extensions = %#v", err.Extensions)
	}
	if len(err.Path) != 2 || err.Path[0] != "user" || err.Path[1] != "email" {
		t.Fatalf("path = %#v", err.Path)
	}
	if len(err.Locations) != 1 || err.Locations[0].Line != 3 {
		t.Fatalf("locations = %#v", err.Locations)
	}
}

func TestCoreErrorHelpers(t *testing.T) {
	res := AttachRequestIDExtension(Response{}, "req_1")
	if res.Extensions["requestId"] != "req_1" {
		t.Fatalf("extensions = %#v", res.Extensions)
	}
	unchanged := AttachRequestIDExtension(Response{}, "")
	if unchanged.Extensions != nil {
		t.Fatalf("unexpected extensions = %#v", unchanged.Extensions)
	}

	err := NewValidationError(coverageLocationErr{})
	if len(err.Locations) != 1 || err.Extensions["code"] != ErrorCodeValidationFailed {
		t.Fatalf("validation error = %#v", err)
	}
}

type coverageLocationErr struct{}

func (coverageLocationErr) Error() string { return "bad location" }

func (coverageLocationErr) GraphQLLocations() []Location {
	return []Location{{Line: 2, Column: 3}}
}
