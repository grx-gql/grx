package exec

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/schema"
)

func scopedVariableExecutor(t *testing.T, options ...ExecutorOption) *Executor {
	t.Helper()
	s, err := ParseSDL(`input Values { value:String = "input-default" } type Query { echo(value:String = "field-default"):String nested(input:Values):String values(input:[String]):[String] }`)
	if err != nil {
		t.Fatal(err)
	}
	s.Query.Fields["echo"].Resolver = func(_ context.Context, p schema.ResolveParams) (any, error) { return p.Args["value"], nil }
	s.Query.Fields["nested"].Resolver = func(_ context.Context, p schema.ResolveParams) (any, error) {
		return p.Args["input"].(map[string]any)["value"], nil
	}
	s.Query.Fields["values"].Resolver = func(_ context.Context, p schema.ResolveParams) (any, error) { return p.Args["input"], nil }
	return New(s, nil, options...)
}

func assertScopedVariableResponse(t *testing.T, e *Executor, req core.Request, want string) {
	t.Helper()
	r := e.Execute(context.Background(), req)
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != want {
		t.Fatalf("operation %s variables=%v: got %s want %s", req.OperationName, req.Variables, data, want)
	}
}

func TestOperationDefaultsDoNotCrossOperationScopes(t *testing.T) {
	operations := `query A($v:String="A"){echo(value:$v)} query B($v:String="B"){echo(value:$v)} query Missing($v:String){echo(value:$v)} query Null($v:String=null){echo(value:$v)}`
	for _, cache := range []int{0, 1} {
		e := scopedVariableExecutor(t, WithDocumentCache(cache))
		for _, tc := range []struct{ name, want string }{{"B", "B"}, {"A", "A"}, {"Missing", "field-default"}, {"Null", ""}, {"B", "B"}} {
			want := `{"data":{"echo":"` + tc.want + `"}}`
			if tc.name == "Null" {
				want = `{"data":{"echo":null}}`
			}
			assertScopedVariableResponse(t, e, core.Request{Query: operations, OperationName: tc.name}, want)
		}
	}
}

func TestSharedFragmentsBindSelectedOperationRegardlessOfPosition(t *testing.T) {
	fragment := `fragment Shared on Query { echo(value:$v) nested(input:{value:$v}) values(input:[$v]) }`
	first := `query A($v:String="A"){...Shared}`
	second := `query B($v:String="B"){...Shared}`
	for _, query := range []string{fragment + first + second, first + fragment + second, first + second + fragment} {
		e := scopedVariableExecutor(t, WithDocumentCache(1))
		for _, name := range []string{"B", "A", "B"} {
			want := `{"data":{"echo":"` + name + `","nested":"` + name + `","values":["` + name + `"]}}`
			assertScopedVariableResponse(t, e, core.Request{Query: query, OperationName: name}, want)
		}
	}
}

func TestOperationVariableInputIsImmutableAndNullIsExplicit(t *testing.T) {
	e := scopedVariableExecutor(t, WithDocumentCache(1))
	query := `query A($v:String="A",$input:Values={value:"A"}){echo(value:$v) nested(input:$input)} query B($v:String="B",$input:Values={value:"B"}){echo(value:$v) nested(input:$input)}`
	for _, tc := range []struct {
		vars map[string]any
		want string
	}{
		{map[string]any{}, `{"data":{"echo":"B","nested":"B"}}`},
		{map[string]any{"v": nil, "input": map[string]any{"value": nil}}, `{"data":{"echo":null,"nested":null}}`},
		{map[string]any{"v": "provided", "input": map[string]any{"value": "provided"}}, `{"data":{"echo":"provided","nested":"provided"}}`},
	} {
		before, err := json.Marshal(tc.vars)
		if err != nil {
			t.Fatal(err)
		}
		assertScopedVariableResponse(t, e, core.Request{Query: query, OperationName: "B", Variables: tc.vars}, tc.want)
		after, err := json.Marshal(tc.vars)
		if err != nil {
			t.Fatal(err)
		}
		if string(before) != string(after) {
			t.Fatalf("caller variables changed: before=%s after=%s", before, after)
		}
	}
}

func TestMissingVariableUsesInputFieldDefaultsAndNullListItem(t *testing.T) {
	e := scopedVariableExecutor(t)
	query := `query A($v:String="A"){...Shared} query B($v:String){...Shared} fragment Shared on Query { nested(input:{value:$v}) values(input:[$v]) }`
	assertScopedVariableResponse(t, e, core.Request{Query: query, OperationName: "B"}, `{"data":{"nested":"input-default","values":[null]}}`)
}

