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

func introspectionIncludeDeprecated(query string) bool {
	return strings.Contains(query, "includeDeprecated: true")
}

func introspectionData(schemaValue *schema.Schema, req core.Request) *core.OrderedObject {
	includeDeprecated := introspectionIncludeDeprecated(req.Query)
	data := core.NewOrderedObject(2)
	if strings.Contains(req.Query, "__schema") {
		data.Set("__schema", introspectionSchema(schemaValue, includeDeprecated))
	}

	if strings.Contains(req.Query, "__type(") {
		data.Set("__type", introspectionNamedType(schemaValue, req, includeDeprecated))
	}

	return data
}

func introspectionSchema(schemaValue *schema.Schema, includeDeprecated bool) *core.OrderedObject {
	result := core.NewOrderedObject(5)
	result.Set("queryType", introspectionRootType(schemaValue.Query))
	result.Set("mutationType", introspectionRootType(schemaValue.Mutation))
	result.Set("subscriptionType", introspectionRootType(schemaValue.Subscription))
	result.Set("types", introspectionTypes(schemaValue, includeDeprecated))
	result.Set("directives", introspectionBuiltinDirectives())
	return result
}

func introspectionBuiltinDirectives() []any {
	boolType := booleanScalar()
	strType := &schema.Scalar{TypeName: "String"}
	intType := &schema.Scalar{TypeName: "Int"}
	return []any{
		introspectionDirective("skip", []string{"FIELD", "FRAGMENT_SPREAD", "INLINE_FRAGMENT"}, []schema.InputValue{
			{Name: "if", Type: &schema.NonNull{OfType: boolType}},
		}),
		introspectionDirective("include", []string{"FIELD", "FRAGMENT_SPREAD", "INLINE_FRAGMENT"}, []schema.InputValue{
			{Name: "if", Type: &schema.NonNull{OfType: boolType}},
		}),
		introspectionDirective("deprecated", []string{"FIELD_DEFINITION", "ARGUMENT_DEFINITION", "INPUT_FIELD_DEFINITION", "ENUM_VALUE"}, []schema.InputValue{
			{Name: "reason", Type: strType, DefaultValue: "No longer supported"},
		}),
		introspectionDirective("specifiedBy", []string{"SCALAR"}, []schema.InputValue{
			{Name: "url", Type: &schema.NonNull{OfType: strType}},
		}),
		introspectionDirectiveRepeatable("oneOf", []string{"INPUT_OBJECT"}, nil, false),
		introspectionDirectiveRepeatable("defer", []string{"FRAGMENT_SPREAD", "FIELD"}, []schema.InputValue{
			{Name: "if", Type: boolType, DefaultValue: true},
			{Name: "label", Type: strType},
		}, false),
		introspectionDirectiveRepeatable("stream", []string{"FIELD"}, []schema.InputValue{
			{Name: "if", Type: boolType, DefaultValue: true},
			{Name: "label", Type: strType},
			{Name: "initialCount", Type: intType, DefaultValue: 0},
		}, false),
	}
}

func introspectionDirectiveRepeatable(name string, locations []string, args []schema.InputValue, repeatable bool) *core.OrderedObject {
	entry := core.NewOrderedObject(5)
	entry.Set("name", name)
	entry.Set("description", nil)
	entry.Set("locations", locations)
	if args == nil {
		args = []schema.InputValue{}
	}
	entry.Set("args", introspectionInputValues(args))
	entry.Set("isRepeatable", repeatable)
	return entry
}

func booleanScalar() schema.Type {
	return &schema.Scalar{TypeName: "Boolean"}
}

func introspectionDirective(name string, locations []string, args []schema.InputValue) *core.OrderedObject {
	entry := core.NewOrderedObject(5)
	entry.Set("name", name)
	entry.Set("description", nil)
	entry.Set("locations", locations)
	entry.Set("args", introspectionInputValues(args))
	entry.Set("isRepeatable", false)
	return entry
}

func introspectionRootType(object *schema.Object) any {
	if object == nil {
		return nil
	}

	result := core.NewOrderedObject(1)
	result.Set("name", object.Name())
	return result
}

func introspectionTypes(schemaValue *schema.Schema, includeDeprecated bool) []any {
	names := make([]string, 0, len(schemaValue.Types))
	for name := range schemaValue.Types {
		names = append(names, name)
	}
	sort.Strings(names)

	types := make([]any, 0, len(names))
	for _, name := range names {
		types = append(types, introspectionType(schemaValue.Types[name], includeDeprecated))
	}
	return types
}

