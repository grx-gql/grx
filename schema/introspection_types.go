package schema

// registerIntrospectionTypes adds GraphQL introspection meta-types to the
// schema type registry so tools can resolve them by name.
func registerIntrospectionTypes(types map[string]Type) {
	for name, kind := range map[string]Kind{
		"__Schema":            ObjectKind,
		"__Type":              ObjectKind,
		"__Field":             ObjectKind,
		"__InputValue":        ObjectKind,
		"__EnumValue":         ObjectKind,
		"__Directive":         ObjectKind,
		"__TypeKind":          EnumKind,
		"__DirectiveLocation": EnumKind,
	} {
		switch kind {
		case ObjectKind:
			types[name] = &Object{TypeName: name, Fields: map[string]*Field{}}
		case EnumKind:
			types[name] = &Enum{TypeName: name, Values: []EnumValue{}}
		}
	}
}
