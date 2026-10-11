package schema

// registerIntrospectionTypes installs the standard introspection schema,
// including recursive type references and enum values used by schema clients.
func registerIntrospectionTypes(types map[string]Type) {
	for _, name := range []string{"__Schema", "__Type", "__Field", "__InputValue", "__EnumValue", "__Directive"} {
		types[name] = &Object{TypeName: name}
	}
	types["__TypeKind"] = introspectionEnum("__TypeKind", []string{"SCALAR", "OBJECT", "INTERFACE", "UNION", "ENUM", "INPUT_OBJECT", "LIST", "NON_NULL"})
	types["__DirectiveLocation"] = introspectionEnum("__DirectiveLocation", []string{"QUERY", "MUTATION", "SUBSCRIPTION", "FIELD", "FRAGMENT_DEFINITION", "FRAGMENT_SPREAD", "INLINE_FRAGMENT", "VARIABLE_DEFINITION", "SCHEMA", "SCALAR", "OBJECT", "FIELD_DEFINITION", "ARGUMENT_DEFINITION", "INTERFACE", "UNION", "ENUM", "ENUM_VALUE", "INPUT_OBJECT", "INPUT_FIELD_DEFINITION"})
	str := types["String"]
	boolean := types["Boolean"]
	nnString := &NonNull{OfType: str}
	nnBoolean := &NonNull{OfType: boolean}
	typ := types["__Type"]
	nnType := &NonNull{OfType: typ}
	typeList := &List{OfType: nnType}
	inputList := &NonNull{OfType: &List{OfType: &NonNull{OfType: types["__InputValue"]}}}
	types["__Schema"].(*Object).Fields = introspectionFieldMap([]*Field{
		{Name: "description", Type: str},
		{Name: "types", Type: &NonNull{OfType: typeList}},
		{Name: "queryType", Type: nnType},
		{Name: "mutationType", Type: typ},
		{Name: "subscriptionType", Type: typ},
		{Name: "directives", Type: &NonNull{OfType: &List{OfType: &NonNull{OfType: types["__Directive"]}}}},
	})
	types["__Type"].(*Object).Fields = introspectionFieldMap([]*Field{
		{Name: "kind", Type: &NonNull{OfType: types["__TypeKind"]}},
		{Name: "name", Type: str}, {Name: "description", Type: str}, {Name: "specifiedByURL", Type: str},
		introspectionDeprecatedField("fields", &List{OfType: &NonNull{OfType: types["__Field"]}}, boolean),
		{Name: "interfaces", Type: typeList}, {Name: "possibleTypes", Type: typeList},
		introspectionDeprecatedField("enumValues", &List{OfType: &NonNull{OfType: types["__EnumValue"]}}, boolean),
		introspectionDeprecatedField("inputFields", &List{OfType: &NonNull{OfType: types["__InputValue"]}}, boolean),
		{Name: "ofType", Type: typ}, {Name: "isOneOf", Type: boolean},
	})
	types["__Field"].(*Object).Fields = introspectionFieldMap([]*Field{
		{Name: "name", Type: nnString}, {Name: "description", Type: str},
		introspectionDeprecatedField("args", inputList, boolean),
		{Name: "type", Type: nnType}, {Name: "isDeprecated", Type: nnBoolean}, {Name: "deprecationReason", Type: str},
	})
	types["__InputValue"].(*Object).Fields = introspectionFieldMap([]*Field{
		{Name: "name", Type: nnString}, {Name: "description", Type: str}, {Name: "type", Type: nnType},
		{Name: "defaultValue", Type: str}, {Name: "isDeprecated", Type: nnBoolean}, {Name: "deprecationReason", Type: str},
	})
	types["__EnumValue"].(*Object).Fields = introspectionFieldMap([]*Field{
		{Name: "name", Type: nnString}, {Name: "description", Type: str}, {Name: "isDeprecated", Type: nnBoolean}, {Name: "deprecationReason", Type: str},
	})
	types["__Directive"].(*Object).Fields = introspectionFieldMap([]*Field{
		{Name: "name", Type: nnString}, {Name: "description", Type: str}, {Name: "isRepeatable", Type: nnBoolean},
		{Name: "locations", Type: &NonNull{OfType: &List{OfType: &NonNull{OfType: types["__DirectiveLocation"]}}}},
		introspectionDeprecatedField("args", inputList, boolean),
	})
}

func introspectionFieldMap(fields []*Field) map[string]*Field {
	result := make(map[string]*Field, len(fields))
	for _, field := range fields {
		result[field.Name] = field
	}
	return result
}

func introspectionDeprecatedField(name string, fieldType Type, boolean Type) *Field {
	return &Field{Name: name, Type: fieldType, Args: []InputValue{{Name: "includeDeprecated", Type: &NonNull{OfType: boolean}, DefaultValue: false}}}
}

func introspectionEnum(name string, names []string) *Enum {
	result := &Enum{TypeName: name, Values: make([]EnumValue, len(names)), valueByName: make(map[string]any, len(names)), nameByValue: make(map[string]string, len(names))}
	for index, value := range names {
		result.Values[index] = EnumValue{Name: value}
		result.valueByName[value] = value
		result.nameByValue[enumValueKey(value)] = value
	}
	return result
}