func introspectionNamedType(schemaValue *schema.Schema, req core.Request, includeDeprecated bool) any {
	name, ok := introspectionTypeName(req)
	if !ok {
		return nil
	}

	typeValue, ok := schemaValue.Types[name]
	if !ok {
		return nil
	}
	return introspectionType(typeValue, includeDeprecated)
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

func introspectionType(typeValue schema.Type, includeDeprecated bool) *core.OrderedObject {
	result := core.NewOrderedObject(9)
	result.Set("kind", typeValue.Kind())
	result.Set("name", typeValue.Name())
	result.Set("description", nil)
	result.Set("fields", nil)
	result.Set("inputFields", nil)
	result.Set("interfaces", []any{})
	result.Set("enumValues", nil)
	result.Set("possibleTypes", nil)
	result.Set("specifiedByURL", nil)

	switch typed := typeValue.(type) {
	case *schema.Scalar:
		result.Set("description", nullableString(typed.Description))
		if typed.SpecifiedByURL != "" {
			result.Set("specifiedByURL", typed.SpecifiedByURL)
		}
	case *schema.Object:
		result.Set("description", nullableString(typed.Description))
		result.Set("fields", introspectionFields(typed.Fields, includeDeprecated))
		result.Set("interfaces", introspectionInterfaces(typed.Interfaces))
	case *schema.Interface:
		result.Set("description", nullableString(typed.Description))
		result.Set("fields", introspectionFields(typed.Fields, includeDeprecated))
		result.Set("possibleTypes", introspectionPossibleTypes(typed.PossibleTypes))
	case *schema.InputObject:
		result.Set("description", nullableString(typed.Description))
		if typed.IsOneOf {
			result.Set("isOneOf", true)
		}
		result.Set("inputFields", introspectionInputFields(typed.Fields))
	case *schema.Union:
		result.Set("description", nullableString(typed.Description))
		result.Set("possibleTypes", introspectionPossibleTypes(typed.Types))
	case *schema.Enum:
		result.Set("description", nullableString(typed.Description))
		result.Set("enumValues", introspectionEnumValues(typed.Values, includeDeprecated))
	}

	return result
}

func introspectionFields(fields map[string]*schema.Field, includeDeprecated bool) []any {
	names := sortedFieldNames(fields)
	values := make([]any, 0, len(names))
	for _, name := range names {
		field := fields[name]
		if field.IsDeprecated && !includeDeprecated {
			continue
		}
		entry := core.NewOrderedObject(6)
		entry.Set("name", field.Name)
		entry.Set("description", nullableString(field.Description))
		entry.Set("args", introspectionInputValues(field.Args))
		entry.Set("type", introspectionTypeRef(field.Type))
		entry.Set("isDeprecated", field.IsDeprecated)
		entry.Set("deprecationReason", field.DeprecationReason)
		values = append(values, entry)
	}
	return values
}

func introspectionInputValues(inputValues []schema.InputValue) []any {
	values := make([]any, 0, len(inputValues))
	for _, inputValue := range inputValues {
		entry := core.NewOrderedObject(4)
		entry.Set("name", inputValue.Name)
		entry.Set("description", nullableString(inputValue.Description))
		entry.Set("type", introspectionTypeRef(inputValue.Type))
		entry.Set("defaultValue", formatDefaultValue(inputValue.DefaultValue))
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
		entry.Set("description", nullableString(field.Description))
		entry.Set("type", introspectionTypeRef(field.Type))
		entry.Set("defaultValue", formatDefaultValue(field.DefaultValue))
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

func introspectionEnumValues(values []schema.EnumValue, includeDeprecated bool) []any {
	enumValues := make([]any, 0, len(values))
	for _, value := range values {
		if value.IsDeprecated && !includeDeprecated {
			continue
		}
		entry := core.NewOrderedObject(4)
		entry.Set("name", value.Name)
		entry.Set("description", nullableString(value.Description))
		entry.Set("isDeprecated", value.IsDeprecated)
		entry.Set("deprecationReason", value.DeprecationReason)
		enumValues = append(enumValues, entry)
	}
	return enumValues
}

func formatDefaultValue(v any) any {
	if s, ok := schema.FormatSDLDefault(v); ok {
		return s
	}
	return nil
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
