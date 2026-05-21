package exec

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
)

func coerceBuiltInScalar(typeName string, value any) (any, error) {
	if value == nil {
		return nil, nil
	}
	switch typeName {
	case "Int":
		return coerceInt(value)
	case "Float":
		return coerceFloat(value)
	case "Boolean":
		return coerceBoolean(value)
	case "String", "ID":
		return coerceString(value)
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

func coerceInt(value any) (any, error) {
	switch typed := value.(type) {
	case int:
		return typed, nil
	case int8:
		return int(typed), nil
	case int16:
		return int(typed), nil
	case int32:
		return int(typed), nil
	case int64:
		if typed < math.MinInt || typed > math.MaxInt {
			return nil, fmt.Errorf("integer overflow")
		}
		return int(typed), nil
	case uint, uint8, uint16, uint32, uint64:
		raw := reflect.ValueOf(value).Uint()
		if raw > uint64(math.MaxInt) {
			return nil, fmt.Errorf("integer overflow")
		}
		return int(raw), nil
	case float32:
		if float32(int(typed)) != typed {
			return nil, fmt.Errorf("expected integer value")
		}
		return int(typed), nil
	case float64:
		if float64(int(typed)) != typed {
			return nil, fmt.Errorf("expected integer value")
		}
		return int(typed), nil
	case string:
		parsed, err := strconv.ParseInt(typed, 10, 64)
		if err != nil {
			return nil, err
		}
		if parsed < math.MinInt || parsed > math.MaxInt {
			return nil, fmt.Errorf("integer overflow")
		}
		return int(parsed), nil
	default:
		return nil, fmt.Errorf("cannot coerce %T to Int", value)
	}
}

func coerceFloat(value any) (any, error) {
	switch typed := value.(type) {
	case float32:
		return float64(typed), nil
	case float64:
		return typed, nil
	case int, int8, int16, int32, int64:
		return float64(reflect.ValueOf(value).Int()), nil
	case uint, uint8, uint16, uint32, uint64:
		return float64(reflect.ValueOf(value).Uint()), nil
	case string:
		return strconv.ParseFloat(typed, 64)
	default:
		return nil, fmt.Errorf("cannot coerce %T to Float", value)
	}
}

func coerceBoolean(value any) (any, error) {
	switch typed := value.(type) {
	case bool:
		return typed, nil
	case string:
		return strconv.ParseBool(typed)
	default:
		return nil, fmt.Errorf("cannot coerce %T to Boolean", value)
	}
}

func coerceString(value any) (any, error) {
	switch typed := value.(type) {
	case string:
		return typed, nil
	case *string:
		if typed == nil {
			return nil, nil
		}
		return *typed, nil
	default:
		return fmt.Sprint(value), nil
	}
}
