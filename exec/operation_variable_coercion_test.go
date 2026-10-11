package exec

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/schema"
)

func TestOperationVariablesFailBeforeResolverSideEffects(t *testing.T) {
	for _, tc := range []struct {
		name, definition string
		vars             map[string]any
	}{
		{"required missing", "$v:String!", nil},
		{"required null", "$v:String!", map[string]any{"v": nil}},
		{"wrong scalar", "$v:String", map[string]any{"v": 123}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := scopedVariableExecutor(t)
			calls := 0
			e.Schema.Query.Fields["echo"].Resolver = func(_ context.Context, p schema.ResolveParams) (any, error) { calls++; return p.Args["value"], nil }
			query := `query Q(` + tc.definition + `){ first:echo last:echo(value:$v) }`
			for _, incremental := range []bool{false, true} {
				var r core.Response
				if incremental {
					r, _ = e.ExecuteIncremental(context.Background(), core.Request{Query: query, Variables: tc.vars})
				} else {
					r = e.Execute(context.Background(), core.Request{Query: query, Variables: tc.vars})
				}
				if calls != 0 || len(r.Errors) != 1 || r.Data != nil || r.DataNull || len(r.Errors[0].Path) != 0 || r.Errors[0].Extensions["classification"] != "request" {
					t.Fatalf("variable request must fail before execution calls=%d response=%+v", calls, r)
				}
			}
		})
	}
}

func TestOperationVariableValidationPrecedesSubscriptionSource(t *testing.T) {
	s, err := ParseSDL(`type Query { ok:String } type Subscription { changed(value:String):String }`)
	if err != nil {
		t.Fatal(err)
	}
	calls := 0
	s.Subscription.Fields["changed"].Resolver = func(context.Context, schema.ResolveParams) (any, error) {
		calls++
		stream := make(chan string)
		close(stream)
		return stream, nil
	}
	_, err = New(s, nil).Subscribe(context.Background(), core.Request{Query: `subscription Q($v:String!){changed(value:$v)}`})
	if err == nil || calls != 0 {
		t.Fatalf("source ran with invalid variables: calls=%d error=%v", calls, err)
	}
}

func TestUnselectedOperationDoesNotRequireRuntimeVariables(t *testing.T) {
	e := scopedVariableExecutor(t)
	assertScopedVariableResponse(t, e, core.Request{Query: `query A($v:String!){echo(value:$v)} query B {echo}`, OperationName: "B"}, `{"data":{"echo":"field-default"}}`)
}

type operationColor string

