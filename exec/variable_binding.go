package exec

import "reflect"

// bindSelectedOperation creates request-local selections and fragments only
// when a document contains variable references. Cached syntax and caller input
// remain unchanged; defaults are read solely from the selected operation.
func bindSelectedOperation(bundle documentBundle, doc document) document {
	doc.Fragments = bundle.fragments
	if !bundle.hasVariableRefs {
		return doc
	}
	return bindOperationVariables(doc, bundle.variables)
}

func bindOperationVariables(doc document, provided map[string]any) document {
	scope := operationVariableValues{provided: provided, definitions: doc.VariableDefinitions}
	return transformOperationValues(doc, func(value any) (any, bool) {
		return bindVariableValue(value, scope)
	})
}

type operationVariableValues struct {
	provided    map[string]any
	definitions map[string]variableDefinition
}

func bindVariableMap(values map[string]any, scope operationVariableValues) map[string]any {
	return transformInputValueMap(values, func(value any) (any, bool) {
		return bindVariableValue(value, scope)
	})
}

func operationVariableInput(scope operationVariableValues, name string) (any, bool) {
	if value, present := scope.provided[name]; present {
		return value, true
	}
	definition := scope.definitions[name]
	return definition.DefaultValue, definition.HasDefault
}

// resolveOperationInputValue binds parser results directly, without allocating
// intermediate variable references or copying literal containers twice.
func resolveOperationInputValue(value any, scope operationVariableValues) (any, bool) {
	switch typed := value.(type) {
	case variableRef:
		raw, present := operationVariableInput(scope, typed.Name)
		return resolveValueVariableRef(raw), present
	case map[string]any:
		return transformInputValueMap(typed, func(item any) (any, bool) {
			return resolveOperationInputValue(item, scope)
		}), true
	case []any:
		out := make([]any, len(typed))
		for index, item := range typed {
			out[index], _ = resolveOperationInputValue(item, scope)
		}
		return out, true
	default:
		return resolveValueVariableRef(value), true
	}
}

func bindVariableValue(value any, scope operationVariableValues) (any, bool) {
	switch typed := value.(type) {
	case variableRef:
		raw, present := operationVariableInput(scope, typed.Name)
		if !present {
			return nil, false
		}
		if ref, ok := raw.(variableRef); ok {
			ref.Value = cloneVariableInput(ref.Value)
			return ref, true
		}
		return variableRef{Name: typed.Name, Value: cloneVariableInput(raw), HasValue: true}, true
	case map[string]any:
		return bindVariableMap(typed, scope), true
	case []any:
		out := make([]any, len(typed))
		for index, item := range typed {
			out[index], _ = bindVariableValue(item, scope)
		}
		return out, true
	default:
		return value, true
	}
}

// cloneVariableInput isolates GraphQL input containers from custom scalar
// parsers and resolvers that may modify their arguments.
func cloneVariableInput(value any) any {
	switch typed := value.(type) {
	case map[string]any:
		if typed == nil {
			return typed
		}
		out := make(map[string]any, len(typed))
		for name, item := range typed {
			out[name] = cloneVariableInput(item)
		}
		return out
	case []any:
		if typed == nil {
			return typed
		}
		out := make([]any, len(typed))
		for index, item := range typed {
			out[index] = cloneVariableInput(item)
		}
		return out
	default:
		raw := reflect.ValueOf(value)
		if raw.IsValid() && (raw.Kind() == reflect.Map || raw.Kind() == reflect.Slice) {
			return cloneTypedVariableContainer(raw).Interface()
		}
		return value
	}
}

// cloneTypedVariableContainer preserves named custom-scalar container types.
func cloneTypedVariableContainer(value reflect.Value) reflect.Value {
	switch value.Kind() {
	case reflect.Interface:
		if value.IsNil() {
			return value
		}
		out := reflect.New(value.Type()).Elem()
		out.Set(cloneTypedVariableContainer(value.Elem()))
		return out
	case reflect.Map:
		if value.IsNil() {
			return value
		}
		out := reflect.MakeMapWithSize(value.Type(), value.Len())
		iterator := value.MapRange()
		for iterator.Next() {
			out.SetMapIndex(iterator.Key(), cloneTypedVariableContainer(iterator.Value()))
		}
		return out
	case reflect.Slice:
		if value.IsNil() {
			return value
		}
		out := reflect.MakeSlice(value.Type(), value.Len(), value.Len())
		for index := 0; index < value.Len(); index++ {
			out.Index(index).Set(cloneTypedVariableContainer(value.Index(index)))
		}
		return out
	default:
		return value
	}
}
