package schema

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"
)

var (
	stringScalar  = &Scalar{TypeName: "String"}
	intScalar     = &Scalar{TypeName: "Int"}
	floatScalar   = &Scalar{TypeName: "Float"}
	booleanScalar = &Scalar{TypeName: "Boolean"}
	idScalar      = &Scalar{TypeName: "ID"}
)

// ScalarConfig registers a custom scalar for a Go type.
type ScalarConfig struct {
	Type           any
	Name           string
	SpecifiedByURL string
	Parse          func(input any) (any, error)
	Serialize      func(value any) (any, error)
}

// EnumValueConfig registers one GraphQL enum member and its Go value.
type EnumValueConfig struct {
	Name  string
	Value any
}

// EnumConfig registers an enum for a named Go type.
type EnumConfig struct {
	Type   any
	Name   string
	Values []EnumValueConfig
}

// InterfaceConfig registers a GraphQL interface for a Go interface type.
type InterfaceConfig struct {
	Type         any
	Implementors []any
}

// UnionConfig registers a GraphQL union for a Go interface type.
type UnionConfig struct {
	Type         any
	Name         string
	Implementors []any
}

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
	// Scalars registers custom scalars by Go type.
	Scalars []ScalarConfig
	// Enums registers enums by Go type.
	Enums []EnumConfig
	// Interfaces registers GraphQL interfaces by Go interface type.
	Interfaces []InterfaceConfig
	// Unions registers GraphQL unions by Go interface type.
	Unions []UnionConfig
}

type customScalar struct {
	scalar *Scalar
	parse  func(input any) (any, error)
}

// Builder accumulates the type registry while a [Schema] is being
// assembled. It is not safe for concurrent use; construct one per Build
// call.
type Builder struct {
	types        map[string]Type
	scalars      map[reflect.Type]customScalar
	enums        map[reflect.Type]*Enum
	interfaces   map[reflect.Type]*Interface
	unions       map[reflect.Type]*Union
	implementors map[reflect.Type][]reflect.Type
}

type tagOptions struct {
	name              string
	required          bool
	hasDefault        bool
	defaultValue      string
	deprecated        bool
	deprecationReason string
	description       string
}

// Build reflects the supplied resolver values into a runtime [Schema].
// It returns an error when the configuration is inconsistent (e.g.
// missing Query root, both Schema and root fields supplied) or when a
// resolver cannot be reflected into the GraphQL type system.
func Build(config Config) (*Schema, error) {
	builder, err := newBuilder(config)
	if err != nil {
		return nil, err
	}

	result := &Schema{Types: builder.types}

	if config.Query != nil {
		query, buildErr := builder.buildRootObject("Query", config.Query)
		if buildErr != nil {
			return nil, buildErr
		}
		result.Query = query
	}

	if config.Mutation != nil {
		mutation, buildErr := builder.buildRootObject("Mutation", config.Mutation)
		if buildErr != nil {
			return nil, buildErr
		}
		result.Mutation = mutation
	}

	if config.Subscription != nil {
		subscription, buildErr := builder.buildRootObject("Subscription", config.Subscription)
		if buildErr != nil {
			return nil, buildErr
		}
		result.Subscription = subscription
	}

	if result.Query == nil {
		return nil, errors.New("schema requires a query root")
	}

	registerIntrospectionTypes(builder.types)
	return result, nil
}

