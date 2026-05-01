package schema

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
)

var (
	stringScalar  = &Scalar{TypeName: "String"}
	intScalar     = &Scalar{TypeName: "Int"}
	floatScalar   = &Scalar{TypeName: "Float"}
	booleanScalar = &Scalar{TypeName: "Boolean"}
	idScalar      = &Scalar{TypeName: "ID"}
)

// Config holds the user-supplied root resolvers that [Build] reflects
// into a runtime [Schema]. Query is required; Mutation and Subscription
// are optional.
type Config struct {
	// Query is the root query resolver value. Its exported methods are
	// reflected into fields on the GraphQL Query root type.
	Query any
	// Mutation is the root mutation resolver value. Optional.
	Mutation any
	// Subscription is the root subscription resolver value. Optional.
	Subscription any
}

// Builder accumulates the type registry while a [Schema] is being
// assembled. It is not safe for concurrent use; construct one per Build
// call.
type Builder struct {
	types map[string]Type
}

// Build reflects the supplied resolver values into a runtime [Schema].
// It returns an error when the configuration is inconsistent (e.g.
// missing Query root, both Schema and root fields supplied) or when a
// resolver cannot be reflected into the GraphQL type system.
func Build(config Config) (*Schema, error) {
	builder := Builder{types: builtinScalars()}
	result := &Schema{Types: builder.types}

	if config.Query != nil {
		query, err := builder.buildRootObject("Query", config.Query)
		if err != nil {
			return nil, err
		}
		result.Query = query
	}

	if config.Mutation != nil {
		mutation, err := builder.buildRootObject("Mutation", config.Mutation)
		if err != nil {
			return nil, err
		}
		result.Mutation = mutation
	}

	if config.Subscription != nil {
		subscription, err := builder.buildRootObject("Subscription", config.Subscription)
		if err != nil {
			return nil, err
		}
		result.Subscription = subscription
	}

	if result.Query == nil {
		return nil, errors.New("schema requires a query root")
	}

	return result, nil
}

func builtinScalars() map[string]Type {
	return map[string]Type{
		"String":  stringScalar,
		"Int":     intScalar,
		"Float":   floatScalar,
		"Boolean": booleanScalar,
		"ID":      idScalar,
	}
}

func (b *Builder) buildRootObject(name string, value any) (*Object, error) {
	object := &Object{TypeName: name, Fields: map[string]*Field{}}
	valueType := reflect.TypeOf(value)

	for index := 0; index < valueType.NumMethod(); index++ {
		method := valueType.Method(index)
		field, err := b.buildMethodField(value, method)
		if err != nil {
			return nil, err
		}
		object.Fields[field.Name] = field
	}

	b.types[name] = object
	return object, nil
}

func (b *Builder) buildMethodField(receiver any, method reflect.Method) (*Field, error) {
	methodType := method.Type
	if methodType.NumOut() == 0 || methodType.NumOut() > 2 {
		return nil, fmt.Errorf("resolver %s must return value or (value, error)", method.Name)
	}
	if methodType.NumOut() == 2 && !methodType.Out(1).Implements(reflect.TypeOf((*error)(nil)).Elem()) {
		return nil, fmt.Errorf("resolver %s second return must be error", method.Name)
	}

	argsType, err := resolverArgsType(method)
	if err != nil {
		return nil, err
	}

	returnType := methodType.Out(0)
	if returnType.Kind() == reflect.Chan {
		returnType = returnType.Elem()
	}

	outputType, err := b.graphQLType(returnType)
	if err != nil {
		return nil, fmt.Errorf("resolver %s return type: %w", method.Name, err)
	}
	args, err := b.graphQLArgs(argsType)
	if err != nil {
		return nil, fmt.Errorf("resolver %s args: %w", method.Name, err)
	}

	fieldName := lowerFirst(method.Name)
	field := &Field{
		Name: fieldName,
		Type: outputType,
		Args: args,
		Resolver: func(ctx context.Context, params ResolveParams) (any, error) {
			return callResolver(ctx, receiver, method, argsType, params.Args)
		},
	}

	return field, nil
}

