package exec

import (
	"fmt"
	"math"
	"reflect"

	"github.com/patrickkabwe/grx/schema"
)

func coerceArguments(defs []schema.InputValue, values map[string]any) (map[string]any, error) {
	if len(defs) == 0 {
		return nil, nil
	}
	out := make(map[string]any, len(defs))
	for _, def := range defs {
		raw, exists := values[def.Name]
		if !exists {
			if def.DefaultValue != nil {
				raw = def.DefaultValue
			} else {
				if _, required := def.Type.(*schema.NonNull); required {
					return nil, fmt.Errorf("missing required argument %q", def.Name)
				}
				continue
			}
		}
		coerced, err := coerceInputValue(def.Type, raw)
		if err != nil {
			return nil, fmt.Errorf("argument %q: %w", def.Name, err)
		}
		out[def.Name] = coerced
	}
	return out, nil
}

func coerceInputValue(inputType schema.Type, raw any) (any, error) {
	if ref, ok := raw.(variableRef); ok {
		if !ref.HasValue {
			return coerceInputValue(inputType, nil)
		}
		return coerceInputValue(inputType, ref.Value)
	}
	switch typed := inputType.(type) {
	case *schema.NonNull:
		if raw == nil {
			return nil, fmt.Errorf("expected non-null %s", typed.Name())
		}
		return coerceInputValue(typed.OfType, raw)
	case *schema.List:
		if raw == nil {
			return nil, nil
		}
		rawValue := reflect.ValueOf(raw)
		if rawValue.IsValid() && rawValue.Kind() != reflect.String && (rawValue.Kind() == reflect.Slice || rawValue.Kind() == reflect.Array) {
			out := make([]any, rawValue.Len())
			for index := 0; index < rawValue.Len(); index++ {
				item, err := coerceInputValue(typed.OfType, rawValue.Index(index).Interface())
				if err != nil {
					return nil, fmt.Errorf("list item %d: %w", index, err)
				}
				out[index] = item
			}
			return out, nil
		}
		item, err := coerceInputValue(typed.OfType, raw)
		if err != nil {
			return nil, err
		}
		return []any{item}, nil
	case *schema.InputObject:
		return coerceInputObject(typed, raw)
	case *schema.Enum:
		parsed, err := typed.Parse(raw)
		if err == nil {
			return parsed, nil
		}
		if _, serializeErr := typed.Serialize(raw); serializeErr == nil {
			return raw, nil
		}
		return nil, err
	case *schema.Scalar:
		return coerceScalarInput(typed.TypeName, raw)
	default:
		return raw, nil
	}
}

func coerceInputObject(inputType *schema.InputObject, raw any) (any, error) {
	if raw == nil {
		return nil, nil
	}
	fields, ok := raw.(map[string]any)
	if !ok {
		return nil, fmt.Errorf("expected input object %s, got %T", inputType.Name(), raw)
	}
	out := make(map[string]any, len(inputType.Fields))
	for name := range fields {
		if _, exists := inputType.Fields[name]; !exists {
			return nil, fmt.Errorf("unknown input field %q on %s", name, inputType.Name())
		}
	}
	for name, field := range inputType.Fields {
		rawField, exists := fields[name]
		if !exists {
			if field.DefaultValue != nil {
				rawField = field.DefaultValue
			} else {
				if _, required := field.Type.(*schema.NonNull); required {
					return nil, fmt.Errorf("missing required input field %q on %s", name, inputType.Name())
				}
				continue
			}
		}
		coerced, err := coerceInputValue(field.Type, rawField)
		if err != nil {
			return nil, fmt.Errorf("input field %q: %w", name, err)
		}
		out[name] = coerced
	}
	if inputType.IsOneOf {
		if len(out) != 1 {
			return nil, fmt.Errorf("OneOf input object %s must specify exactly one field", inputType.Name())
		}
		for fieldName, value := range out {
			if value == nil {
				return nil, fmt.Errorf("OneOf input object %s field %q must not be null", inputType.Name(), fieldName)
			}
		}
	}
	return out, nil
}

func coerceScalarInput(typeName string, raw any) (any, error) {
	if raw == nil {
		return nil, nil
	}
	switch typeName {
	case "Int":
		return coerceIntInput(raw)
	case "Float":
		return coerceFloatInput(raw)
	case "Boolean":
		if value, ok := raw.(bool); ok {
			return value, nil
		}
		return nil, fmt.Errorf("expected Boolean, got %T", raw)
	case "String":
		if value, ok := raw.(string); ok {
			return value, nil
		}
		return nil, fmt.Errorf("expected String, got %T", raw)
	case "ID":
		return coerceIDInput(raw)
	default:
		return raw, nil
	}
}

func coerceIntInput(raw any) (any, error) {
	value, ok := signedInteger(raw)
	if !ok {
		return nil, fmt.Errorf("expected Int, got %T", raw)
	}
	if value < math.MinInt32 || value > math.MaxInt32 {
		return nil, fmt.Errorf("Int value %d outside 32-bit range", value)
	}
	return int(value), nil
}

func coerceFloatInput(raw any) (any, error) {
	switch value := raw.(type) {
	case float32:
		return float64(value), nil
	case float64:
		if math.IsNaN(value) || math.IsInf(value, 0) {
			return nil, fmt.Errorf("Float value must be finite")
		}
		return value, nil
	case int, int8, int16, int32, int64:
		return float64(reflect.ValueOf(raw).Int()), nil
	case uint, uint8, uint16, uint32:
		return float64(reflect.ValueOf(raw).Uint()), nil
	default:
		return nil, fmt.Errorf("expected Float, got %T", raw)
	}
}

func coerceIDInput(raw any) (any, error) {
	switch value := raw.(type) {
	case string:
		return value, nil
	case int, int8, int16, int32, int64:
		return fmt.Sprint(reflect.ValueOf(raw).Int()), nil
	case uint, uint8, uint16, uint32:
		return fmt.Sprint(reflect.ValueOf(raw).Uint()), nil
	default:
		return nil, fmt.Errorf("expected ID, got %T", raw)
	}
}

func signedInteger(raw any) (int64, bool) {
	switch value := raw.(type) {
	case int:
		return int64(value), true
	case int8:
		return int64(value), true
	case int16:
		return int64(value), true
	case int32:
		return int64(value), true
	case int64:
		return value, true
	case float64:
		if math.IsNaN(value) || math.IsInf(value, 0) || math.Trunc(value) != value {
			return 0, false
		}
		return int64(value), true
	default:
		return 0, false
	}
}
