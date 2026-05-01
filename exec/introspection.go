package exec

import (
	"sort"
	"strings"

	"github.com/patrickkabwe/grx/core"
	"github.com/patrickkabwe/grx/schema"
)

func isIntrospectionQuery(query string) bool {
	return strings.Contains(query, "__schema") || strings.Contains(query, "__type(")
}

func introspectionData(schemaValue *schema.Schema, req core.Request) map[string]any {
	data := map[string]any{}
	if strings.Contains(req.Query, "__schema") {
		data["__schema"] = introspectionSchema(schemaValue)
	}

	if strings.Contains(req.Query, "__type(") {
		data["__type"] = introspectionNamedType(schemaValue, req)
	}

	return data
}

func introspectionSchema(schemaValue *schema.Schema) map[string]any {
	return map[string]any{
		"queryType":        introspectionRootType(schemaValue.Query),
		"mutationType":     introspectionRootType(schemaValue.Mutation),
		"subscriptionType": introspectionRootType(schemaValue.Subscription),
		"types":            introspectionTypes(schemaValue),
		"directives":       []any{},
	}
}

func introspectionRootType(object *schema.Object) any {
	if object == nil {
		return nil
	}

	return map[string]any{"name": object.Name()}
}

func introspectionTypes(schemaValue *schema.Schema) []any {
	names := make([]string, 0, len(schemaValue.Types))
	for name := range schemaValue.Types {
		names = append(names, name)
	}
	sort.Strings(names)

	types := make([]any, 0, len(names))
	for _, name := range names {
		types = append(types, introspectionType(schemaValue.Types[name]))
	}
	return types
}

func introspectionNamedType(schemaValue *schema.Schema, req core.Request) any {
	name, ok := introspectionTypeName(req)
	if !ok {
		return nil
	}

	typeValue, ok := schemaValue.Types[name]
	if !ok {
		return nil
	}
	return introspectionType(typeValue)
}

func introspectionTypeName(req core.Request) (string, bool) {
	if raw, ok := req.Variables["name"]; ok {
		name, ok := raw.(string)
		return name, ok
	}

	marker := "name:"
	index := strings.Index(req.Query, marker)
	if index == -1 {
		return "", false
	}

	value := strings.TrimSpace(req.Query[index+len(marker):])
	if value == "" || value[0] != '"' {
		return "", false
	}

	value = value[1:]
	end := strings.Index(value, `"`)
	if end == -1 {
		return "", false
	}

	return value[:end], true
}

func introspectionType(typeValue schema.Type) map[string]any {
	result := map[string]any{
		"kind":          typeValue.Kind(),
		"name":          typeValue.Name(),
		"description":   nil,
		"fields":        nil,
		"inputFields":   nil,
		"interfaces":    []any{},
		"enumValues":    nil,
		"possibleTypes": nil,
	}

	switch typed := typeValue.(type) {
	case *schema.Object:
		result["fields"] = introspectionFields(typed.Fields)
	case *schema.Interface:
		result["fields"] = introspectionFields(typed.Fields)
	case *schema.InputObject:
		result["inputFields"] = introspectionInputFields(typed.Fields)
	case *schema.Union:
		result["possibleTypes"] = introspectionPossibleTypes(typed.Types)
	}

	return result
}

func introspectionFields(fields map[string]*schema.Field) []any {
	names := sortedFieldNames(fields)
	values := make([]any, 0, len(names))
	for _, name := range names {
		field := fields[name]
		values = append(values, map[string]any{
			"name":              field.Name,
			"description":       nil,
			"args":              introspectionInputValues(field.Args),
			"type":              introspectionTypeRef(field.Type),
			"isDeprecated":      false,
			"deprecationReason": nil,
		})
	}
	return values
}

func introspectionInputValues(inputValues []schema.InputValue) []any {
	values := make([]any, 0, len(inputValues))
	for _, inputValue := range inputValues {
		values = append(values, map[string]any{
			"name":         inputValue.Name,
			"description":  nil,
			"type":         introspectionTypeRef(inputValue.Type),
			"defaultValue": inputValue.DefaultValue,
		})
	}
	return values
}

func introspectionInputFields(fields map[string]*schema.Field) []any {
	names := sortedFieldNames(fields)
	values := make([]any, 0, len(names))
	for _, name := range names {
		field := fields[name]
		values = append(values, map[string]any{
			"name":         field.Name,
			"description":  nil,
			"type":         introspectionTypeRef(field.Type),
			"defaultValue": nil,
		})
	}
	return values
}

func introspectionPossibleTypes(types []*schema.Object) []any {
	values := make([]any, 0, len(types))
	for _, object := range types {
		values = append(values, introspectionTypeRef(object))
	}
	return values
}

func sortedFieldNames(fields map[string]*schema.Field) []string {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func introspectionTypeRef(typeValue schema.Type) map[string]any {
	switch typed := typeValue.(type) {
	case *schema.NonNull:
		return map[string]any{
			"kind":   typed.Kind(),
			"name":   nil,
			"ofType": introspectionTypeRef(typed.OfType),
		}
	case *schema.List:
		return map[string]any{
			"kind":   typed.Kind(),
			"name":   nil,
			"ofType": introspectionTypeRef(typed.OfType),
		}
	default:
		return map[string]any{
			"kind":   typeValue.Kind(),
			"name":   typeValue.Name(),
			"ofType": nil,
		}
	}
}