func resolverArgsType(method reflect.Method) (reflect.Type, error) {
	methodType := method.Type
	if methodType.NumIn() < 1 || methodType.NumIn() > 3 {
		return nil, fmt.Errorf("resolver %s must accept receiver, optional context, and optional args", method.Name)
	}

	nextIndex := 1
	if methodType.NumIn() > nextIndex && methodType.In(nextIndex) == reflect.TypeOf((*context.Context)(nil)).Elem() {
		nextIndex++
	}

	if methodType.NumIn() == nextIndex {
		return nil, nil
	}

	argsType := methodType.In(nextIndex)
	if argsType.Kind() != reflect.Struct {
		return nil, fmt.Errorf("resolver %s args must be a struct", method.Name)
	}
	if methodType.NumIn() != nextIndex+1 {
		return nil, fmt.Errorf("resolver %s has unsupported parameters", method.Name)
	}

	return argsType, nil
}

func callResolver(ctx context.Context, receiver any, method reflect.Method, argsType reflect.Type, args map[string]any) (any, error) {
	values := []reflect.Value{reflect.ValueOf(receiver)}
	if method.Type.NumIn() > 1 && method.Type.In(1) == reflect.TypeOf((*context.Context)(nil)).Elem() {
		values = append(values, reflect.ValueOf(ctx))
	}
	if argsType != nil {
		argValue, err := buildArgs(argsType, args)
		if err != nil {
			return nil, err
		}
		values = append(values, argValue)
	}

	results := method.Func.Call(values)
	if len(results) == 2 && !results[1].IsNil() {
		return nil, results[1].Interface().(error)
	}

	return results[0].Interface(), nil
}

func buildArgs(argsType reflect.Type, args map[string]any) (reflect.Value, error) {
	value := reflect.New(argsType).Elem()
	for index := 0; index < argsType.NumField(); index++ {
		field := argsType.Field(index)
		argName, required := parseTag(field)
		if argName == "" {
			argName = lowerFirst(field.Name)
		}

		raw, exists := args[argName]
		if !exists {
			if required {
				return reflect.Value{}, fmt.Errorf("missing required argument %q", argName)
			}
			continue
		}

		if err := setValue(value.Field(index), raw); err != nil {
			return reflect.Value{}, fmt.Errorf("argument %q: %w", argName, err)
		}
	}

	return value, nil
}

func setValue(target reflect.Value, raw any) error {
	if raw == nil {
		return nil
	}
	if target.Kind() == reflect.Pointer {
		value := reflect.New(target.Type().Elem())
		if err := setValue(value.Elem(), raw); err != nil {
			return err
		}
		target.Set(value)
		return nil
	}
	if target.Kind() == reflect.Struct {
		fields, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("cannot assign %T to %s", raw, target.Type().String())
		}
		inputValue, err := buildArgs(target.Type(), fields)
		if err != nil {
			return err
		}
		target.Set(inputValue)
		return nil
	}

	rawValue := reflect.ValueOf(raw)
	if rawValue.Type().AssignableTo(target.Type()) {
		target.Set(rawValue)
		return nil
	}
	if rawValue.Type().ConvertibleTo(target.Type()) {
		target.Set(rawValue.Convert(target.Type()))
		return nil
	}

	return fmt.Errorf("cannot assign %T to %s", raw, target.Type().String())
}

func (b *Builder) graphQLArgs(argsType reflect.Type) ([]InputValue, error) {
	if argsType == nil {
		return []InputValue{}, nil
	}

	args := make([]InputValue, 0, argsType.NumField())
	for index := 0; index < argsType.NumField(); index++ {
		field := argsType.Field(index)
		if field.PkgPath != "" {
			continue
		}

		argName, required := parseTag(field)
		if argName == "-" {
			continue
		}
		if argName == "" {
			argName = lowerFirst(field.Name)
		}

		argType, err := b.graphQLInputType(field.Type)
		if err != nil {
			return nil, fmt.Errorf("field %s.%s: %w", argsType.Name(), field.Name, err)
		}
		if required {
			argType = &NonNull{OfType: argType}
		}

		args = append(args, InputValue{Name: argName, Type: argType})
	}
	return args, nil
}

func (b *Builder) graphQLInputType(goType reflect.Type) (Type, error) {
	if goType.Kind() == reflect.Pointer {
		return b.graphQLInputType(goType.Elem())
	}
	if goType.Kind() == reflect.Slice || goType.Kind() == reflect.Array {
		itemType, err := b.graphQLInputType(goType.Elem())
		if err != nil {
			return nil, err
		}
		return &List{OfType: itemType}, nil
	}

	switch goType.Kind() {
	case reflect.String:
		return stringScalar, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return intScalar, nil
	case reflect.Float32, reflect.Float64:
		return floatScalar, nil
	case reflect.Bool:
		return booleanScalar, nil
	case reflect.Struct:
		return b.buildInputObject(goType)
	default:
		return nil, fmt.Errorf("unsupported Go input type %s", goType.String())
	}
}

