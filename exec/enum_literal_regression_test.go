package exec

import (
	"context"
	"github.com/grx-gql/grx/core"
	"testing"
)

func TestRejectStringEnumLiteralKindMismatch(t *testing.T) {
	e := newValExecutor(t)
	for _, query := range []string{`{search(term:"ok",role:"ADMIN")}`, `{search(term:ADMIN)}`} {
		if errs := valErrors(t, e, query); len(errs) == 0 {
			t.Errorf("wrong literal kind accepted: %s", query)
		}
	}
}

func TestEnumVariableAndDefaultLiteralKinds(t *testing.T) {
	e := newValExecutor(t)
	for _, tc := range []struct {
		query     string
		variables map[string]any
		invalid   bool
	}{
		{`query Q($role:valRole){search(term:"ok",role:$role)}`, map[string]any{"role": "ADMIN"}, false},
		{`query Q($role:valRole=ADMIN){search(term:"ok",role:$role)}`, nil, false},
		{`query Q($role:valRole="ADMIN"){search(term:"ok",role:$role)}`, nil, true},
		{`query Q($term:String=ADMIN){search(term:$term)}`, nil, true},
		{`query Q($role:[valRole!]=[ADMIN]){__typename @include(if:true) search(term:"ok",role:$role)}`, nil, true},
	} {
		response := e.Execute(context.Background(), core.Request{Query: tc.query, Variables: tc.variables})
		if (len(response.Errors) > 0) != tc.invalid {
			t.Errorf("%s: errors=%v", tc.query, response.Errors)
		}
	}
}
