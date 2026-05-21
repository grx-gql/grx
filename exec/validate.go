package exec

import (
	"fmt"

	"github.com/patrickkabwe/grx/core"
	"github.com/patrickkabwe/grx/schema"
)

// ValidateDocument runs GraphQL validation rules on the selected operation.
// Error messages follow graphql-js conventions.
func ValidateDocument(s *schema.Schema, bundle documentBundle, op document) []validationError {
	if s == nil {
		return nil
	}

	var errs []validationError
	errs = append(errs, validateUniqueOperationNames(bundle)...)
	errs = append(errs, validateLoneAnonymousOperation(bundle)...)
	errs = append(errs, validateFragmentDefinitions(s, bundle.fragments)...)

	root, rootErr := rootObjectForKind(s, op.Kind)
	if rootErr != nil {
		errs = append(errs, newValidationError(core.Location{Line: 1, Column: 1}, "%s", rootErr.Error()))
		return errs
	}

	if op.Kind == operationSubscription {
		errs = append(errs, validateSubscriptionRoot(op)...)
	}

	usedFragments := make(map[string]bool)
	errs = append(errs, validateSelections(s, root, op.Selections, op.Fragments, usedFragments, nil, root.Name())...)

	for name := range op.Fragments {
		if !usedFragments[name] {
			fd := op.Fragments[name]
			errs = append(errs, newValidationError(
				locationForOffset(bundle.source, fd.NameOffset),
				`Fragment "%s" is never used.`, name,
			))
		}
	}

	return errs
}

func validateUniqueOperationNames(bundle documentBundle) []validationError {
	seen := make(map[string]core.Location)
	var errs []validationError
	for _, op := range bundle.operations {
		if op.Name == "" {
			continue
		}
		if prev, dup := seen[op.Name]; dup {
			errs = append(errs,
				newValidationError(prev, `There can be only one operation named "%s".`, op.Name),
				newValidationError(core.Location{Line: 1, Column: 1}, `There can be only one operation named "%s".`, op.Name),
			)
		} else {
			seen[op.Name] = core.Location{Line: 1, Column: 1}
		}
	}
	return errs
}

func validateLoneAnonymousOperation(bundle documentBundle) []validationError {
	if len(bundle.operations) <= 1 {
		return nil
	}
	for _, op := range bundle.operations {
		if op.Name == "" {
			return []validationError{
				newValidationError(core.Location{Line: 1, Column: 1},
					"This anonymous operation must be the only defined operation."),
			}
		}
	}
	return nil
}

func validateFragmentDefinitions(s *schema.Schema, fragments map[string]*fragmentDef) []validationError {
	var errs []validationError
	for _, fd := range fragments {
		loc := core.Location{Line: 1, Column: 1}
		typeValue, ok := s.Types[fd.TypeCondition]
		if !ok {
			errs = append(errs, newValidationError(loc,
				`Unknown type "%s".`, fd.TypeCondition))
			continue
		}
		if !isCompositeType(typeValue) {
			errs = append(errs, newValidationError(loc,
				`Fragment "%s" cannot condition on non composite type "%s".`, fd.Name, fd.TypeCondition))
		}
	}
	return errs
}

func validateSubscriptionRoot(op document) []validationError {
	count := 0
	for _, sel := range op.Selections {
		if sel.isField() {
			count++
		}
	}
	if count != 1 {
		name := op.Name
		if name == "" {
			name = "anonymous"
		}
		return []validationError{
			newValidationError(op.Selections[0].Location,
				`Subscription "%s" must select only one top level field.`, name),
		}
	}
	return nil
}

func validateSelections(
	s *schema.Schema,
	parent *schema.Object,
	selections []selection,
	fragments map[string]*fragmentDef,
	usedFragments map[string]bool,
	spreadStack []string,
	parentTypeName string,
) []validationError {
	var errs []validationError

	for _, sel := range selections {
		switch {
		case sel.isFragmentSpread():
			errs = append(errs, validateFragmentSpread(s, parent, sel, fragments, usedFragments, spreadStack, parentTypeName)...)
		case sel.isInlineFragment():
			errs = append(errs, validateInlineFragment(s, parent, sel, fragments, usedFragments, spreadStack)...)
		case sel.isField():
			errs = append(errs, validateField(s, parent, sel, fragments, usedFragments, spreadStack)...)
		default:
			errs = append(errs, newValidationError(sel.Location, "Invalid selection."))
		}
	}

	return errs
}