func TestSelectedOperationDirectiveDefaultsRemainLocal(t *testing.T) {
	e := scopedVariableExecutor(t, WithDocumentCache(1))
	query := `fragment Shared on Query { echo @include(if:$show) } query A($show:Boolean!=false){...Shared} query B($show:Boolean!=true){...Shared}`
	assertScopedVariableResponse(t, e, core.Request{Query: query, OperationName: "B"}, `{"data":{"echo":"field-default"}}`)
	assertScopedVariableResponse(t, e, core.Request{Query: query, OperationName: "A"}, `{"data":{}}`)
}

func TestSelectedBindingDoesNotChangeParsedBundle(t *testing.T) {
	bundle, err := parseDocumentBundle(`query A($v:String="A"){...Shared} query B($v:String="B"){...Shared} fragment Shared on Query { echo(value:$v) }`, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	original := bundle.fragments["Shared"].Selections[0].Arguments["value"]
	for _, name := range []string{"B", "A"} {
		if _, err := selectOperation(bundle, name); err != nil {
			t.Fatal(err)
		}
	}
	if !reflect.DeepEqual(original, bundle.fragments["Shared"].Selections[0].Arguments["value"]) {
		t.Fatal("binding mutated shared fragment")
	}
	if ref := original.(variableRef); ref.HasValue || ref.Value != nil {
		t.Fatalf("parse eagerly bound variable: %+v", ref)
	}
}

func TestCachedOperationBindingsAreRequestLocal(t *testing.T) {
	e := scopedVariableExecutor(t, WithDocumentCache(1))
	query := `query A($v:String="A"){...Shared} query B($v:String="B"){...Shared} fragment Shared on Query { echo(value:$v) }`
	for index := 0; index < 12; index++ {
		name := "A"
		if index%2 == 1 {
			name = "B"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assertScopedVariableResponse(t, e, core.Request{Query: query, OperationName: name}, `{"data":{"echo":"`+name+`"}}`)
		})
	}
}

func TestVariableFreeSelectionRetainsParsedStorage(t *testing.T) {
	bundle, err := parseDocumentBundle(`{ echo }`, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	doc, err := selectOperation(bundle, "")
	if err != nil {
		t.Fatal(err)
	}
	if &doc.Selections[0] != &bundle.operations[0].Selections[0] {
		t.Fatal("variable-free operation cloned selections")
	}
}

func TestIncrementalDetectionUsesSelectedFragmentBindings(t *testing.T) {
	e := scopedVariableExecutor(t, WithDocumentCache(1))
	query := `fragment Shared on Query { echo @defer(if:$active) } query A($active:Boolean!=false){...Shared} query B($active:Boolean!=true){...Shared}`
	for _, name := range []string{"A", "B", "A"} {
		if got := e.HasIncrementalDirectives(core.Request{Query: query, OperationName: name}); got != (name == "B") {
			t.Fatalf("operation %s incremental=%t", name, got)
		}
	}
}

func TestCustomScalarVariableContainersAreRequestLocal(t *testing.T) {
	s, err := ParseSDL(`scalar JSON type Query { take(input:JSON):String }`)
	if err != nil {
		t.Fatal(err)
	}
	s.Query.Fields["take"].Resolver = func(_ context.Context, p schema.ResolveParams) (any, error) {
		input := p.Args["input"].(map[string]any)
		list := input["values"].([]any)
		original := list[0]
		list[0] = "changed"
		input["added"] = true
		return original, nil
	}
	e := New(s, nil, WithDocumentCache(1))
	query := `query Q($input:JSON={values:["default"]}){take(input:$input)}`
	for range 2 {
		assertScopedVariableResponse(t, e, core.Request{Query: query}, `{"data":{"take":"default"}}`)
	}
	vars := map[string]any{"input": map[string]any{"values": []any{"provided"}}}
	assertScopedVariableResponse(t, e, core.Request{Query: query, Variables: vars}, `{"data":{"take":"provided"}}`)
	if !reflect.DeepEqual(vars, map[string]any{"input": map[string]any{"values": []any{"provided"}}}) {
		t.Fatalf("caller input changed: %v", vars)
	}
}
