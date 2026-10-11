package exec

import (
	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/schema"
	"reflect"
	"slices"
)

type mergeField struct {
	Parent    schema.Type
	Selection *selection
}

type mergePair struct {
	Left      mergeField
	Right     mergeField
	Exclusive bool
}

type mergeGroup struct {
	First  mergeField
	Others []mergeField
}

type mergePairKey struct {
	Left      core.Location
	Right     core.Location
	LeftType  string
	RightType string
}

func validateFieldMergeConflicts(s *schema.Schema, parent schema.Type, selections []selection, fragments map[string]*fragmentDef) []validationError {
	if len(selections) < 2 {
		return nil
	}
	seen := make(map[string]mergeGroup)
	var errs []validationError
	compare := func(field mergeField) {
		key := field.Selection.responseKey()
		group, exists := seen[key]
		if !exists {
			seen[key] = mergeGroup{First: field}
			return
		}
		errs = append(errs, validateMergePair(s, mergePair{Left: group.First, Right: field}, fragments, nil)...)
		for _, previous := range group.Others {
			errs = append(errs, validateMergePair(s, mergePair{Left: previous, Right: field}, fragments, nil)...)
		}
		group.Others = append(group.Others, field)
		seen[key] = group
	}
	direct := true
	for _, sel := range selections {
		if !sel.isField() {
			direct = false
			break
		}
	}
	if direct {
		for index := range selections {
			compare(mergeField{Parent: parent, Selection: &selections[index]})
		}
	} else {
		for _, field := range collectMergeFields(s, parent, selections, fragments) {
			compare(field)
		}
	}
	return errs
}

func collectMergeFields(s *schema.Schema, parent schema.Type, selections []selection, fragments map[string]*fragmentDef) []mergeField {
	fields := make([]mergeField, 0, len(selections))
	var visited map[string]bool
	var collect func(schema.Type, []selection)
	collect = func(parent schema.Type, selections []selection) {
		for index := range selections {
			sel := &selections[index]
			switch {
			case sel.isField():
				fields = append(fields, mergeField{Parent: parent, Selection: sel})
			case sel.isInlineFragment():
				condition := parent
				if sel.InlineFragmentOn != "" {
					condition = s.Types[sel.InlineFragmentOn]
				}
				if condition != nil {
					collect(condition, sel.Selections)
				}
			case sel.isFragmentSpread():
				fragment := fragments[sel.FragmentSpread]
				if fragment == nil || visited[sel.FragmentSpread] {
					continue
				}
				if visited == nil {
					visited = make(map[string]bool)
				}
				visited[sel.FragmentSpread] = true
				if condition := s.Types[fragment.TypeCondition]; condition != nil {
					collect(condition, fragment.Selections)
				}
			}
		}
	}
	collect(parent, selections)
	return fields
}

func validateMergePair(s *schema.Schema, pair mergePair, fragments map[string]*fragmentDef, stack []mergePairKey) []validationError {
	left, right := pair.Left, pair.Right
	if left.Parent == nil || right.Parent == nil {
		return nil
	}
	key := mergePairKey{Left: left.Selection.Location, Right: right.Selection.Location, LeftType: left.Parent.Name(), RightType: right.Parent.Name()}
	if slices.Contains(stack, key) {
		return nil
	}
	if key.Left == key.Right && key.LeftType == key.RightType && key.Left.Line > 0 {
		return nil
	}
	stack = append(stack, key)
	exclusive := pair.Exclusive
	if left.Parent != nil && right.Parent != nil && left.Parent.Kind() == schema.ObjectKind && right.Parent.Kind() == schema.ObjectKind && left.Parent.Name() != right.Parent.Name() {
		exclusive = true
	}
	leftType := mergeFieldType(left)
	rightType := mergeFieldType(right)
	if leftType == nil || rightType == nil {
		return nil
	}
	if !compatibleResponseShape(leftType, rightType) || !exclusive && (left.Selection.Name != right.Selection.Name || !argumentValuesEqual(left.Selection.Arguments, right.Selection.Arguments)) {
		return []validationError{newValidationError(right.Selection.Location, `Fields "%s" conflict because they select incompatible fields, arguments, or response types. Use different aliases on the fields to fetch both if this was intentional.`, right.Selection.responseKey())}
	}
	if len(left.Selection.Selections) == 0 || len(right.Selection.Selections) == 0 {
		return nil
	}
	leftChildren := collectMergeFields(s, innerNamedType(leftType), left.Selection.Selections, fragments)
	rightChildren := collectMergeFields(s, innerNamedType(rightType), right.Selection.Selections, fragments)
	var errs []validationError
	for _, leftChild := range leftChildren {
		for _, rightChild := range rightChildren {
			if leftChild.Selection.responseKey() == rightChild.Selection.responseKey() {
				errs = append(errs, validateMergePair(s, mergePair{Left: leftChild, Right: rightChild, Exclusive: exclusive}, fragments, stack)...)
			}
		}
	}
	return errs
}

func mergeFieldType(field mergeField) schema.Type {
	if field.Selection.Name == "__typename" {
		return &schema.NonNull{OfType: &schema.Scalar{TypeName: "String"}}
	}
	definition, ok := selectionField(field.Parent, field.Selection.Name)
	if !ok {
		return nil
	}
	return definition.Type
}

func compatibleResponseShape(left schema.Type, right schema.Type) bool {
	if typed, ok := left.(*schema.NonNull); ok {
		other, ok := right.(*schema.NonNull)
		return ok && compatibleResponseShape(typed.OfType, other.OfType)
	}
	if _, ok := right.(*schema.NonNull); ok {
		return false
	}
	if typed, ok := left.(*schema.List); ok {
		other, ok := right.(*schema.List)
		return ok && compatibleResponseShape(typed.OfType, other.OfType)
	}
	if _, ok := right.(*schema.List); ok {
		return false
	}
	if isLeafType(left) || isLeafType(right) {
		return left.Name() == right.Name()
	}
	return true
}

func argumentValuesEqual(left map[string]any, right map[string]any) bool {
	if len(left) != len(right) {
		return false
	}
	for name, value := range left {
		other, ok := right[name]
		if !ok || !argumentValueEqual(value, other) {
			return false
		}
	}
	return true
}

func argumentValueEqual(left any, right any) bool {
	switch typed := left.(type) {
	case variableRef:
		other, ok := right.(variableRef)
		return ok && typed.Name == other.Name
	case map[string]any:
		other, ok := right.(map[string]any)
		return ok && argumentValuesEqual(typed, other)
	case []any:
		other, ok := right.([]any)
		if !ok || len(typed) != len(other) {
			return false
		}
		for index, item := range typed {
			if !argumentValueEqual(item, other[index]) {
				return false
			}
		}
		return true
	default:
		return reflect.DeepEqual(left, right)
	}
}
