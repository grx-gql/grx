package exec

import (
	"context"
	"strings"
	"testing"

	"github.com/patrickkabwe/grx/core"
)

// Parser errors must be grammar-compliant: a clear message plus a 1-based
// source location pointing at the offending token.
func TestParseErrorsCarryLocations(t *testing.T) {
	cases := []struct {
		name      string
		query     string
		wantLine  int
		substring string
	}{
		{"unclosed selection", "{\n  user {\n    id\n", 4, "end of query"},
		{"bad value token", "{ user(id: @) { id } }", 1, "value token"},
		{"missing fragment name", "{ ... }", 1, "fragment name"},
		{"leading zero", "{ f(n: 01) }", 1, "leading zeros"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseDocument(tc.query, nil)
			if err == nil {
				t.Fatalf("expected parse error for %q", tc.query)
			}
			pe, ok := err.(parseError)
			if !ok {
				t.Fatalf("expected parseError, got %T: %v", err, err)
			}
			if !strings.Contains(pe.Error(), tc.substring) {
				t.Fatalf("error %q does not contain %q", pe.Error(), tc.substring)
			}
			locs := pe.GraphQLLocations()
			if len(locs) == 0 {
				t.Fatalf("expected source location on parse error %q", pe.Error())
			}
			if locs[0].Line != tc.wantLine {
				t.Fatalf("error line = %d, want %d (%+v)", locs[0].Line, tc.wantLine, locs[0])
			}
			if locs[0].Column < 1 {
				t.Fatalf("expected 1-based column, got %d", locs[0].Column)
			}
		})
	}
}

// Null in a non-null position is rejected by validation with a located error.
func TestNullValueRejectedInNonNullContext(t *testing.T) {
	e := newValExecutor(t)
	resp := e.Execute(context.Background(), core.Request{Query: `{ search(term: null) }`})
	if len(resp.Errors) == 0 {
		t.Fatal("expected error for null in non-null position")
	}
	if !hasErrContaining(resp.Errors, "found null") {
		t.Fatalf("expected null-in-non-null error, got %#v", resp.Errors)
	}
	if len(resp.Errors[0].Locations) == 0 {
		t.Fatalf("expected a source location on the error: %#v", resp.Errors[0])
	}
}
