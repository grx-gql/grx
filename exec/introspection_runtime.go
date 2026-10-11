package exec

import (
	"context"
	"fmt"
	"maps"
	"sort"
	"strings"

	"github.com/grx-gql/grx/schema"
)

// executableIntrospectionSchema adds implicit root fields and meta-field
// resolvers to a private schema view without modifying the caller's schema.
func executableIntrospectionSchema(source *schema.Schema) *schema.Schema {
	if source == nil || source.Query == nil {
		return source
	}
	result := *source
	result.Types = maps.Clone(source.Types)
	if result.Types == nil {
		result.Types = make(map[string]schema.Type)
	}
	if _, ok := result.Types["__Schema"].(*schema.Object); !ok {
		for name, value := range schema.IntrospectionTypes() {
			if result.Types[name] == nil {
				result.Types[name] = value
			}
		}
	}
	metadata := maps.Clone(result.Types)
	metadata[source.Query.Name()] = source.Query
	for _, name := range []string{"__Schema", "__Type", "__Field", "__InputValue", "__EnumValue", "__Directive"} {
		original := metadata[name].(*schema.Object)
		object := *original
		object.Fields = make(map[string]*schema.Field, len(original.Fields))
		result.Types[name] = &object
	}
	for _, name := range []string{"__Schema", "__Type", "__Field", "__InputValue", "__EnumValue", "__Directive"} {
		original := metadata[name].(*schema.Object)
		object := result.Types[name].(*schema.Object)
		for name, originalField := range original.Fields {
			field := *originalField
			field.Type = introspectionRuntimeType(field.Type, result.Types)
			field.Resolver = introspectionRuntimeResolver(name)
			object.Fields[name] = &field
		}
	}
	schemaSource := *source
	schemaSource.Types = metadata
	query := *source.Query
	query.Fields = maps.Clone(query.Fields)
	if query.Fields == nil {
		query.Fields = make(map[string]*schema.Field)
	}
	query.Fields["__schema"] = &schema.Field{Name: "__schema", Type: &schema.NonNull{OfType: result.Types["__Schema"]}, Resolver: func(context.Context, schema.ResolveParams) (any, error) { return &schemaSource, nil }}
	query.Fields["__type"] = &schema.Field{Name: "__type", Type: result.Types["__Type"], Args: []schema.InputValue{{Name: "name", Type: &schema.NonNull{OfType: result.Types["String"]}}}, Resolver: func(_ context.Context, p schema.ResolveParams) (any, error) {
		return metadata[p.Args["name"].(string)], nil
	}}
	result.Query = &query
	result.Types[query.Name()] = &query
	return &result
}

func introspectionRuntimeType(value schema.Type, types map[string]schema.Type) schema.Type {
	switch typed := value.(type) {
	case *schema.NonNull:
		return &schema.NonNull{OfType: introspectionRuntimeType(typed.OfType, types)}
	case *schema.List:
		return &schema.List{OfType: introspectionRuntimeType(typed.OfType, types)}
	default:
		return types[value.Name()]
	}
}

func introspectionRuntimeResolver(name string) schema.Resolver {
	return func(_ context.Context, p schema.ResolveParams) (any, error) {
		return introspectionRuntimeValue(p.Source, name, p.Args)
	}
}

