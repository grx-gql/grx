package exec

import (
	"context"
	"reflect"
	"testing"

	"github.com/grx-gql/grx/core"
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

func TestSDLDirectiveEnumArgumentsUsePublicValues(t *testing.T) {
	s, err := ParseSDL(`type Query @meta(value: ADMIN, nested: {roles: [ADMIN, USER]}) { name: String }`)
	if err != nil {
		t.Fatal(err)
	}
	got := s.Query.Directives[0].Args
	want := map[string]any{"value": "ADMIN", "nested": map[string]any{"roles": []any{"ADMIN", "USER"}}}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("directive values = %#v; want public string values %#v", got, want)
	}
}