func (b *Builder) graphQLType(goType reflect.Type) (Type, error) {
	if goType.Kind() == reflect.Pointer {
		return b.graphQLType(goType.Elem())
	}
	if goType.Kind() == reflect.Slice || goType.Kind() == reflect.Array {
		itemType, err := b.graphQLType(goType.Elem())
		if err != nil {
			return nil, err
		}
		return &List{OfType: itemType}, nil
	}

	switch goType.Kind() {
	case reflect.String:
		return stringScalar, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return intScalar, nil
	case reflect.Float32, reflect.Float64:
		return floatScalar, nil
	case reflect.Bool:
		return booleanScalar, nil
	case reflect.Struct:
		return b.buildObject(goType)
	default:
		return nil, fmt.Errorf("unsupported Go type %s", goType.String())
	}
}

func (b *Builder) buildObject(goType reflect.Type) (*Object, error) {
	name := goType.Name()
	if existing, ok := b.types[name]; ok {
		object, ok := existing.(*Object)
		if !ok {
			return nil, fmt.Errorf("type name %q already registered as %s", name, existing.Kind())
		}
		return object, nil
	}

	object := &Object{TypeName: name, Fields: map[string]*Field{}}
	b.types[name] = object

	for index := 0; index < goType.NumField(); index++ {
		structField := goType.Field(index)
		if structField.PkgPath != "" {
			continue
		}

		fieldName, required := parseTag(structField)
		if fieldName == "-" {
			continue
		}
		if fieldName == "" {
			fieldName = lowerFirst(structField.Name)
		}

		fieldType, err := b.graphQLType(structField.Type)
		if err != nil {
			return nil, fmt.Errorf("field %s.%s: %w", goType.Name(), structField.Name, err)
		}
		if required {
			fieldType = &NonNull{OfType: fieldType}
		}

		fieldIndex := index
		object.Fields[fieldName] = &Field{
			Name: fieldName,
			Type: fieldType,
			Resolver: func(ctx context.Context, params ResolveParams) (any, error) {
				source := reflect.Indirect(reflect.ValueOf(params.Source))
				if !source.IsValid() {
					return nil, fmt.Errorf("field %q received nil source", fieldName)
				}
				return source.Field(fieldIndex).Interface(), nil
			},
		}
	}

	return object, nil
}

func (b *Builder) buildInputObject(goType reflect.Type) (*InputObject, error) {
	name := goType.Name()
	if name == "" {
		return nil, fmt.Errorf("input object type %s must be named", goType.String())
	}

	if existing, ok := b.types[name]; ok {
		inputObject, ok := existing.(*InputObject)
		if !ok {
			return nil, fmt.Errorf("type name %q already registered as %s", name, existing.Kind())
		}
		return inputObject, nil
	}

	inputObject := &InputObject{TypeName: name, Fields: map[string]*Field{}}
	b.types[name] = inputObject

	for index := 0; index < goType.NumField(); index++ {
		structField := goType.Field(index)
		if structField.PkgPath != "" {
			continue
		}

		fieldName, required := parseTag(structField)
		if fieldName == "-" {
			continue
		}
		if fieldName == "" {
			fieldName = lowerFirst(structField.Name)
		}

		fieldType, err := b.graphQLInputType(structField.Type)
		if err != nil {
			return nil, fmt.Errorf("field %s.%s: %w", goType.Name(), structField.Name, err)
		}
		if required {
			fieldType = &NonNull{OfType: fieldType}
		}

		inputObject.Fields[fieldName] = &Field{
			Name: fieldName,
			Type: fieldType,
		}
	}

	return inputObject, nil
}

func parseTag(field reflect.StructField) (string, bool) {
	tag := field.Tag.Get("gql")
	if tag == "" {
		return "", false
	}

	parts := strings.Split(tag, ",")
	name := strings.TrimSpace(parts[0])
	required := false
	for _, part := range parts[1:] {
		if strings.TrimSpace(part) == "nonNull" {
			required = true
		}
	}

	return name, required
}

func lowerFirst(value string) string {
	if value == "" {
		return value
	}
	return strings.ToLower(value[:1]) + value[1:]
}
