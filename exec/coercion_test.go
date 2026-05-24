package exec

import (
	"context"
	"fmt"
	"math"
	"testing"

	"github.com/patrickkabwe/grx/core"
	"github.com/patrickkabwe/grx/schema"
)

type listCoercionQuery struct{}

func (listCoercionQuery) Echo(ctx context.Context, args struct {
	Names []string `gql:"names,nonNull"`
}) ([]string, error) {
	return args.Names, nil
}

func TestListInputCoercionFromJSONVariables(t *testing.T) {
	schemaValue, err := schema.Build(schema.Config{Query: listCoercionQuery{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	executor := New(schemaValue, nil)
	response := executor.Execute(context.Background(), core.Request{
		Query:     `query($names: [String!]!) { echo(names: $names) }`,
		Variables: map[string]any{"names": []any{"a", "b"}},
	})
	if len(response.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", response.Errors)
	}

	data := responseObject(t, response.Data)
	got, ok := data["echo"].([]any)
	if !ok {
		t.Fatalf("expected list result, got %T", data["echo"])
	}
	if len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("unexpected echo result: %#v", got)
	}
}

type scalarCoercionQuery struct{}

func (scalarCoercionQuery) Count(ctx context.Context) (int, error) {
	return 42, nil
}

func TestScalarResultCoercionSerializesBuiltInInt(t *testing.T) {
	schemaValue, err := schema.Build(schema.Config{Query: scalarCoercionQuery{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	executor := New(schemaValue, nil)
	response := executor.Execute(context.Background(), core.Request{
		Query: `{ count }`,
	})
	if len(response.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", response.Errors)
	}

	data := responseObject(t, response.Data)
	switch v := data["count"].(type) {
	case int:
		if v != 42 {
			t.Fatalf("expected 42, got %d", v)
		}
	case int64:
		if v != 42 {
			t.Fatalf("expected 42, got %d", v)
		}
	case float64:
		if v != 42 {
			t.Fatalf("expected 42, got %v", v)
		}
	default:
		t.Fatalf("expected numeric count, got %T %#v", data["count"], data["count"])
	}
}

type builtinScalarsQuery struct{}

func (builtinScalarsQuery) NullableInt(context.Context) (*int, error) {
	return nil, nil
}

func (builtinScalarsQuery) BoxedNine(context.Context) (*int, error) {
	v := 9
	return &v, nil
}

func (builtinScalarsQuery) Truth(context.Context) (bool, error) {
	return true, nil
}

func (builtinScalarsQuery) Label(context.Context) (string, error) {
	return "ok", nil
}

func (builtinScalarsQuery) Narrow16(context.Context) (int16, error) {
	return 5, nil
}

func (builtinScalarsQuery) Narrow32(context.Context) (int32, error) {
	return 6, nil
}

func (builtinScalarsQuery) Narrow64(context.Context) (int64, error) {
	return 100, nil
}

func (builtinScalarsQuery) Approx32(context.Context) (float32, error) {
	return 1.25, nil
}

func (builtinScalarsQuery) Exact64(context.Context) (float64, error) {
	return 2.5, nil
}

func TestExecutorBuiltinScalarsFromTypicalResolverTypes(t *testing.T) {
	sv, err := schema.Build(schema.Config{Query: builtinScalarsQuery{}})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}
	ex := New(sv, nil)
	resp := ex.Execute(context.Background(), core.Request{Query: `{
			nullableInt
			boxedNine
			truth
			label
			narrow16
			narrow32
			narrow64
			approx32
			exact64
		}`})
	if len(resp.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", resp.Errors)
	}
	data := responseObject(t, resp.Data)

	if raw, exists := data["nullableInt"]; !exists || raw != nil {
		t.Fatalf(`expected nullable GraphQL Int to be absent or JSON null (nil Go), exists=%v %#v`, exists, raw)
	}
	for key, want := range map[string]int{
		"boxedNine": 9,
		"narrow16":  5,
		"narrow32":  6,
		"narrow64":  100,
	} {
		switch v := data[key].(type) {
		case int:
			if v != want {
				t.Fatalf("%s: want %d, got %d", key, want, v)
			}
		case int64:
			if int(v) != want {
				t.Fatalf("%s: want %d, got %d", key, want, v)
			}
		case float64:
			if int(v) != want {
				t.Fatalf("%s: want %d, got %v", key, want, v)
			}
		default:
			t.Fatalf("%s: unexpected type %T value %#v", key, data[key], data[key])
		}
	}

	if truth, ok := data["truth"].(bool); !ok || !truth {
		t.Fatalf(`expected truth = true, got %#v (%T)`, data["truth"], data["truth"])
	}
	if got, ok := data["label"].(string); !ok || got != "ok" {
		t.Fatalf(`expected label "ok", got %#v (%T)`, data["label"], data["label"])
	}

	for key, want := range map[string]float64{
		"approx32": 1.25,
		"exact64":  2.5,
	} {
		switch v := data[key].(type) {
		case float64:
			diff := math.Abs(v - want)
			if diff > 1e-9 {
				t.Fatalf("%s: want ~%v, got %v", key, want, data[key])
			}
		case float32:
			if math.Abs(float64(v)-want) > 1e-6 {
				t.Fatalf("%s: want ~%v, got %v", key, want, data[key])
			}
		default:
			t.Fatalf("%s: unexpected type %T value %#v", key, data[key], data[key])
		}
	}
}

const builtinIntSerializeNullSentinel = 724

type customIntSerializeQuery struct{}

func (customIntSerializeQuery) Ordinary(context.Context) (int, error) {
	return 11, nil
}

func (customIntSerializeQuery) AbsentViaCustomSerialize(context.Context) (int, error) {
	return builtinIntSerializeNullSentinel, nil
}

func TestExecutorGraphQLNullWhenBuiltinIntSerializeReturnsAbsent(t *testing.T) {
	// When a scalar Serialize hook returns absent (nil), the executor must expose
	// GraphQL null on the wire even if the resolver yielded a concrete int.
	parseIntInput := func(input any) (any, error) {
		switch value := input.(type) {
		case int:
			return value, nil
		case int64:
			return int(value), nil
		case float64:
			return int(value), nil
		default:
			return nil, fmt.Errorf("expected numeric Int literal, got %T", input)
		}
	}

	serializeInt := func(value any) (any, error) {
		switch typed := value.(type) {
		case nil:
			return nil, nil
		case int:
			if typed == builtinIntSerializeNullSentinel {
				return nil, nil
			}
			return typed, nil
		default:
			return nil, fmt.Errorf("unexpected resolver value type %T for Int", typed)
		}
	}

	sv, err := schema.Build(schema.Config{
		Query: customIntSerializeQuery{},
		Scalars: []schema.ScalarConfig{
			{Name: "Int", Type: int(0), Parse: parseIntInput, Serialize: serializeInt},
		},
	})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	ex := New(sv, nil)
	resp := ex.Execute(context.Background(), core.Request{
		Query: `{ ordinary absentViaCustomSerialize }`,
	})
	if len(resp.Errors) != 0 {
		t.Fatalf("unexpected errors: %#v", resp.Errors)
	}
	data := responseObject(t, resp.Data)

	switch ordinary := data["ordinary"].(type) {
	case int:
		if ordinary != 11 {
			t.Fatalf("ordinary want 11 got %d", ordinary)
		}
	case float64:
		if int(ordinary) != 11 {
			t.Fatalf("ordinary want ~11 got %v", data["ordinary"])
		}
	default:
		t.Fatalf("ordinary unexpected %#v (%T)", data["ordinary"], data["ordinary"])
	}

	if raw, exists := data["absentViaCustomSerialize"]; exists && raw != nil {
		t.Fatalf("want GraphQL null for custom-serialized nil Int; exists=%v value=%#v", exists, raw)
	}
}

func TestCoercionAndDirectiveHelpers(t *testing.T) {
	if got, err := coerceBuiltInScalarOutput(struct{ A int }{A: 1}); err != nil || got == nil {
		t.Fatalf("generic output = %#v %v", got, err)
	}
	if got, err := coerceIntOutput(int8(7)); err != nil || got != 7 {
		t.Fatalf("int output = %#v %v", got, err)
	}
	if got, err := coerceIntOutput(uint16(7)); err != nil || got != 7 {
		t.Fatalf("uint int output = %#v %v", got, err)
	}
	if _, err := coerceIntOutput(int64(math.MaxInt64)); err == nil {
		t.Fatal("expected int output overflow")
	}
	if got, err := coerceFloatOutput(int32(7)); err != nil || got != float64(7) {
		t.Fatalf("float output = %#v %v", got, err)
	}
	if got, err := coerceFloatOutput(uint16(7)); err != nil || got != float64(7) {
		t.Fatalf("uint float output = %#v %v", got, err)
	}
	if _, err := coerceFloatOutput(math.NaN()); err == nil {
		t.Fatal("expected finite float constraint for NaN")
	}

	if _, err := coerceFloatOutput(math.Inf(-1)); err == nil {
		t.Fatal("expected finite float constraint for negative infinity")
	}
	if _, err := coerceIntOutput(int(math.MaxInt32) + 1); err == nil {
		t.Fatal("expected int overflow beyond int32 envelope")
	}
	if _, err := coerceFloatOutput(struct{}{}); err == nil {
		t.Fatal("expected float coercion error for structs")
	}
	if _, err := coerceIntOutput("x"); err == nil {
		t.Fatal("expected int coercion error for strings")
	}
	if got, err := coerceBuiltInScalar("UnsetScalar", complex(1, 2)); err != nil || got != complex(1, 2) {
		t.Fatalf("fallback scalar passthrough = %#v %v", got, err)
	}
	if got, err := coerceIDOutput(uint16(7)); err != nil || got != "7" {
		t.Fatalf("id output = %#v %v", got, err)
	}
	if _, err := coerceBuiltInScalar("Boolean", "x"); err == nil {
		t.Fatal("expected scalar output error")
	}
	for _, value := range []any{int(1), int16(2), int32(3), int64(4), uint(5), uint8(6), uint32(7), float32(8)} {
		if got, err := coerceBuiltInScalarOutput(value); err != nil || got == nil {
			t.Fatalf("generic scalar output %T = %#v %v", value, got, err)
		}
	}
	if _, err := coerceIntOutput(uint64(math.MaxUint64)); err == nil {
		t.Fatal("expected uint int output overflow")
	}
	if got, err := coerceIDOutput("abc"); err != nil || got != "abc" {
		t.Fatalf("string id output = %#v %v", got, err)
	}
	if got, err := coerceIDOutput(int32(9)); err != nil || got != "9" {
		t.Fatalf("signed id output = %#v %v", got, err)
	}
	if _, err := coerceIDOutput(struct{}{}); err == nil {
		t.Fatal("expected invalid id output")
	}

	if got, err := coerceFloatInput(int64(3)); err != nil || got != float64(3) {
		t.Fatalf("float input = %#v %v", got, err)
	}
	if got, err := coerceIntInput(int64(3)); err != nil || got != 3 {
		t.Fatalf("int input = %#v %v", got, err)
	}
	if _, err := coerceIntInput(uint16(3)); err == nil {
		t.Fatal("expected unsigned int input error")
	}
	if _, err := coerceIntInput(float64(1.5)); err == nil {
		t.Fatal("expected fractional int input error")
	}
	if got, err := coerceFloatInput(uint16(3)); err != nil || got != float64(3) {
		t.Fatalf("uint float input = %#v %v", got, err)
	}
	if _, err := coerceFloatInput(math.Inf(1)); err == nil {
		t.Fatal("finite float infinity expected error")
	}
	if _, err := coerceFloatInput(uint64(99)); err == nil {
		t.Fatal("uint64 unexpected for Float input coercion")
	}
	if got, err := coerceIDInput(int64(3)); err != nil || got != "3" {
		t.Fatalf("id input = %#v %v", got, err)
	}
	if got, err := coerceIDInput(uint16(3)); err != nil || got != "3" {
		t.Fatalf("uint id input = %#v %v", got, err)
	}
	if got, err := coerceIDInput("abc"); err != nil || got != "abc" {
		t.Fatalf("string id input = %#v %v", got, err)
	}
	if _, err := coerceIDInput(struct{}{}); err == nil {
		t.Fatal("expected invalid id input")
	}
	for _, value := range []any{int(1), int8(1), int16(1), int32(1), int64(1), float64(1)} {
		if _, ok := signedInteger(value); !ok {
			t.Fatalf("signedInteger(%T) failed", value)
		}
	}
	if _, ok := signedInteger(1.5); ok {
		t.Fatal("fractional float should not be signed integer")
	}
	if _, ok := signedInteger(uint64(1)); ok {
		t.Fatal("expected unsigned integer rejection")
	}

	inputObject := &schema.InputObject{TypeName: "Input", Fields: map[string]*schema.Field{
		"required":    {Name: "required", Type: &schema.NonNull{OfType: &schema.Scalar{TypeName: "String"}}},
		"list":        {Name: "list", Type: &schema.List{OfType: &schema.Scalar{TypeName: "Int"}}},
		"withDefault": {Name: "withDefault", Type: &schema.Scalar{TypeName: "Boolean"}, DefaultValue: true},
	}}
	if got, err := coerceInputValue(inputObject, map[string]any{"required": "x", "list": int64(1)}); err != nil || got == nil {
		t.Fatalf("input object coercion = %#v %v", got, err)
	}
	if _, err := coerceInputValue(inputObject, map[string]any{"unknown": true}); err == nil {
		t.Fatal("expected unknown input field")
	}
	if _, err := coerceInputValue(inputObject, map[string]any{}); err == nil {
		t.Fatal("expected required input field")
	}
	oneOf := &schema.InputObject{TypeName: "One", IsOneOf: true, Fields: map[string]*schema.Field{
		"a": {Name: "a", Type: &schema.Scalar{TypeName: "String"}},
		"b": {Name: "b", Type: &schema.Scalar{TypeName: "String"}},
	}}
	if _, err := coerceInputValue(oneOf, map[string]any{"a": "x", "b": "y"}); err == nil {
		t.Fatal("expected oneOf count error")
	}
	if _, err := coerceInputValue(oneOf, map[string]any{"a": nil}); err == nil {
		t.Fatal("expected oneOf nil field error")
	}

	listOfInt := &schema.List{OfType: &schema.Scalar{TypeName: "Int"}}
	if got, err := coerceInputValue(listOfInt, int64(44)); err != nil {
		t.Fatalf("scalar-to-list: %v", err)
	} else if sl, ok := got.([]any); !ok || len(sl) != 1 || sl[0] != int(44) {
		t.Fatalf("wrapped scalar list = %#v", got)
	}
	if _, err := coerceInputValue(listOfInt, "nope"); err == nil {
		t.Fatal("expected list coercion error for incompatible scalar")
	}
	if _, err := coerceInputValue(listOfInt, []any{"bad"}); err == nil {
		t.Fatal("expected list element coercion error")
	}
	if got, err := coerceInputValue(&schema.Interface{TypeName: "Iface"}, []any{"delegated"}); err != nil || got == nil {
		t.Fatalf("passthrough coercion = %#v %v", got, err)
	}
	nnStr := &schema.NonNull{OfType: &schema.Scalar{TypeName: "String"}}
	if _, err := coerceInputValue(nnStr, variableRef{Name: "$missing"}); err == nil {
		t.Fatal("expected non-null error for unresolved variable placeholder")
	}
	gotStr, err := coerceInputValue(nnStr, variableRef{Name: "$n", HasValue: true, Value: "ok"})
	if err != nil || gotStr != "ok" {
		t.Fatalf("resolved variable coercion = %#v %v", gotStr, err)
	}

	dirs := []directive{{Name: "include", Args: map[string]any{"if": true}}, {Name: "skip", Args: map[string]any{"if": false}}}
	if skip, include, err := evalSkipInclude(dirs); err != nil || skip || !include {
		t.Fatalf("skip/include = %v %v %v", skip, include, err)
	}
	if _, _, err := evalSkipInclude([]directive{{Name: "skip", Args: map[string]any{"if": "bad"}}}); err == nil {
		t.Fatal("expected bool directive error")
	}

	obj := &schema.Object{TypeName: "Obj", Interfaces: []*schema.Interface{{TypeName: "Iface"}}}
	if !fragmentTypeMatches(obj, "Obj") {
		t.Fatal("object should match itself")
	}
	if fragmentTypeMatches(obj, "Other") {
		t.Fatal("object should not match unrelated type")
	}
	if !fragmentTypeMatches(obj, "Iface") {
		t.Fatal("object should match interface possible type")
	}
}
