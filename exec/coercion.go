package exec

import (
	"fmt"
	"math"
	"reflect"
)

func coerceBuiltInScalar(typeName string, value any) (any, error) {
	if value == nil {
		return nil, nil
	}
	raw := reflect.ValueOf(value)
	for raw.IsValid() && raw.Kind() == reflect.Pointer {
		if raw.IsNil() {
			return nil, nil
		}
		value = raw.Elem().Interface()
		raw = reflect.ValueOf(value)
	}
	switch typeName {
	case "Int":
		return coerceIntOutput(value)
	case "Float":
		return coerceFloatOutput(value)
	case "Boolean":
		if typed, ok := value.(bool); ok {
			return typed, nil
		}
		return nil, fmt.Errorf("cannot serialize %T as Boolean", value)
	case "String":
		if typed, ok := value.(string); ok {
			return typed, nil
		}
		return nil, fmt.Errorf("cannot serialize %T as String", value)
	case "ID":
		return coerceIDOutput(value)
	default:
		return value, nil
	}
}

func coerceBuiltInScalarOutput(value any) (any, error) {
	switch value.(type) {
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64, float32, float64, bool, string:
		return value, nil
	default:
		return value, nil
	}
}

func coerceIntOutput(value any) (any, error) {
	switch typed := value.(type) {
	case int:
		if typed < math.MinInt32 || typed > math.MaxInt32 {
			return nil, fmt.Errorf("integer overflow")
		}
		return typed, nil
	case int8:
		return int(typed), nil
	case int16:
		return int(typed), nil
	case int32:
		return int(typed), nil
	case int64:
		if typed < math.MinInt32 || typed > math.MaxInt32 {
			return nil, fmt.Errorf("integer overflow")
		}
		return int(typed), nil
	case uint, uint8, uint16, uint32, uint64:
		raw := reflect.ValueOf(value).Uint()
		if raw > uint64(math.MaxInt32) {
			return nil, fmt.Errorf("integer overflow")
		}
		return int(raw), nil
	default:
		return nil, fmt.Errorf("cannot serialize %T as Int", value)
	}
}

func coerceFloatOutput(value any) (any, error) {
	switch typed := value.(type) {
	case float32:
		return float64(typed), nil
	case float64:
		if math.IsNaN(typed) || math.IsInf(typed, 0) {
			return nil, fmt.Errorf("Float value must be finite")
		}
		return typed, nil
	case int, int8, int16, int32, int64:
		return float64(reflect.ValueOf(value).Int()), nil
	case uint, uint8, uint16, uint32, uint64:
		return float64(reflect.ValueOf(value).Uint()), nil
	default:
		return nil, fmt.Errorf("cannot serialize %T as Float", value)
	}
}

func coerceIDOutput(value any) (any, error) {
	switch typed := value.(type) {
	case string:
		return typed, nil
	case int, int8, int16, int32, int64:
		return fmt.Sprint(reflect.ValueOf(value).Int()), nil
	case uint, uint8, uint16, uint32, uint64:
		return fmt.Sprint(reflect.ValueOf(value).Uint()), nil
	default:
		return nil, fmt.Errorf("cannot serialize %T as ID", value)
	}
}
