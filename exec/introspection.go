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

func introspectionData(schemaValue *schema.Schema, req core.Request) *core.OrderedObject {
	data := core.NewOrderedObject(2)
	if strings.Contains(req.Query, "__schema") {
		data.Set("__schema", introspectionSchema(schemaValue))
	}

	if strings.Contains(req.Query, "__type(") {
		data.Set("__type", introspectionNamedType(schemaValue, req))
	}

	return data
}

func introspectionSchema(schemaValue *schema.Schema) *core.OrderedObject {
	result := core.NewOrderedObject(5)
	result.Set("queryType", introspectionRootType(schemaValue.Query))
	result.Set("mutationType", introspectionRootType(schemaValue.Mutation))
	result.Set("subscriptionType", introspectionRootType(schemaValue.Subscription))
	result.Set("types", introspectionTypes(schemaValue))
	result.Set("directives", []any{})
	return result
}

func introspectionRootType(object *schema.Object) any {
	if object == nil {
		return nil
	}

	result := core.NewOrderedObject(1)
	result.Set("name", object.Name())
	return result
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

func introspectionType(typeValue schema.Type) *core.OrderedObject {
	result := core.NewOrderedObject(8)
	result.Set("kind", typeValue.Kind())
	result.Set("name", typeValue.Name())
	result.Set("description", nil)
	result.Set("fields", nil)
	result.Set("inputFields", nil)
	result.Set("interfaces", []any{})
	result.Set("enumValues", nil)
	result.Set("possibleTypes", nil)

	switch typed := typeValue.(type) {
	case *schema.Object:
		result.Set("fields", introspectionFields(typed.Fields))
		result.Set("interfaces", introspectionInterfaces(typed.Interfaces))
	case *schema.Interface:
		result.Set("fields", introspectionFields(typed.Fields))
		result.Set("possibleTypes", introspectionPossibleTypes(typed.PossibleTypes))
	case *schema.InputObject:
		result.Set("inputFields", introspectionInputFields(typed.Fields))
	case *schema.Union:
		result.Set("possibleTypes", introspectionPossibleTypes(typed.Types))
	case *schema.Enum:
		result.Set("enumValues", introspectionEnumValues(typed.Values))
	}

	return result
}

func introspectionFields(fields map[string]*schema.Field) []any {
	names := sortedFieldNames(fields)
	values := make([]any, 0, len(names))
	for _, name := range names {
		field := fields[name]
		entry := core.NewOrderedObject(6)
		entry.Set("name", field.Name)
		entry.Set("description", nil)
		entry.Set("args", introspectionInputValues(field.Args))
		entry.Set("type", introspectionTypeRef(field.Type))
		entry.Set("isDeprecated", false)
		entry.Set("deprecationReason", nil)
		values = append(values, entry)
	}
	return values
}

func introspectionInputValues(inputValues []schema.InputValue) []any {
	values := make([]any, 0, len(inputValues))
	for _, inputValue := range inputValues {
		entry := core.NewOrderedObject(4)
		entry.Set("name", inputValue.Name)
		entry.Set("description", nil)
		entry.Set("type", introspectionTypeRef(inputValue.Type))
		entry.Set("defaultValue", inputValue.DefaultValue)
		values = append(values, entry)
	}
	return values
}

func introspectionInputFields(fields map[string]*schema.Field) []any {
	names := sortedFieldNames(fields)
	values := make([]any, 0, len(names))
	for _, name := range names {
		field := fields[name]
		entry := core.NewOrderedObject(4)
		entry.Set("name", field.Name)
		entry.Set("description", nil)
		entry.Set("type", introspectionTypeRef(field.Type))
		entry.Set("defaultValue", field.DefaultValue)
		values = append(values, entry)
	}
	return values
}

func introspectionInterfaces(interfaces []*schema.Interface) []any {
	values := make([]any, 0, len(interfaces))
	for _, interfaceType := range interfaces {
		values = append(values, introspectionTypeRef(interfaceType))
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

func introspectionEnumValues(values []schema.EnumValue) []any {
	enumValues := make([]any, 0, len(values))
	for _, value := range values {
		entry := core.NewOrderedObject(4)
		entry.Set("name", value.Name)
		entry.Set("description", nil)
		entry.Set("isDeprecated", false)
		entry.Set("deprecationReason", nil)
		enumValues = append(enumValues, entry)
	}
	return enumValues
}

func sortedFieldNames(fields map[string]*schema.Field) []string {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func introspectionTypeRef(typeValue schema.Type) *core.OrderedObject {
	switch typed := typeValue.(type) {
	case *schema.NonNull:
		result := core.NewOrderedObject(3)
		result.Set("kind", typed.Kind())
		result.Set("name", nil)
		result.Set("ofType", introspectionTypeRef(typed.OfType))
		return result
	case *schema.List:
		result := core.NewOrderedObject(3)
		result.Set("kind", typed.Kind())
		result.Set("name", nil)
		result.Set("ofType", introspectionTypeRef(typed.OfType))
		return result
	default:
		result := core.NewOrderedObject(3)
		result.Set("kind", typeValue.Kind())
		result.Set("name", typeValue.Name())
		result.Set("ofType", nil)
		return result
	}
}
