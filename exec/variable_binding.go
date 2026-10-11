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
	doc.Selections = bindSelectionVariables(doc.Selections, scope)
	doc.Directives = bindDirectiveVariables(doc.Directives, scope)
	if len(doc.Fragments) > 0 {
		fragments := make(map[string]*fragmentDef, len(doc.Fragments))
		for name, original := range doc.Fragments {
			fragment := *original
			fragment.Selections = bindSelectionVariables(fragment.Selections, scope)
			fragment.Directives = bindDirectiveVariables(fragment.Directives, scope)
			fragments[name] = &fragment
		}
		doc.Fragments = fragments
	}
	return doc
}

type operationVariableValues struct {
	provided    map[string]any
	definitions map[string]variableDefinition
}

func bindSelectionVariables(selections []selection, scope operationVariableValues) []selection {
	if len(selections) == 0 {
		return selections
	}
	out := make([]selection, len(selections))
	for index, selected := range selections {
		out[index] = selected
		out[index].Arguments = bindVariableMap(selected.Arguments, scope)
		out[index].Directives = bindDirectiveVariables(selected.Directives, scope)
		out[index].Selections = bindSelectionVariables(selected.Selections, scope)
	}
	return out
}

func bindDirectiveVariables(directives []directive, scope operationVariableValues) []directive {
	if len(directives) == 0 {
		return directives
	}
	out := make([]directive, len(directives))
	for index, dir := range directives {
		out[index] = dir
		out[index].Args = bindVariableMap(dir.Args, scope)
	}
	return out
}

func bindVariableMap(values map[string]any, scope operationVariableValues) map[string]any {
	if values == nil {
		return nil
	}
	out := make(map[string]any, len(values))
	for name, value := range values {
		if bound, present := bindVariableValue(value, scope); present {
			out[name] = bound
		}
	}
	return out
}

func bindVariableValue(value any, scope operationVariableValues) (any, bool) {
	switch typed := value.(type) {
	case variableRef:
		raw, present := scope.provided[typed.Name]
		if !present {
			definition := scope.definitions[typed.Name]
			raw, present = definition.DefaultValue, definition.HasDefault
		}
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
