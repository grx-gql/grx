package schema

import (
	"errors"
	"testing"
)

type scalarInputToken string
type scalarInputQuery struct{}

func (scalarInputQuery) Token() scalarInputToken { return "ok" }

func TestScalarParsePreservesRegisteredGoTypeAndErrors(t *testing.T) {
	sentinel := errors.New("bad input")
	for _, tc := range []struct {
		name       string
		value      any
		parseError error
		want       any
		wantError  bool
	}{
		{"typed", scalarInputToken("typed"), nil, scalarInputToken("typed"), false},
		{"convertible", "raw", nil, scalarInputToken("raw"), false},
		{"null", nil, nil, nil, false},
		{"error", nil, sentinel, nil, true},
		{"incompatible", make(chan int), nil, nil, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := Build(Config{Query: scalarInputQuery{}, Scalars: []ScalarConfig{{Type: scalarInputToken(""), Name: "Token", Parse: func(any) (any, error) { return tc.value, tc.parseError }}}})
			if err != nil {
				t.Fatal(err)
			}
			got, err := s.Types["Token"].(*Scalar).Parse("input")
			if (err != nil) != tc.wantError || (!tc.wantError && got != tc.want) {
				t.Fatalf("got%#v error=%v", got, err)
			}
			if tc.parseError != nil && !errors.Is(err, sentinel) {
				t.Fatalf("parser error lost: %v", err)
			}
		})
	}
	scalar := &Scalar{TypeName: "String"}
	if got, err := scalar.Parse("unchanged"); err != nil || got != "unchanged" {
		t.Fatalf("unregistered scalar parse=%v err=%v", got, err)
	}
}