func TestOperationVariableNestedInputCoercion(t *testing.T) {
	s, err := ParseSDL(`enum Color { RED BLUE } input Input { count:Int! values:[Color!] label:String="default" } type Query { take(value:Input):String }`)
	if err != nil {
		t.Fatal(err)
	}
	registered, err := schema.Build(schema.Config{Query: introQuery{}, Enums: []schema.EnumConfig{{Name: "Color", Type: operationColor(""), Values: []schema.EnumValueConfig{{Name: "RED", Value: "RED"}, {Name: "BLUE", Value: "BLUE"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	s.Types["Color"] = registered.Types["Color"]
	s.Types["Input"].(*schema.InputObject).Fields["values"].Type = &schema.List{OfType: &schema.NonNull{OfType: s.Types["Color"]}}
	calls := 0
	s.Query.Fields["take"].Resolver = func(_ context.Context, p schema.ResolveParams) (any, error) {
		calls++
		input := p.Args["value"].(map[string]any)
		if !reflect.DeepEqual(input, map[string]any{"count": 2, "values": []any{"RED"}, "label": "default"}) {
			return nil, errors.New("incorrect coerced input")
		}
		return "ok", nil
	}
	e := New(s, nil)
	for _, input := range []map[string]any{{"count": float64(2), "values": "RED"}, {"count": 2, "values": []any{"RED"}}} {
		assertScopedVariableResponse(t, e, core.Request{Query: `query Q($v:Input){take(value:$v)}`, Variables: map[string]any{"v": input}}, `{"data":{"take":"ok"}}`)
		if _, changed := input["label"]; changed {
			t.Fatal("input object mutated")
		}
	}
	assertScopedVariableResponse(t, e, core.Request{Query: `query Q($v:Input={count:2,values:[RED]}){take(value:$v)}`}, `{"data":{"take":"ok"}}`)
	before := calls
	for _, input := range []map[string]any{{}, {"count": 2, "extra": true}, {"count": 2, "values": []any{nil}}, {"count": 2, "values": "UNKNOWN"}} {
		r := e.Execute(context.Background(), core.Request{Query: `query Q($v:Input){take(value:$v)}`, Variables: map[string]any{"v": input}})
		if len(r.Errors) == 0 || r.Data != nil || r.DataNull || calls != before {
			t.Fatalf("invalid nested input reached execution: %+v", r)
		}
	}
}

type variableScalarArgs struct{ Value string }
type variableScalarQuery struct{ calls *int }

func (q variableScalarQuery) Take(args variableScalarArgs) string { (*q.calls)++; return args.Value }
func (q variableScalarQuery) Side() int                           { (*q.calls)++; return 1 }

func TestCustomVariableScalarParsesOnceBeforeExecution(t *testing.T) {
	calls, parses := 0, 0
	s, err := schema.Build(schema.Config{Query: variableScalarQuery{calls: &calls}, Scalars: []schema.ScalarConfig{{Name: "Token", Type: "", Parse: func(raw any) (any, error) {
		parses++
		text, ok := raw.(string)
		if !ok || text == "invalid" {
			return nil, errors.New("invalid token")
		}
		return strings.ToUpper(text), nil
	}}}})
	if err != nil {
		t.Fatal(err)
	}
	e := New(s, nil)
	assertScopedVariableResponse(t, e, core.Request{Query: `query Q($v:Token!){a:take(value:$v) b:take(value:$v)}`, Variables: map[string]any{"v": "ok"}}, `{"data":{"a":"OK","b":"OK"}}`)
	if parses != 1 || calls != 2 {
		t.Fatalf("parses=%d calls=%d", parses, calls)
	}
	r := e.Execute(context.Background(), core.Request{Query: `query Q($v:Token!){side take(value:$v)}`, Variables: map[string]any{"v": "invalid"}})
	if parses != 2 || calls != 2 || r.Data != nil || len(r.Errors) != 1 || r.DataNull {
		t.Fatalf("invalid custom scalar reached resolvers parses=%d calls=%d response=%+v", parses, calls, r)
	}
}

func TestCoercedNullStillRejectsNonNullArgument(t *testing.T) {
	e := scopedVariableExecutor(t)
	e.Schema.Query.Fields["echo"].Args[0].Type = &schema.NonNull{OfType: e.Schema.Types["String"]}
	calls := 0
	e.Schema.Query.Fields["echo"].Resolver = func(context.Context, schema.ResolveParams) (any, error) { calls++; return "wrong", nil }
	r := e.Execute(context.Background(), core.Request{Query: `query Q($v:String){echo(value:$v)}`, Variables: map[string]any{"v": nil}})
	if calls != 0 || len(r.Errors) != 1 || len(r.Errors[0].Path) == 0 {
		t.Fatalf("coerced null bypassed argument non-null: calls=%d response=%+v", calls, r)
	}
}

type operationToken string
type operationTokenInput struct {
	Value operationToken `gql:"value,default=base"`
}
type operationTokenQuery struct{}

func (operationTokenQuery) Take(args struct{ Value operationToken }) string {
	return string(args.Value)
}
func (operationTokenQuery) Nested(args struct{ Input operationTokenInput }) string {
	return string(args.Input.Value)
}

func TestCustomVariableScalarNormalizesOnceAndPreservesLiteralDefaults(t *testing.T) {
	parses := 0
	s, err := schema.Build(schema.Config{Query: operationTokenQuery{}, Scalars: []schema.ScalarConfig{{Name: "Token", Type: operationToken(""), Parse: func(raw any) (any, error) {
		parses++
		text, ok := raw.(string)
		if !ok {
			return nil, errors.New("expected raw string")
		}
		return text + "!", nil
	}}}})
	if err != nil {
		t.Fatal(err)
	}
	parses = 0
	e := New(s, nil, WithDocumentCache(1))
	assertScopedVariableResponse(t, e, core.Request{Query: `query Q($v:Token!){a:take(value:$v) b:take(value:$v)}`, Variables: map[string]any{"v": "provided"}}, `{"data":{"a":"provided!","b":"provided!"}}`)
	if parses != 1 {
		t.Fatalf("variable parsed %d times", parses)
	}
	assertScopedVariableResponse(t, e, core.Request{Query: `{take(value:"literal")}`}, `{"data":{"take":"literal!"}}`)
	if parses != 2 {
		t.Fatalf("literal parsed %d total times", parses)
	}
	assertScopedVariableResponse(t, e, core.Request{Query: `query Q($v:operationTokenInput!){nested(input:$v)}`, Variables: map[string]any{"v": map[string]any{}}}, `{"data":{"nested":"base!"}}`)
	if parses != 2 {
		t.Fatalf("schema default reparsed: %d", parses)
	}
	assertScopedVariableResponse(t, e, core.Request{Query: `query Q($v:operationTokenInput!={value:"operation"}){nested(input:$v)}`}, `{"data":{"nested":"operation!"}}`)
	if parses != 3 {
		t.Fatalf("operation default parsed %d total times", parses)
	}
}

type operationJSON map[string]any
type operationJSONQuery struct{}

func (operationJSONQuery) Take(args struct{ Value operationJSON }) string {
	return args.Value["value"].(string)
}

func TestCustomVariableParserCannotMutateCallerOrCachedDefault(t *testing.T) {
	s, err := schema.Build(schema.Config{Query: operationJSONQuery{}, Scalars: []schema.ScalarConfig{{Name: "JSON", Type: operationJSON{}, Parse: func(raw any) (any, error) {
		input, ok := raw.(map[string]any)
		if !ok {
			return nil, errors.New("expected object")
		}
		value := input["value"].(string)
		input["value"] = value + "!"
		return input, nil
	}}}})
	if err != nil {
		t.Fatal(err)
	}
	e := New(s, nil, WithDocumentCache(1))
	query := `query Q($v:JSON={value:"default"}){take(value:$v)}`
	for range 2 {
		assertScopedVariableResponse(t, e, core.Request{Query: query}, `{"data":{"take":"default!"}}`)
	}
	vars := map[string]any{"v": map[string]any{"value": "provided"}}
	assertScopedVariableResponse(t, e, core.Request{Query: query, Variables: vars}, `{"data":{"take":"provided!"}}`)
	if !reflect.DeepEqual(vars, map[string]any{"v": map[string]any{"value": "provided"}}) {
		t.Fatalf("parser mutated caller input: %v", vars)
	}
}

type operationTokenOnlyQuery struct{}

func (operationTokenOnlyQuery) Take(args struct{ Value operationToken }) string {
	return string(args.Value)
}

func TestVariableScalarParseFailureAndNullResult(t *testing.T) {
	for _, result := range []any{nil, make(chan int)} {
		s, err := schema.Build(schema.Config{Query: operationTokenOnlyQuery{}, Scalars: []schema.ScalarConfig{{Name: "Token", Type: operationToken(""), Parse: func(any) (any, error) { return result, nil }}}})
		if err != nil {
			t.Fatal(err)
		}
		r := New(s, nil).Execute(context.Background(), core.Request{Query: `query Q($v:Token!){take(value:$v)}`, Variables: map[string]any{"v": "raw"}})
		if len(r.Errors) != 1 || r.Data != nil || r.DataNull {
			t.Fatalf("invalid scalar parse result reached resolver: %+v", r)
		}
	}
}

func TestOperationEnumValueIsNotParsedAgainAtArgument(t *testing.T) {
	s, err := schema.Build(schema.Config{Query: introQuery{}, Enums: []schema.EnumConfig{{Name: "Color", Type: operationColor(""), Values: []schema.EnumValueConfig{{Name: "FIRST", Value: "SECOND"}, {Name: "SECOND", Value: "THIRD"}}}}})
	if err != nil {
		t.Fatal(err)
	}
	s.Query.Fields["echo"] = &schema.Field{Name: "echo", Type: s.Types["String"], Args: []schema.InputValue{{Name: "value", Type: s.Types["Color"]}}, Resolver: func(_ context.Context, p schema.ResolveParams) (any, error) { return p.Args["value"], nil }}
	e := New(s, nil)
	assertScopedVariableResponse(t, e, core.Request{Query: `query Q($v:Color=FIRST){echo(value:$v)}`}, `{"data":{"echo":"SECOND"}}`)
	assertScopedVariableResponse(t, e, core.Request{Query: `query Q($v:Color){echo(value:$v)}`, Variables: map[string]any{"v": nil}}, `{"data":{"echo":null}}`)
}

type operationJSONList []any
type operationMutableJSONQuery struct{}

func (operationMutableJSONQuery) Change(args struct{ Value operationJSON }) string {
	child := args.Value["child"].(operationJSON)
	values := child["values"].(operationJSONList)
	before := values[0].(string)
	values[0] = "changed"
	child["extra"] = true
	return before
}
func (operationMutableJSONQuery) Read(args struct{ Value operationJSON }) string {
	child := args.Value["child"].(operationJSON)
	return child["values"].(operationJSONList)[0].(string)
}

func TestCoercedNamedContainersAreIsolatedAcrossAliases(t *testing.T) {
	parses := 0
	s, err := schema.Build(schema.Config{Query: operationMutableJSONQuery{}, Scalars: []schema.ScalarConfig{{Name: "JSON", Type: operationJSON{}, Parse: func(raw any) (any, error) {
		parses++
		return operationJSON{"child": operationJSON{"values": operationJSONList{raw.(map[string]any)["value"]}}}, nil
	}}}})
	if err != nil {
		t.Fatal(err)
	}
	e := New(s, nil)
	assertScopedVariableResponse(t, e, core.Request{Query: `query Q($v:JSON!){first:change(value:$v) second:read(value:$v)}`, Variables: map[string]any{"v": map[string]any{"value": "original"}}}, `{"data":{"first":"original","second":"original"}}`)
	if parses != 1 {
		t.Fatalf("custom input parsed %d times", parses)
	}
	for _, value := range []any{operationJSON(nil), operationJSONList(nil), operationJSON{"nil": nil}} {
		if got := cloneVariableInput(value); !reflect.DeepEqual(got, value) {
			t.Fatalf("clone changed typed nil values: got%#v want%#v", got, value)
		}
	}
}
