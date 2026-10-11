package exec

import "testing"

func TestRejectFieldConflictsThroughFragments(t *testing.T) {
	e := newValExecutor(t)
	for _, query := range []string{
		`{ result: search(term:"a") ...F } fragment F on Query { result:search(term:"b") }`,
		`{ result: search(term:"a") ... on Query { result:search(term:"b") } }`,
	} {
		if errs := valErrors(t, e, query); len(errs) == 0 {
			t.Errorf("conflict accepted: %s", query)
		}
	}
}

func TestMergeConflictResponseShapesAndNestedSelections(t *testing.T) {
	s, err := ParseSDL(`type User { id:ID name:String child:User } type Other { id:ID title:String child:Other } union Result=User|Other type Query { user:User result:Result value(x:Float, values:[Int], object:Filter):String } input Filter { x:Int }`)
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		query   string
		invalid bool
	}{
		{`{user{x:id} user{x:name}}`, true},
		{`{user{id} user{name}}`, false},
		{`{user{...F} user{...G}} fragment F on User {child{...F}} fragment G on User {child{...G}}`, true},
		{`{result{...on User{x:name} ...on Other{x:title}}}`, false},
		{`{result{...on User{x:name} ...on Other{x:id}}}`, true},
		{`{result{...on User{child{x:name}} ...on Other{child{x:title}}}}`, false},
		{`{value(x:1) value(x:1.0)}`, true},
		{`{value(values:[1,2]) ...F} fragment F on Query {value(values:[1,2])}`, false},
		{`{value(object:{x:1}) value(object:{x:2})}`, true},
	} {
		bundle, err := parseDocumentBundle(tc.query, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		op, err := selectOperation(bundle, "")
		if err != nil {
			t.Fatal(err)
		}
		if errs := ValidateDocument(s, bundle, op); (len(errs) > 0) != tc.invalid {
			t.Errorf("%s: errors=%v", tc.query, errs)
		}
	}
}
