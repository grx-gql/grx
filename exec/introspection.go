package exec

import (
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/grx-gql/grx/schema"
)

func introspectionDefaultValue(valueType schema.Type, value any) any {
	if value == nil {
		return nil
	}
	formatted, ok := formatGraphQLValueLiteral(valueType, value)
	if !ok {
		return nil
	}
	return formatted
}

func formatGraphQLValueLiteral(valueType schema.Type, value any) (string, bool) {
	if value == nil {
		return "null", true
	}
	switch typed := valueType.(type) {
	case *schema.NonNull:
		return formatGraphQLValueLiteral(typed.OfType, value)
	case *schema.List:
		return formatGraphQLListLiteral(typed.OfType, value)
	case *schema.InputObject:
		return formatGraphQLInputObjectLiteral(typed, value)
	case *schema.Enum:
		name, err := typed.Serialize(value)
		if err != nil {
			if raw, ok := value.(string); ok {
				return raw, true
			}
			return "", false
		}
		enumName, ok := name.(string)
		return enumName, ok
	case *schema.Scalar:
		return formatGraphQLScalarLiteral(typed.TypeName, value)
	default:
		return "", false
	}
}

func formatGraphQLScalarLiteral(typeName string, value any) (string, bool) {
	switch typeName {
	case "String":
		raw, ok := value.(string)
		if !ok {
			return "", false
		}
		return strconv.Quote(raw), true
	case "Boolean":
		raw, ok := value.(bool)
		if !ok {
			return "", false
		}
		if raw {
			return "true", true
		}
		return "false", true
	case "Int":
		raw, ok := signedInteger(value)
		if !ok {
			return "", false
		}
		return strconv.FormatInt(raw, 10), true
	case "Float":
		switch raw := value.(type) {
		case float32:
			return strconv.FormatFloat(float64(raw), 'f', -1, 32), true
		case float64:
			return strconv.FormatFloat(raw, 'f', -1, 64), true
		case int, int8, int16, int32, int64:
			return strconv.FormatInt(reflect.ValueOf(value).Int(), 10), true
		default:
			return "", false
		}
	case "ID":
		switch raw := value.(type) {
		case string:
			return strconv.Quote(raw), true
		case int, int8, int16, int32, int64:
			return strconv.FormatInt(reflect.ValueOf(value).Int(), 10), true
		case uint, uint8, uint16, uint32:
			return strconv.FormatUint(reflect.ValueOf(value).Uint(), 10), true
		default:
			return "", false
		}
	default:
		if raw, ok := value.(string); ok {
			return strconv.Quote(raw), true
		}
		return "", false
	}
}

func formatGraphQLListLiteral(itemType schema.Type, value any) (string, bool) {
	raw := reflect.ValueOf(value)
	if !raw.IsValid() || (raw.Kind() != reflect.Slice && raw.Kind() != reflect.Array) {
		return "", false
	}
	parts := make([]string, 0, raw.Len())
	for index := 0; index < raw.Len(); index++ {
		formatted, ok := formatGraphQLValueLiteral(itemType, raw.Index(index).Interface())
		if !ok {
			return "", false
		}
		parts = append(parts, formatted)
	}
	return "[" + strings.Join(parts, ", ") + "]", true
}

func formatGraphQLInputObjectLiteral(inputType *schema.InputObject, value any) (string, bool) {
	fields, ok := value.(map[string]any)
	if !ok {
		return "", false
	}
	names := sortedFieldNames(inputType.Fields)
	parts := make([]string, 0, len(names))
	for _, name := range names {
		fieldValue, exists := fields[name]
		if !exists {
			continue
		}
		field := inputType.Fields[name]
		formatted, ok := formatGraphQLValueLiteral(field.Type, fieldValue)
		if !ok {
			return "", false
		}
		parts = append(parts, fmt.Sprintf("%s: %s", name, formatted))
	}
	return "{" + strings.Join(parts, ", ") + "}", true
}

func nullableString(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func sortedFieldNames(fields map[string]*schema.Field) []string {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