func validateFragmentSpread(
	s *schema.Schema,
	parent *schema.Object,
	sel selection,
	fragments map[string]*fragmentDef,
	usedFragments map[string]bool,
	spreadStack []string,
	parentTypeName string,
) []validationError {
	name := sel.FragmentSpread
	fd, ok := fragments[name]
	if !ok {
		return []validationError{newValidationError(sel.Location, `Unknown fragment "%s".`, name)}
	}
	usedFragments[name] = true

	for _, active := range spreadStack {
		if active == name {
			return []validationError{newValidationError(sel.Location,
				`Cannot spread fragment "%s" within itself.`, name)}
		}
	}

	var errs []validationError
	errs = append(errs, validateDirectives(sel, "FRAGMENT_SPREAD")...)

	if parent != nil && !fragmentTypeMatches(parent, fd.TypeCondition) {
		errs = append(errs, newValidationError(sel.Location,
			`Fragment cannot be spread here as objects of type "%s" can never be of type "%s".`,
			parentTypeName, fd.TypeCondition))
	}

	condType, ok := s.Types[fd.TypeCondition]
	if !ok {
		return errs
	}
	objectParent := objectTypeForSelection(condType)
	if objectParent == nil {
		return errs
	}

	nextStack := append(append([]string{}, spreadStack...), name)
	errs = append(errs, validateSelections(s, objectParent, fd.Selections, fragments, usedFragments, nextStack, fd.TypeCondition)...)
	return errs
}

func validateInlineFragment(
	s *schema.Schema,
	parent *schema.Object,
	sel selection,
	fragments map[string]*fragmentDef,
	usedFragments map[string]bool,
	spreadStack []string,
) []validationError {
	var errs []validationError
	errs = append(errs, validateDirectives(sel, "INLINE_FRAGMENT")...)

	cond := sel.InlineFragmentOn
	if cond == "" {
		if parent == nil {
			return errs
		}
		return append(errs, validateSelections(s, parent, sel.Selections, fragments, usedFragments, spreadStack, parent.Name())...)
	}

	typeValue, ok := s.Types[cond]
	if !ok {
		return append(errs, newValidationError(sel.Location, `Unknown type "%s".`, cond))
	}
	if !isCompositeType(typeValue) {
		return append(errs, newValidationError(sel.Location,
			`Fragment cannot condition on non composite type "%s".`, cond))
	}
	if parent != nil && !fragmentTypeMatches(parent, cond) {
		return append(errs, newValidationError(sel.Location,
			`Fragment cannot be spread here as objects of type "%s" can never be of type "%s".`,
			parent.Name(), cond))
	}

	objectParent := objectTypeForSelection(typeValue)
	if objectParent == nil {
		return errs
	}
	return append(errs, validateSelections(s, objectParent, sel.Selections, fragments, usedFragments, spreadStack, cond)...)
}

func validateField(
	s *schema.Schema,
	parent *schema.Object,
	sel selection,
	fragments map[string]*fragmentDef,
	usedFragments map[string]bool,
	spreadStack []string,
) []validationError {
	if parent == nil {
		return []validationError{newValidationError(sel.Location, "Cannot select field on abstract parent without type condition.")}
	}

	var errs []validationError
	errs = append(errs, validateDirectives(sel, "FIELD")...)

	fieldName := sel.Name
	if fieldName == "__typename" {
		if len(sel.Selections) > 0 {
			errs = append(errs, newValidationError(sel.Location,
				`Field "%s" must not have a selection since type "String" has no subfields.`, sel.responseKey()))
		}
		return errs
	}

	field, ok := parent.Fields[fieldName]
	if !ok {
		return append(errs, newValidationError(sel.Location,
			`Cannot query field "%s" on type "%s".`, fieldName, parent.Name()))
	}

	errs = append(errs, validateArguments(sel, field, parent.Name())...)

	fieldType := field.Type
	if isLeafType(fieldType) {
		if len(sel.Selections) > 0 {
			errs = append(errs, newValidationError(sel.Location,
				`Field "%s" must not have a selection since type "%s" has no subfields.`,
				sel.responseKey(), typeString(fieldType)))
		}
		return errs
	}

	if len(sel.Selections) == 0 {
		errs = append(errs, newValidationError(sel.Location,
			`Field "%s" of type "%s" must have a selection of subfields. Did you mean "%s { ... }"?`,
			sel.responseKey(), typeString(fieldType), sel.responseKey()))
		return errs
	}

	childParent := objectTypeForFieldType(s, fieldType)
	if childParent == nil {
		for _, child := range sel.Selections {
			if child.isField() && child.Name == "__typename" && len(child.Selections) == 0 {
				continue
			}
			if child.isInlineFragment() || child.isFragmentSpread() {
				if child.isFragmentSpread() {
					errs = append(errs, validateFragmentSpread(s, nil, child, fragments, usedFragments, spreadStack, typeString(fieldType))...)
				} else {
					errs = append(errs, validateInlineFragment(s, nil, child, fragments, usedFragments, spreadStack)...)
				}
				continue
			}
			errs = append(errs, newValidationError(child.Location,
				`Field "%s" of type "%s" must have a sub selection of subfields. Did you mean "%s { ... }"?`,
				sel.responseKey(), typeString(fieldType), sel.responseKey()))
		}
		return errs
	}
	errs = append(errs, validateSelections(s, childParent, sel.Selections, fragments, usedFragments, spreadStack, typeString(fieldType))...)
	return errs
}