func introspectionRuntimeValue(source any, name string, args map[string]any) (any, error) {
	includeDeprecated, _ := args["includeDeprecated"].(bool)
	switch typed := source.(type) {
	case *schema.Schema:
		switch name {
		case "description":
			return nil, nil
		case "queryType":
			return typed.Query, nil
		case "mutationType":
			return typed.Mutation, nil
		case "subscriptionType":
			return typed.Subscription, nil
		case "types":
			names := make([]string, 0, len(typed.Types))
			for name := range typed.Types {
				names = append(names, name)
			}
			sort.Strings(names)
			values := make([]schema.Type, len(names))
			for index, name := range names {
				values[index] = typed.Types[name]
			}
			return values, nil
		case "directives":
			return introspectionDirectiveDefinitions(typed), nil
		}
	case schema.Type:
		return introspectionRuntimeTypeValue(typed, name, includeDeprecated)
	case *schema.Field:
		switch name {
		case "name":
			return typed.Name, nil
		case "description":
			return nullableString(typed.Description), nil
		case "args":
			return introspectionRuntimeArguments(typed.Args, includeDeprecated), nil
		case "type":
			return typed.Type, nil
		case "isDeprecated":
			return typed.IsDeprecated, nil
		case "deprecationReason":
			return typed.DeprecationReason, nil
		}
	case schema.InputValue:
		switch name {
		case "name":
			return typed.Name, nil
		case "description":
			return nullableString(typed.Description), nil
		case "type":
			return typed.Type, nil
		case "defaultValue":
			return introspectionDefaultValue(typed.Type, typed.DefaultValue), nil
		case "isDeprecated":
			return typed.IsDeprecated, nil
		case "deprecationReason":
			return typed.DeprecationReason, nil
		}
	case schema.EnumValue:
		switch name {
		case "name":
			return typed.Name, nil
		case "description":
			return nullableString(typed.Description), nil
		case "isDeprecated":
			return typed.IsDeprecated, nil
		case "deprecationReason":
			return typed.DeprecationReason, nil
		}
	case *schema.DirectiveDefinition:
		switch name {
		case "name":
			return typed.Name, nil
		case "description":
			return nullableString(typed.Description), nil
		case "isRepeatable":
			return typed.IsRepeatable, nil
		case "locations":
			return typed.Locations, nil
		case "args":
			return introspectionRuntimeArguments(typed.Args, includeDeprecated), nil
		}
	}
	return nil, fmt.Errorf("cannot resolve introspection field %q on %T", name, source)
}

func introspectionRuntimeTypeValue(value schema.Type, name string, includeDeprecated bool) (any, error) {
	switch name {
	case "kind":
		return string(value.Kind()), nil
	case "name":
		if value.Kind() == schema.ListKind || value.Kind() == schema.NonNullKind {
			return nil, nil
		}
		return value.Name(), nil
	case "ofType":
		switch typed := value.(type) {
		case *schema.List:
			return typed.OfType, nil
		case *schema.NonNull:
			return typed.OfType, nil
		}
		return nil, nil
	case "description":
		return nullableString(introspectionTypeDescription(value)), nil
	case "specifiedByURL":
		if typed, ok := value.(*schema.Scalar); ok {
			return nullableString(typed.SpecifiedByURL), nil
		}
		return nil, nil
	case "isOneOf":
		if typed, ok := value.(*schema.InputObject); ok {
			return typed.IsOneOf, nil
		}
		return nil, nil
	case "fields":
		var fields map[string]*schema.Field
		switch typed := value.(type) {
		case *schema.Object:
			fields = typed.Fields
		case *schema.Interface:
			fields = typed.Fields
		default:
			return nil, nil
		}
		values := make([]*schema.Field, 0, len(fields))
		for _, name := range sortedFieldNames(fields) {
			field := fields[name]
			if !strings.HasPrefix(name, "__") && (includeDeprecated || !field.IsDeprecated) {
				values = append(values, field)
			}
		}
		return values, nil
	case "interfaces":
		switch typed := value.(type) {
		case *schema.Object:
			return typed.Interfaces, nil
		case *schema.Interface:
			return typed.Interfaces, nil
		}
		return nil, nil
	case "possibleTypes":
		switch typed := value.(type) {
		case *schema.Union:
			return typed.Types, nil
		case *schema.Interface:
			return typed.PossibleTypes, nil
		}
		return nil, nil
	case "enumValues":
		typed, ok := value.(*schema.Enum)
		if !ok {
			return nil, nil
		}
		values := make([]schema.EnumValue, 0, len(typed.Values))
		for _, item := range typed.Values {
			if includeDeprecated || !item.IsDeprecated {
				values = append(values, item)
			}
		}
		return values, nil
	case "inputFields":
		typed, ok := value.(*schema.InputObject)
		if !ok {
			return nil, nil
		}
		values := make([]schema.InputValue, 0, len(typed.Fields))
		for _, name := range sortedFieldNames(typed.Fields) {
			field := typed.Fields[name]
			if includeDeprecated || !field.IsDeprecated {
				values = append(values, schema.InputValue{Name: field.Name, Description: field.Description, Type: field.Type, DefaultValue: field.DefaultValue, IsDeprecated: field.IsDeprecated, DeprecationReason: field.DeprecationReason})
			}
		}
		return values, nil
	}
	return nil, fmt.Errorf("unknown introspection type field %q", name)
}

