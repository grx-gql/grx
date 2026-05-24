package exec

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/patrickkabwe/grx/core"
	"github.com/patrickkabwe/grx/schema"
)

func TestOperationKindAndSecurityBranches(t *testing.T) {
	cases := []struct {
		query string
		name  string
		want  core.OperationKind
	}{
		{`{ ok }`, "", core.OperationQuery},
		{`query Q { ok } mutation M { ok }`, "M", core.OperationMutation},
		{`subscription S { ok } fragment F on Query { ok }`, "S", core.OperationSubscription},
	}
	for _, tc := range cases {
		got, err := parseOperationKind(tc.query, tc.name)
		if err != nil {
			t.Fatalf("parseOperationKind(%q): %v", tc.query, err)
		}
		if got != tc.want {
			t.Fatalf("kind = %s, want %s", got, tc.want)
		}
	}
	for _, tc := range []struct {
		query string
		name  string
	}{
		{`query A { ok } query B { ok }`, ""},
		{`query A { ok }`, "Missing"},
		{`fragment F on Query { ok }`, ""},
		{`query Missing`, ""},
		{`{ ok `, ""},
	} {
		if _, err := parseOperationKind(tc.query, tc.name); err == nil {
			t.Fatalf("expected operation kind error for %q", tc.query)
		}
	}

	s, err := schema.Build(schema.Config{Query: thunkQuery{}})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	deny := errors.New("denied")
	e := New(s, nil,
		WithMaxAliasCount(1),
		WithMaxRootFieldCount(1),
		WithClientErrorMasking("masked"),
		WithOperationAuthorizer(func(context.Context, OperationContext) error { return deny }),
		WithRateLimiter(func(context.Context, OperationContext) error { return deny }),
		WithTrustedDocuments(map[string]string{"id": `{ plain }`}),
		WithRejectUnknownVariables(),
	)
	if resp := e.Execute(context.Background(), core.Request{Query: `{ plain }`}); len(resp.Errors) != 1 {
		t.Fatalf("expected authorizer/rate error: %#v", resp.Errors)
	}
}

func TestOperationKindParsesFragmentsWithBracketDefaults(t *testing.T) {
	kind, err := parseOperationKind(
		`subscription S ($v: [Int] = [1, 2]) { evt } fragment F on Query { __typename }`,
		"S",
	)
	if err != nil {
		t.Fatalf("parseOperationKind: %v", err)
	}
	if kind != core.OperationSubscription {
		t.Fatalf("kind = %s, want subscription", kind)
	}
	if _, err := parseOperationKind(`mutation M ($id: ID)`, ""); err == nil {
		t.Fatal("expected error for mutation missing selection")
	} else if !strings.Contains(strings.ToLower(err.Error()), "selection") {
		t.Fatalf("expected selection-set error language, got %v", err)
	}
}