func validateArguments(sel selection, field *schema.Field, parentType string) []validationError {
	var errs []validationError
	seenArgs := make(map[string]bool)

	for argName := range sel.Arguments {
		if seenArgs[argName] {
			errs = append(errs, newValidationError(sel.Location,
				`There can be only one argument named "%s".`, argName))
			continue
		}
		seenArgs[argName] = true

		found := false
		for _, arg := range field.Args {
			if arg.Name == argName {
				found = true
				break
			}
		}
		if !found {
			errs = append(errs, newValidationError(sel.Location,
				`Unknown argument "%s" on field "%s" of type "%s".`, argName, sel.Name, parentType))
		}
	}

	for _, arg := range field.Args {
		if _, ok := sel.Arguments[arg.Name]; ok {
			continue
		}
		if arg.DefaultValue != nil {
			continue
		}
		if _, isRequired := arg.Type.(*schema.NonNull); isRequired {
			errs = append(errs, newValidationError(sel.Location,
				`Field "%s" argument "%s" of type "%s" is required, but it was not provided.`,
				sel.Name, arg.Name, typeString(arg.Type)))
		}
	}

	return errs
}

// executableDirectiveLocations maps built-in directives that are valid on
// executable documents to the location(s) where they may appear.
var executableDirectiveLocations = map[string]map[string]bool{
	"skip":    {"FIELD": true, "FRAGMENT_SPREAD": true, "INLINE_FRAGMENT": true},
	"include": {"FIELD": true, "FRAGMENT_SPREAD": true, "INLINE_FRAGMENT": true},
	"defer":   {"FIELD": true, "FRAGMENT_SPREAD": true},
	"stream":  {"FIELD": true},
}

// repeatableExecutableDirectives lists directives that may appear more than
// once on the same location in an executable document.
var repeatableExecutableDirectives = map[string]bool{}

func validateDirectives(sel selection, location string) []validationError {
	var errs []validationError
	seen := make(map[string]bool)
	for _, d := range sel.Directives {
		allowedLocs, known := executableDirectiveLocations[d.Name]
		if !known {
			errs = append(errs, newValidationError(sel.Location, `Unknown directive "@%s".`, d.Name))
			continue
		}

		isRepeatable := repeatableExecutableDirectives[d.Name]
		if seen[d.Name] && !isRepeatable {
			errs = append(errs, newValidationError(sel.Location,
				`Directive "@%s" may not be used more than once at this location.`, d.Name))
			continue
		}
		seen[d.Name] = true

		if !allowedLocs[location] {
			errs = append(errs, newValidationError(sel.Location,
				`Directive "@%s" may not be used on %s.`, d.Name, location))
		}
	}
	return errs
}

func rootObjectForKind(s *schema.Schema, kind operationKind) (*schema.Object, error) {
	switch kind {
	case operationQuery:
		if s.Query == nil {
			return nil, fmt.Errorf("schema has no query root")
		}
		return s.Query, nil
	case operationMutation:
		if s.Mutation == nil {
			return nil, fmt.Errorf("schema has no mutation root")
		}
		return s.Mutation, nil
	case operationSubscription:
		if s.Subscription == nil {
			return nil, fmt.Errorf("schema has no subscription root")
		}
		return s.Subscription, nil
	default:
		return nil, fmt.Errorf("unknown operation kind %q", kind)
	}
}

func isCompositeType(t schema.Type) bool {
	switch innerNamedType(t).(type) {
	case *schema.Object, *schema.Interface, *schema.Union:
		return true
	default:
		return false
	}
}

func isLeafType(t schema.Type) bool {
	switch innerNamedType(t).(type) {
	case *schema.Scalar, *schema.Enum:
		return true
	default:
		return false
	}
}

func innerNamedType(t schema.Type) schema.Type {
	for {
		switch x := t.(type) {
		case *schema.NonNull:
			t = x.OfType
		case *schema.List:
			t = x.OfType
		default:
			return t
		}
	}
}

func typeString(t schema.Type) string {
	if t == nil {
		return ""
	}
	return t.Name()
}

func objectTypeForSelection(t schema.Type) *schema.Object {
	switch x := innerNamedType(t).(type) {
	case *schema.Object:
		return x
	case *schema.Interface:
		// Use interface fields for validation when concrete type unknown.
		return &schema.Object{TypeName: x.Name(), Fields: x.Fields}
	default:
		return nil
	}
}

func objectTypeForFieldType(s *schema.Schema, t schema.Type) *schema.Object {
	named := innerNamedType(t)
	switch x := named.(type) {
	case *schema.Object:
		return x
	case *schema.Interface:
		return &schema.Object{TypeName: x.Name(), Fields: x.Fields}
	case *schema.Union:
		return nil
	default:
		return nil
	}
}
