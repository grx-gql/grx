package exec

import "maps"

// inputValueTransform returns the mapped value and whether it is present.
type inputValueTransform func(any) (any, bool)

// transformOperationValues copies only syntax paths containing arguments.
// Argument-free subtrees are immutable and can be shared across requests.
func transformOperationValues(doc document, transform inputValueTransform) document {
	doc.Selections = transformSelectionValues(doc.Selections, transform)
	doc.Directives = transformDirectiveValues(doc.Directives, transform)
	var fragments map[string]*fragmentDef
	for name, original := range doc.Fragments {
		selections := transformSelectionValues(original.Selections, transform)
		directives := transformDirectiveValues(original.Directives, transform)
		if sameSelectionStorage(selections, original.Selections) && sameDirectiveStorage(directives, original.Directives) {
			continue
		}
		if fragments == nil {
			fragments = maps.Clone(doc.Fragments)
		}
		fragment := *original
		fragment.Selections, fragment.Directives = selections, directives
		fragments[name] = &fragment
	}
	if fragments != nil {
		doc.Fragments = fragments
	}
	return doc
}

func sameSelectionStorage(left, right []selection) bool {
	return len(left) == len(right) && (len(left) == 0 || &left[0] == &right[0])
}

func sameDirectiveStorage(left, right []directive) bool {
	return len(left) == len(right) && (len(left) == 0 || &left[0] == &right[0])
}

func transformSelectionValues(selections []selection, transform inputValueTransform) []selection {
	var out []selection
	for index, selected := range selections {
		children := transformSelectionValues(selected.Selections, transform)
		directives := transformDirectiveValues(selected.Directives, transform)
		if len(selected.Arguments) == 0 && sameSelectionStorage(children, selected.Selections) && sameDirectiveStorage(directives, selected.Directives) {
			continue
		}
		if out == nil {
			out = append([]selection(nil), selections...)
		}
		out[index].Arguments = transformInputValueMap(selected.Arguments, transform)
		out[index].Directives = directives
		out[index].Selections = children
	}
	if out == nil {
		return selections
	}
	return out
}

func transformDirectiveValues(directives []directive, transform inputValueTransform) []directive {
	var out []directive
	for index, dir := range directives {
		if len(dir.Args) == 0 {
			continue
		}
		if out == nil {
			out = append([]directive(nil), directives...)
		}
		out[index].Args = transformInputValueMap(dir.Args, transform)
	}
	if out == nil {
		return directives
	}
	return out
}

func transformInputValueMap(values map[string]any, transform inputValueTransform) map[string]any {
	if values == nil {
		return nil
	}
	out := make(map[string]any, len(values))
	for name, value := range values {
		if bound, present := transform(value); present {
			out[name] = bound
		}
	}
	return out
}