func newBuilder(config Config) (*Builder, error) {
	builder := &Builder{
		types:        builtinScalars(),
		scalars:      map[reflect.Type]customScalar{},
		enums:        map[reflect.Type]*Enum{},
		interfaces:   map[reflect.Type]*Interface{},
		unions:       map[reflect.Type]*Union{},
		implementors: map[reflect.Type][]reflect.Type{},
	}

	if err := builder.registerScalars(config.Scalars); err != nil {
		return nil, err
	}
	if err := builder.registerEnums(config.Enums); err != nil {
		return nil, err
	}
	if err := builder.registerInterfaces(config.Interfaces); err != nil {
		return nil, err
	}
	if err := builder.registerUnions(config.Unions); err != nil {
		return nil, err
	}

	return builder, nil
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

func (b *Builder) registerScalars(configs []ScalarConfig) error {
	for _, config := range configs {
		goType, err := namedTypeOf(config.Type)
		if err != nil {
			return fmt.Errorf("scalar %q: %w", config.Name, err)
		}
		if config.Name == "" {
			return fmt.Errorf("scalar %s must have a GraphQL name", goType.String())
		}
		if config.Parse == nil {
			return fmt.Errorf("scalar %q must define Parse", config.Name)
		}
		scalar := &Scalar{
			TypeName:       config.Name,
			SpecifiedByURL: config.SpecifiedByURL,
			serializeFn:    config.Serialize,
		}
		b.types[config.Name] = scalar
		b.scalars[goType] = customScalar{scalar: scalar, parse: config.Parse}
	}
	return nil
}

func (b *Builder) registerEnums(configs []EnumConfig) error {
	for _, config := range configs {
		goType, err := namedTypeOf(config.Type)
		if err != nil {
			return fmt.Errorf("enum %q: %w", config.Name, err)
		}
		if config.Name == "" {
			return fmt.Errorf("enum %s must have a GraphQL name", goType.String())
		}
		if len(config.Values) == 0 {
			return fmt.Errorf("enum %q must define at least one value", config.Name)
		}

		enumType := &Enum{
			TypeName:    config.Name,
			Values:      make([]EnumValue, 0, len(config.Values)),
			valueByName: map[string]any{},
			nameByValue: map[string]string{},
		}
		for _, value := range config.Values {
			enumType.Values = append(enumType.Values, EnumValue{Name: value.Name})
			enumType.valueByName[value.Name] = value.Value
			enumType.nameByValue[enumValueKey(value.Value)] = value.Name
		}

		b.types[config.Name] = enumType
		b.enums[goType] = enumType
	}
	return nil
}

func (b *Builder) registerInterfaces(configs []InterfaceConfig) error {
	for _, config := range configs {
		interfaceType, err := interfaceTypeOf(config.Type)
		if err != nil {
			return err
		}

		graphqlInterface := &Interface{
			TypeName:      interfaceType.Name(),
			Fields:        map[string]*Field{},
			PossibleTypes: make([]*Object, 0, len(config.Implementors)),
		}
		b.types[graphqlInterface.TypeName] = graphqlInterface
		b.interfaces[interfaceType] = graphqlInterface

		for _, implementor := range config.Implementors {
			goType, typeErr := namedTypeOf(implementor)
			if typeErr != nil {
				return fmt.Errorf("interface %q implementor: %w", graphqlInterface.TypeName, typeErr)
			}
			b.implementors[interfaceType] = append(b.implementors[interfaceType], goType)
		}
	}
	return nil
}

func (b *Builder) registerUnions(configs []UnionConfig) error {
	for _, config := range configs {
		interfaceType, err := interfaceTypeOf(config.Type)
		if err != nil {
			return err
		}

		name := config.Name
		if name == "" {
			name = interfaceType.Name()
		}
		if name == "" {
			return fmt.Errorf("union %s must have a GraphQL name", interfaceType.String())
		}

		unionType := &Union{TypeName: name, Types: make([]*Object, 0, len(config.Implementors))}
		b.types[unionType.TypeName] = unionType
		b.unions[interfaceType] = unionType

		for _, implementor := range config.Implementors {
			goType, typeErr := namedTypeOf(implementor)
			if typeErr != nil {
				return fmt.Errorf("union %q implementor: %w", unionType.TypeName, typeErr)
			}
			b.implementors[interfaceType] = append(b.implementors[interfaceType], goType)
		}
	}
	return nil
}

func namedTypeOf(value any) (reflect.Type, error) {
	if value == nil {
		return nil, errors.New("type cannot be nil")
	}

	goType := reflect.TypeOf(value)
	for goType.Kind() == reflect.Pointer {
		goType = goType.Elem()
	}
	if goType.Name() == "" {
		return nil, fmt.Errorf("type %s must be named", goType.String())
	}
	return goType, nil
}

func interfaceTypeOf(value any) (reflect.Type, error) {
	if value == nil {
		return nil, errors.New("interface type cannot be nil")
	}

	goType := reflect.TypeOf(value)
	if goType.Kind() == reflect.Pointer {
		goType = goType.Elem()
	}
	if goType.Kind() != reflect.Interface {
		return nil, fmt.Errorf("type %s must be an interface", goType.String())
	}
	if goType.Name() == "" {
		return nil, fmt.Errorf("interface type %s must be named", goType.String())
	}
	return goType, nil
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
		Name:       fieldName,
		Type:       outputType,
		Args:       args,
		ArgsByName: inputValueMap(args),
		Resolver: func(ctx context.Context, params ResolveParams) (any, error) {
			return b.callResolver(ctx, receiver, method, argsType, params.Args)
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

func (b *Builder) callResolver(ctx context.Context, receiver any, method reflect.Method, argsType reflect.Type, args map[string]any) (any, error) {
	values := []reflect.Value{reflect.ValueOf(receiver)}
	if method.Type.NumIn() > 1 && method.Type.In(1) == reflect.TypeOf((*context.Context)(nil)).Elem() {
		values = append(values, reflect.ValueOf(ctx))
	}
	if argsType != nil {
		argValue, err := b.buildArgs(argsType, args)
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

func (b *Builder) buildArgs(argsType reflect.Type, args map[string]any) (reflect.Value, error) {
	value := reflect.New(argsType).Elem()
	for index := 0; index < argsType.NumField(); index++ {
		field := argsType.Field(index)
		if field.PkgPath != "" {
			continue
		}

		options := parseTag(field)
		if options.name == "-" {
			continue
		}

		argName := options.name
		if argName == "" {
			argName = lowerFirst(field.Name)
		}

		raw, exists := args[argName]
		if !exists {
			if options.hasDefault {
				defaultValue, err := b.parseDefaultValue(field.Type, options.defaultValue)
				if err != nil {
					return reflect.Value{}, fmt.Errorf("argument %q default: %w", argName, err)
				}
				if err := b.setValue(value.Field(index), defaultValue); err != nil {
					return reflect.Value{}, fmt.Errorf("argument %q default: %w", argName, err)
				}
				continue
			}
			if options.required {
				return reflect.Value{}, fmt.Errorf("missing required argument %q", argName)
			}
			continue
		}

		if err := b.setValue(value.Field(index), raw); err != nil {
			return reflect.Value{}, fmt.Errorf("argument %q: %w", argName, err)
		}
	}

	return value, nil
}

func (b *Builder) setValue(target reflect.Value, raw any) error {
	if raw == nil {
		return nil
	}
	if target.Kind() == reflect.Pointer {
		value := reflect.New(target.Type().Elem())
		if err := b.setValue(value.Elem(), raw); err != nil {
			return err
		}
		target.Set(value)
		return nil
	}

	if enumType, ok := b.enums[target.Type()]; ok {
		value, err := enumType.Parse(raw)
		if err != nil {
			return err
		}
		target.Set(reflect.ValueOf(value).Convert(target.Type()))
		return nil
	}
	if scalar, ok := b.scalars[target.Type()]; ok {
		rawValue := reflect.ValueOf(raw)
		if rawValue.IsValid() && rawValue.Type().AssignableTo(target.Type()) {
			target.Set(rawValue)
			return nil
		}
		value, err := scalar.parse(raw)
		if err != nil {
			return err
		}
		parsedValue := reflect.ValueOf(value)
		if !parsedValue.Type().AssignableTo(target.Type()) && !parsedValue.Type().ConvertibleTo(target.Type()) {
			return fmt.Errorf("cannot assign %T to %s", value, target.Type().String())
		}
		if parsedValue.Type().AssignableTo(target.Type()) {
			target.Set(parsedValue)
			return nil
		}
		target.Set(parsedValue.Convert(target.Type()))
		return nil
	}

	rawValue := reflect.ValueOf(raw)
	if rawValue.IsValid() {
		if rawValue.Type().AssignableTo(target.Type()) {
			target.Set(rawValue)
			return nil
		}
		if setScalarValue(target, rawValue) {
			return nil
		}
	}

	if target.Kind() == reflect.Slice || target.Kind() == reflect.Array {
		list, ok := raw.([]any)
		if !ok {
			return fmt.Errorf("cannot assign %T to %s", raw, target.Type().String())
		}
		elemType := target.Type().Elem()
		slice := reflect.MakeSlice(target.Type(), len(list), len(list))
		for index, item := range list {
			elem := reflect.New(elemType).Elem()
			if err := b.setValue(elem, item); err != nil {
				return err
			}
			slice.Index(index).Set(elem)
		}
		target.Set(slice)
		return nil
	}

	if target.Kind() == reflect.Struct {
		fields, ok := raw.(map[string]any)
		if !ok {
			return fmt.Errorf("cannot assign %T to %s", raw, target.Type().String())
		}
		inputValue, err := b.buildArgs(target.Type(), fields)
		if err != nil {
			return err
		}
		target.Set(inputValue)
		return nil
	}

	return fmt.Errorf("cannot assign %T to %s", raw, target.Type().String())
}

func setScalarValue(target reflect.Value, raw reflect.Value) bool {
	switch target.Kind() {
	case reflect.String:
		if raw.Kind() != reflect.String {
			return false
		}
		target.SetString(raw.String())
		return true
	case reflect.Bool:
		if raw.Kind() != reflect.Bool {
			return false
		}
		target.SetBool(raw.Bool())
		return true
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if !isSignedIntegerKind(raw.Kind()) {
			return false
		}
		value := raw.Int()
		if target.OverflowInt(value) {
			return false
		}
		target.SetInt(value)
		return true
	case reflect.Float32, reflect.Float64:
		switch raw.Kind() {
		case reflect.Float32, reflect.Float64:
			value := raw.Float()
			if target.OverflowFloat(value) {
				return false
			}
			target.SetFloat(value)
			return true
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			target.SetFloat(float64(raw.Int()))
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func isSignedIntegerKind(kind reflect.Kind) bool {
	switch kind {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return true
	default:
		return false
	}
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

		options := parseTag(field)
		if options.name == "-" {
			continue
		}

		argName := options.name
		if argName == "" {
			argName = lowerFirst(field.Name)
		}

		argType, err := b.graphQLInputType(field.Type)
		if err != nil {
			return nil, fmt.Errorf("field %s.%s: %w", argsType.Name(), field.Name, err)
		}
		if options.required {
			argType = &NonNull{OfType: argType}
		}

		defaultValue, err := b.defaultValue(field.Type, options)
		if err != nil {
			return nil, fmt.Errorf("field %s.%s default: %w", argsType.Name(), field.Name, err)
		}

		args = append(args, InputValue{Name: argName, Type: argType, DefaultValue: defaultValue})
	}
	return args, nil
}

func inputValueMap(values []InputValue) map[string]InputValue {
	if len(values) == 0 {
		return nil
	}
	out := make(map[string]InputValue, len(values))
	for _, value := range values {
		out[value.Name] = value
	}
	return out
}

func (b *Builder) graphQLInputType(goType reflect.Type) (Type, error) {
	baseType := goType
	for baseType.Kind() == reflect.Pointer {
		baseType = baseType.Elem()
	}

	if enumType, ok := b.enums[baseType]; ok {
		return enumType, nil
	}
	if scalar, ok := b.scalars[baseType]; ok {
		return scalar.scalar, nil
	}
	if baseType.Kind() == reflect.Slice || baseType.Kind() == reflect.Array {
		itemType, err := b.graphQLInputType(baseType.Elem())
		if err != nil {
			return nil, err
		}
		return &List{OfType: itemType}, nil
	}

	switch baseType.Kind() {
	case reflect.String:
		return stringScalar, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return intScalar, nil
	case reflect.Float32, reflect.Float64:
		return floatScalar, nil
	case reflect.Bool:
		return booleanScalar, nil
	case reflect.Struct:
		return b.buildInputObject(baseType)
	default:
		return nil, fmt.Errorf("unsupported Go input type %s", baseType.String())
	}
}

func (b *Builder) graphQLType(goType reflect.Type) (Type, error) {
	baseType := goType
	for baseType.Kind() == reflect.Pointer {
		baseType = baseType.Elem()
	}

	if enumType, ok := b.enums[baseType]; ok {
		return enumType, nil
	}
	if scalar, ok := b.scalars[baseType]; ok {
		return scalar.scalar, nil
	}
	if interfaceType, ok := b.interfaces[baseType]; ok {
		if err := b.populateInterface(interfaceType, baseType); err != nil {
			return nil, err
		}
		return interfaceType, nil
	}
	if unionType, ok := b.unions[baseType]; ok {
		if err := b.populateUnion(unionType, baseType); err != nil {
			return nil, err
		}
		return unionType, nil
	}
	if baseType.Kind() == reflect.Slice || baseType.Kind() == reflect.Array {
		itemType, err := b.graphQLType(baseType.Elem())
		if err != nil {
			return nil, err
		}
		return &List{OfType: itemType}, nil
	}

	switch baseType.Kind() {
	case reflect.String:
		return stringScalar, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return intScalar, nil
	case reflect.Float32, reflect.Float64:
		return floatScalar, nil
	case reflect.Bool:
		return booleanScalar, nil
	case reflect.Struct:
		return b.buildObject(baseType)
	default:
		return nil, fmt.Errorf("unsupported Go type %s", baseType.String())
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

	object := &Object{TypeName: name, Fields: map[string]*Field{}, goType: goType}
	b.types[name] = object

	for index := 0; index < goType.NumField(); index++ {
		structField := goType.Field(index)
		if structField.PkgPath != "" {
			continue
		}

		options := parseTag(structField)
		if options.name == "-" {
			continue
		}

		fieldName := options.name
		if fieldName == "" {
			fieldName = lowerFirst(structField.Name)
		}

		fieldType, err := b.graphQLType(structField.Type)
		if err != nil {
			return nil, fmt.Errorf("field %s.%s: %w", goType.Name(), structField.Name, err)
		}
		if options.required {
			fieldType = &NonNull{OfType: fieldType}
		}

		fieldIndex := index
		f := &Field{
			Name:        fieldName,
			Type:        fieldType,
			Description: options.description,
			Resolver: func(ctx context.Context, params ResolveParams) (any, error) {
				source := reflect.Indirect(reflect.ValueOf(params.Source))
				if !source.IsValid() {
					return nil, fmt.Errorf("field %q received nil source", fieldName)
				}
				return source.Field(fieldIndex).Interface(), nil
			},
		}
		if options.deprecated {
			f.IsDeprecated = true
			if options.deprecationReason != "" {
				f.DeprecationReason = &options.deprecationReason
			}
		}
		object.Fields[fieldName] = f
	}

	for interfaceType, implementors := range b.implementors {
		if !containsType(implementors, goType) {
			continue
		}
		if graphqlInterface, ok := b.interfaces[interfaceType]; ok {
			if err := b.populateInterface(graphqlInterface, interfaceType); err != nil {
				return nil, err
			}
			object.Interfaces = append(object.Interfaces, graphqlInterface)
		}
		if graphqlUnion, ok := b.unions[interfaceType]; ok {
			if err := b.populateUnion(graphqlUnion, interfaceType); err != nil {
				return nil, err
			}
		}
	}

	return object, nil
}

func (b *Builder) populateInterface(interfaceType *Interface, goType reflect.Type) error {
	if len(interfaceType.PossibleTypes) > 0 {
		return nil
	}

	implementors := b.implementors[goType]
	commonFields := map[string]Type{}
	initialized := false

	for _, implementor := range implementors {
		objectType, err := b.buildObject(implementor)
		if err != nil {
			return err
		}
		interfaceType.PossibleTypes = append(interfaceType.PossibleTypes, objectType)

		if !initialized {
			for name, field := range objectType.Fields {
				commonFields[name] = field.Type
			}
			initialized = true
			continue
		}

		for name, fieldType := range commonFields {
			objectField, ok := objectType.Fields[name]
			if !ok || objectField.Type.Name() != fieldType.Name() {
				delete(commonFields, name)
			}
		}
	}

	for name, fieldType := range commonFields {
		interfaceType.Fields[name] = &Field{Name: name, Type: fieldType}
	}

	return nil
}

func (b *Builder) populateUnion(unionType *Union, goType reflect.Type) error {
	if len(unionType.Types) > 0 {
		return nil
	}

	for _, implementor := range b.implementors[goType] {
		objectType, err := b.buildObject(implementor)
		if err != nil {
			return err
		}
		unionType.Types = append(unionType.Types, objectType)
	}

	return nil
}

func containsType(values []reflect.Type, target reflect.Type) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
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

		options := parseTag(structField)
		if options.name == "-" {
			continue
		}

		fieldName := options.name
		if fieldName == "" {
			fieldName = lowerFirst(structField.Name)
		}

		fieldType, err := b.graphQLInputType(structField.Type)
		if err != nil {
			return nil, fmt.Errorf("field %s.%s: %w", goType.Name(), structField.Name, err)
		}
		if options.required {
			fieldType = &NonNull{OfType: fieldType}
		}

		defaultValue, err := b.defaultValue(structField.Type, options)
		if err != nil {
			return nil, fmt.Errorf("field %s.%s default: %w", goType.Name(), structField.Name, err)
		}

		inputObject.Fields[fieldName] = &Field{
			Name:         fieldName,
			Type:         fieldType,
			DefaultValue: defaultValue,
		}
	}

	return inputObject, nil
}

func (b *Builder) defaultValue(goType reflect.Type, options tagOptions) (any, error) {
	if !options.hasDefault {
		return nil, nil
	}
	return b.parseDefaultValue(goType, options.defaultValue)
}

func (b *Builder) parseDefaultValue(goType reflect.Type, raw string) (any, error) {
	baseType := goType
	for baseType.Kind() == reflect.Pointer {
		baseType = baseType.Elem()
	}

	if enumType, ok := b.enums[baseType]; ok {
		return enumType.Parse(raw)
	}
	if scalar, ok := b.scalars[baseType]; ok {
		return scalar.parse(raw)
	}

	switch baseType.Kind() {
	case reflect.String:
		return raw, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, err
		}
		return reflect.ValueOf(value).Convert(baseType).Interface(), nil
	case reflect.Float32, reflect.Float64:
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, err
		}
		return reflect.ValueOf(value).Convert(baseType).Interface(), nil
	case reflect.Bool:
		return strconv.ParseBool(raw)
	default:
		return nil, fmt.Errorf("unsupported default for type %s", baseType.String())
	}
}

func parseTag(field reflect.StructField) tagOptions {
	tag := field.Tag.Get("gql")
	if tag == "" {
		return tagOptions{}
	}

	parts := strings.Split(tag, ",")
	options := tagOptions{name: strings.TrimSpace(parts[0])}
	for _, part := range parts[1:] {
		trimmed := strings.TrimSpace(part)
		if trimmed == "nonNull" {
			options.required = true
			continue
		}
		if strings.HasPrefix(trimmed, "default=") {
			options.hasDefault = true
			options.defaultValue = strings.TrimPrefix(trimmed, "default=")
			continue
		}
		if trimmed == "deprecated" {
			options.deprecated = true
			continue
		}
		if strings.HasPrefix(trimmed, "deprecated=") {
			options.deprecated = true
			options.deprecationReason = strings.TrimPrefix(trimmed, "deprecated=")
			continue
		}
		if strings.HasPrefix(trimmed, "description=") {
			options.description = strings.TrimPrefix(trimmed, "description=")
		}
	}

	return options
}

func lowerFirst(value string) string {
	if value == "" {
		return value
	}
	return strings.ToLower(value[:1]) + value[1:]
}