func introspectionTypeDescription(value schema.Type) string {
	switch typed := value.(type) {
	case *schema.Scalar:
		return typed.Description
	case *schema.Object:
		return typed.Description
	case *schema.Interface:
		return typed.Description
	case *schema.Union:
		return typed.Description
	case *schema.Enum:
		return typed.Description
	case *schema.InputObject:
		return typed.Description
	default:
		return ""
	}
}

func introspectionRuntimeArguments(args []schema.InputValue, includeDeprecated bool) []schema.InputValue {
	values := make([]schema.InputValue, 0, len(args))
	for _, arg := range args {
		if includeDeprecated || !arg.IsDeprecated {
			values = append(values, arg)
		}
	}
	return values
}

func introspectionDirectiveDefinitions(s *schema.Schema) []*schema.DirectiveDefinition {
	values := introspectionBuiltinDirectiveDefinitions()
	builtinNames := make(map[string]bool, len(values))
	for index, definition := range values {
		builtinNames[definition.Name] = true
		if registered := s.DirectiveDefinitions[definition.Name]; registered != nil {
			values[index] = registered
		}
	}
	names := make([]string, 0, len(s.DirectiveDefinitions))
	for name := range s.DirectiveDefinitions {
		if !builtinNames[name] {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		values = append(values, s.DirectiveDefinitions[name])
	}
	return values
}

func introspectionBuiltinDirectiveDefinitions() []*schema.DirectiveDefinition {
	boolean := &schema.Scalar{TypeName: "Boolean"}
	str := &schema.Scalar{TypeName: "String"}
	integer := &schema.Scalar{TypeName: "Int"}
	return []*schema.DirectiveDefinition{
		{Name: "skip", Locations: []string{"FIELD", "FRAGMENT_SPREAD", "INLINE_FRAGMENT"}, Args: []schema.InputValue{{Name: "if", Type: &schema.NonNull{OfType: boolean}}}},
		{Name: "include", Locations: []string{"FIELD", "FRAGMENT_SPREAD", "INLINE_FRAGMENT"}, Args: []schema.InputValue{{Name: "if", Type: &schema.NonNull{OfType: boolean}}}},
		{Name: "deprecated", Locations: []string{"FIELD_DEFINITION", "ARGUMENT_DEFINITION", "INPUT_FIELD_DEFINITION", "ENUM_VALUE"}, Args: []schema.InputValue{{Name: "reason", Type: str, DefaultValue: "No longer supported"}}},
		{Name: "specifiedBy", Locations: []string{"SCALAR"}, Args: []schema.InputValue{{Name: "url", Type: &schema.NonNull{OfType: str}}}},
		{Name: "oneOf", Locations: []string{"INPUT_OBJECT"}},
		{Name: "defer", Locations: []string{"FRAGMENT_SPREAD", "INLINE_FRAGMENT", "FIELD"}, Args: []schema.InputValue{{Name: "if", Type: boolean, DefaultValue: true}, {Name: "label", Type: str}}},
		{Name: "stream", Locations: []string{"FIELD"}, Args: []schema.InputValue{{Name: "if", Type: boolean, DefaultValue: true}, {Name: "label", Type: str}, {Name: "initialCount", Type: integer, DefaultValue: 0}}},
	}
}

func selectionsIntrospect(selections []selection, fragments map[string]*fragmentDef, visited map[string]bool) bool {
	for _, selected := range selections {
		if selected.Name == "__schema" || selected.Name == "__type" {
			return true
		}
		if selectionsIntrospect(selected.Selections, fragments, visited) {
			return true
		}
		if name := selected.FragmentSpread; name != "" && !visited[name] {
			visited[name] = true
			if fragment := fragments[name]; fragment != nil && selectionsIntrospect(fragment.Selections, fragments, visited) {
				return true
			}
		}
	}
	return false
}
