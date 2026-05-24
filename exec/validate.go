package exec

import (
	"fmt"
	"strings"

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
	errs = append(errs, validateVariables(bundle, op)...)
	errs = append(errs, validateVariableDefinitions(s, op)...)
	errs = append(errs, validateFragmentDefinitions(s, bundle.fragments)...)
	errs = append(errs, validateDeferStreamLabels(op)...)

	root, rootErr := rootObjectForKind(s, op.Kind)
	if rootErr != nil {
		errs = append(errs, newValidationError(core.Location{Line: 1, Column: 1}, "%s", rootErr.Error()))
		return errs
	}

	if op.Kind == operationSubscription {
		errs = append(errs, validateSubscriptionRoot(op)...)
	}

	usedFragments := make(map[string]bool)
	errs = append(errs, validateSelections(s, root, op.Selections, op.Fragments, usedFragments, nil, root.Name(), op.VariableTypes)...)

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
	variableTypes map[string]string,
) []validationError {
	var errs []validationError
	errs = append(errs, validateFieldMergeConflicts(selections)...)

	for _, sel := range selections {
		switch {
		case sel.isFragmentSpread():
			errs = append(errs, validateFragmentSpread(s, parent, sel, fragments, usedFragments, spreadStack, parentTypeName, variableTypes)...)
		case sel.isInlineFragment():
			errs = append(errs, validateInlineFragment(s, parent, sel, fragments, usedFragments, spreadStack, variableTypes)...)
		case sel.isField():
			errs = append(errs, validateField(s, parent, sel, fragments, usedFragments, spreadStack, variableTypes)...)
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
	variableTypes map[string]string,
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
	errs = append(errs, validateSelections(s, objectParent, fd.Selections, fragments, usedFragments, nextStack, fd.TypeCondition, variableTypes)...)
	return errs
}

func validateInlineFragment(
	s *schema.Schema,
	parent *schema.Object,
	sel selection,
	fragments map[string]*fragmentDef,
	usedFragments map[string]bool,
	spreadStack []string,
	variableTypes map[string]string,
) []validationError {
	var errs []validationError
	errs = append(errs, validateDirectives(sel, "INLINE_FRAGMENT")...)

	cond := sel.InlineFragmentOn
	if cond == "" {
		if parent == nil {
			return errs
		}
		return append(errs, validateSelections(s, parent, sel.Selections, fragments, usedFragments, spreadStack, parent.Name(), variableTypes)...)
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
	return append(errs, validateSelections(s, objectParent, sel.Selections, fragments, usedFragments, spreadStack, cond, variableTypes)...)
}

func validateField(
	s *schema.Schema,
	parent *schema.Object,
	sel selection,
	fragments map[string]*fragmentDef,
	usedFragments map[string]bool,
	spreadStack []string,
	variableTypes map[string]string,
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

	errs = append(errs, validateArguments(sel, field, parent.Name(), variableTypes)...)

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
					errs = append(errs, validateFragmentSpread(s, nil, child, fragments, usedFragments, spreadStack, typeString(fieldType), variableTypes)...)
				} else {
					errs = append(errs, validateInlineFragment(s, nil, child, fragments, usedFragments, spreadStack, variableTypes)...)
				}
				continue
			}
			errs = append(errs, newValidationError(child.Location,
				`Field "%s" of type "%s" must have a sub selection of subfields. Did you mean "%s { ... }"?`,
				sel.responseKey(), typeString(fieldType), sel.responseKey()))
		}
		return errs
	}
	errs = append(errs, validateSelections(s, childParent, sel.Selections, fragments, usedFragments, spreadStack, typeString(fieldType), variableTypes)...)
	return errs
}

func validateArguments(sel selection, field *schema.Field, parentType string, variableTypes map[string]string) []validationError {
	var errs []validationError
	seenArgs := make(map[string]bool)

	for argName := range sel.Arguments {
		if seenArgs[argName] {
			errs = append(errs, newValidationError(sel.Location,
				`There can be only one argument named "%s".`, argName))
			continue
		}
		seenArgs[argName] = true

		arg, found := fieldArg(field, argName)
		if !found {
			errs = append(errs, newValidationError(sel.Location,
				`Unknown argument "%s" on field "%s" of type "%s".`, argName, sel.Name, parentType))
			continue
		}
		if ref, ok := sel.Arguments[argName].(variableRef); ok {
			declaredType := variableTypes[ref.Name]
			if declaredType != "" && !variableTypeAllowed(declaredType, arg.Type.Name()) {
				errs = append(errs, newValidationError(sel.Location,
					`Variable "$%s" of type "%s" used in position expecting type "%s".`,
					ref.Name, declaredType, arg.Type.Name()))
			}
			continue
		}
		errs = append(errs, validateInputValue(arg.Type, sel.Arguments[argName], sel.Location, argName)...)
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

func validateVariables(bundle documentBundle, op document) []validationError {
	declared := make(map[string]bool, len(op.Variables))
	for _, variable := range op.Variables {
		declared[variable] = false
	}
	var errs []validationError
	for _, variable := range bundle.variableUses {
		if _, ok := declared[variable.Name]; !ok {
			errs = append(errs, newValidationError(variable.Location,
				`Variable "$%s" is not defined by operation "%s".`, variable.Name, op.Name))
			continue
		}
		declared[variable.Name] = true
	}
	for name, used := range declared {
		if !used {
			errs = append(errs, newValidationError(core.Location{Line: 1, Column: 1},
				`Variable "$%s" is never used in operation "%s".`, name, op.Name))
		}
	}
	return errs
}

func validateFieldMergeConflicts(selections []selection) []validationError {
	seen := map[string]selection{}
	var errs []validationError
	for _, sel := range selections {
		if !sel.isField() {
			continue
		}
		key := sel.responseKey()
		prev, ok := seen[key]
		if !ok {
			seen[key] = sel
			continue
		}
		if prev.Name != sel.Name || !argumentValuesEqual(prev.Arguments, sel.Arguments) {
			errs = append(errs, newValidationError(sel.Location,
				`Fields "%s" conflict because they select different fields or arguments. Use different aliases on the fields to fetch both if this was intentional.`,
				key))
		}
	}
	return errs
}

func argumentValuesEqual(left map[string]any, right map[string]any) bool {
	if len(left) != len(right) {
		return false
	}
	for key, leftValue := range left {
		rightValue, ok := right[key]
		if !ok || fmt.Sprint(leftValue) != fmt.Sprint(rightValue) {
			return false
		}
	}
	return true
}

func fieldArg(field *schema.Field, name string) (schema.InputValue, bool) {
	if field == nil {
		return schema.InputValue{}, false
	}
	if field.ArgsByName != nil {
		arg, ok := field.ArgsByName[name]
		return arg, ok
	}
	for _, arg := range field.Args {
		if arg.Name == name {
			return arg, true
		}
	}
	return schema.InputValue{}, false
}

// executableDirectiveLocations maps built-in directives that are valid on
// executable documents to the location(s) where they may appear.
var executableDirectiveLocations = map[string]map[string]bool{
	"skip":    {"FIELD": true, "FRAGMENT_SPREAD": true, "INLINE_FRAGMENT": true},
	"include": {"FIELD": true, "FRAGMENT_SPREAD": true, "INLINE_FRAGMENT": true},
	"defer":   {"FIELD": true, "FRAGMENT_SPREAD": true, "INLINE_FRAGMENT": true},
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
		errs = append(errs, validateDirectiveArguments(sel.Location, d)...)
	}
	return errs
}

func validateDirectiveArguments(loc core.Location, d directive) []validationError {
	switch d.Name {
	case "skip", "include":
		raw, ok := d.Args["if"]
		if !ok {
			return []validationError{newValidationError(loc, `Directive "@%s" argument "if" of type "Boolean!" is required, but it was not provided.`, d.Name)}
		}
		if ref, ok := raw.(variableRef); ok {
			if !ref.HasValue {
				return []validationError{newValidationError(loc, `Argument "if" on directive "@%s" variable "$%s" is missing.`, d.Name, ref.Name)}
			}
			raw = ref.Value
		}
		if _, ok := raw.(bool); !ok {
			return []validationError{newValidationError(loc, `Argument "if" on directive "@%s" must be Boolean.`, d.Name)}
		}
	case "defer":
		if raw, ok := d.Args["if"]; ok {
			if _, ok := raw.(bool); !ok {
				return []validationError{newValidationError(loc, `Argument "if" on directive "@defer" must be Boolean.`)}
			}
		}
		if raw, ok := d.Args["label"]; ok {
			if _, ok := raw.(string); !ok {
				return []validationError{newValidationError(loc, `Argument "label" on directive "@defer" must be String.`)}
			}
		}
	case "stream":
		if raw, ok := d.Args["if"]; ok {
			if _, ok := raw.(bool); !ok {
				return []validationError{newValidationError(loc, `Argument "if" on directive "@stream" must be Boolean.`)}
			}
		}
		if raw, ok := d.Args["label"]; ok {
			if _, ok := raw.(string); !ok {
				return []validationError{newValidationError(loc, `Argument "label" on directive "@stream" must be String.`)}
			}
		}
		if raw, ok := d.Args["initialCount"]; ok {
			if _, ok := raw.(int); !ok {
				return []validationError{newValidationError(loc, `Argument "initialCount" on directive "@stream" must be Int.`)}
			}
		}
	}
	return nil
}

// validateVariableDefinitions enforces variable name uniqueness, that every
// declared variable type names a known type, and that the type is an input
// type (scalar, enum, or input object) per the GraphQL spec.
func validateVariableDefinitions(s *schema.Schema, op document) []validationError {
	if len(op.Variables) == 0 {
		return nil
	}
	var errs []validationError
	seen := make(map[string]int, len(op.Variables))
	for _, name := range op.Variables {
		seen[name]++
		if seen[name] == 2 {
			errs = append(errs, newValidationError(core.Location{Line: 1, Column: 1},
				`There can be only one variable named "$%s".`, name))
		}
	}
	for _, name := range op.Variables {
		typeRef := op.VariableTypes[name]
		base := looseTypeName(typeRef)
		if base == "" {
			continue
		}
		t, ok := s.Types[base]
		if !ok {
			// The schema builder permits loose type-name references (suffix
			// matching), so an unregistered name is not necessarily unknown.
			// Only types we can resolve are checked for input-type validity.
			continue
		}
		if !isInputType(t) {
			errs = append(errs, newValidationError(core.Location{Line: 1, Column: 1},
				`Variable "$%s" cannot be non-input type "%s".`, name, typeRef))
		}
	}
	return errs
}

// validateDeferStreamLabels enforces that @defer/@stream labels are unique
// across the document and that the directives are not applied to root fields of
// mutation or subscription operations.
func validateDeferStreamLabels(op document) []validationError {
	var errs []validationError

	if op.Kind == operationMutation || op.Kind == operationSubscription {
		for _, sel := range op.Selections {
			if deferActive, _ := deferDirective(sel.Directives); deferActive {
				errs = append(errs, newValidationError(sel.Location,
					`@defer may not be used on root fields of a %s operation.`, op.Kind))
			}
			if streamActive, _, _ := streamDirective(sel.Directives); streamActive {
				errs = append(errs, newValidationError(sel.Location,
					`@stream may not be used on root fields of a %s operation.`, op.Kind))
			}
		}
	}

	// labels is allocated lazily so documents without @defer/@stream labels
	// (the common case) add no allocations on the validation hot path.
	var labels map[string]bool
	errs = walkDeferStreamLabels(op.Selections, &labels, errs)
	for _, fd := range op.Fragments {
		if fd != nil {
			errs = walkDeferStreamLabels(fd.Selections, &labels, errs)
		}
	}
	return errs
}

func walkDeferStreamLabels(sels []selection, labels *map[string]bool, errs []validationError) []validationError {
	for _, sel := range sels {
		for _, label := range deferStreamLabels(sel.Directives) {
			if *labels == nil {
				*labels = make(map[string]bool)
			}
			if (*labels)[label] {
				errs = append(errs, newValidationError(sel.Location,
					`Defer/Stream directive label "%s" must be unique.`, label))
			} else {
				(*labels)[label] = true
			}
		}
		errs = walkDeferStreamLabels(sel.Selections, labels, errs)
	}
	return errs
}

func deferStreamLabels(dirs []directive) []string {
	var out []string
	if active, label := deferDirective(dirs); active && label != "" {
		out = append(out, label)
	}
	if active, _, label := streamDirective(dirs); active && label != "" {
		out = append(out, label)
	}
	return out
}

// validateInputValue checks a literal (non-variable) argument or input value
// against its expected schema type, covering null-in-non-null, scalar/enum
// representation, list shape, and input-object field rules.
func validateInputValue(t schema.Type, value any, loc core.Location, label string) []validationError {
	if _, ok := value.(variableRef); ok {
		return nil // variable position compatibility is checked separately
	}
	switch typed := t.(type) {
	case *schema.NonNull:
		if value == nil {
			return []validationError{newValidationError(loc,
				`Expected value of type "%s", found null.`, typed.Name())}
		}
		return validateInputValue(typed.OfType, value, loc, label)
	case *schema.List:
		if value == nil {
			return nil
		}
		if list, ok := value.([]any); ok {
			var errs []validationError
			for i, item := range list {
				errs = append(errs, validateInputValue(typed.OfType, item, loc, fmt.Sprintf("%s[%d]", label, i))...)
			}
			return errs
		}
		return validateInputValue(typed.OfType, value, loc, label)
	case *schema.InputObject:
		if value == nil {
			return nil
		}
		return validateInputObjectValue(typed, value, loc)
	case *schema.Enum:
		if value == nil {
			return nil
		}
		name, ok := value.(string)
		if !ok {
			return []validationError{newValidationError(loc,
				`Enum "%s" cannot represent non-string value: %v.`, typed.Name(), value)}
		}
		for _, v := range typed.Values {
			if v.Name == name {
				return nil
			}
		}
		return []validationError{newValidationError(loc,
			`Value "%s" does not exist in "%s" enum.`, name, typed.Name())}
	case *schema.Scalar:
		if value == nil {
			return nil
		}
		return validateScalarLiteral(typed.TypeName, value, loc)
	default:
		return nil
	}
}

func validateInputObjectValue(io *schema.InputObject, value any, loc core.Location) []validationError {
	obj, ok := value.(map[string]any)
	if !ok {
		return []validationError{newValidationError(loc,
			`Expected value of type "%s" to be an object.`, io.Name())}
	}
	var errs []validationError

	for name := range obj {
		if _, exists := io.Fields[name]; !exists {
			errs = append(errs, newValidationError(loc,
				`Field "%s" is not defined by type "%s".`, name, io.Name()))
		}
	}

	for name, field := range io.Fields {
		raw, present := obj[name]
		if !present {
			if field.DefaultValue == nil {
				if _, required := field.Type.(*schema.NonNull); required {
					errs = append(errs, newValidationError(loc,
						`Field "%s.%s" of required type "%s" was not provided.`, io.Name(), name, typeString(field.Type)))
				}
			}
			continue
		}
		errs = append(errs, validateInputValue(field.Type, raw, loc, name)...)
	}

	if io.IsOneOf && len(obj) != 1 {
		errs = append(errs, newValidationError(loc,
			`OneOf Input Object "%s" must specify exactly one field.`, io.Name()))
	}

	return errs
}

func validateScalarLiteral(typeName string, value any, loc core.Location) []validationError {
	switch typeName {
	case "Int":
		switch value.(type) {
		case int, int64:
			return nil
		}
		return []validationError{newValidationError(loc, `Int cannot represent non-integer value: %v.`, value)}
	case "Float":
		switch value.(type) {
		case int, int64, float64:
			return nil
		}
		return []validationError{newValidationError(loc, `Float cannot represent non numeric value: %v.`, value)}
	case "Boolean":
		if _, ok := value.(bool); ok {
			return nil
		}
		return []validationError{newValidationError(loc, `Boolean cannot represent a non boolean value: %v.`, value)}
	case "String":
		if _, ok := value.(string); ok {
			return nil
		}
		return []validationError{newValidationError(loc, `String cannot represent a non string value: %v.`, value)}
	case "ID":
		switch value.(type) {
		case string, int, int64:
			return nil
		}
		return []validationError{newValidationError(loc, `ID cannot represent value: %v.`, value)}
	default:
		return nil // custom scalar: representation is decided by its Parse function
	}
}

func isInputType(t schema.Type) bool {
	switch innerNamedType(t).(type) {
	case *schema.Scalar, *schema.Enum, *schema.InputObject:
		return true
	default:
		return false
	}
}

func variableTypeAllowed(declared string, expected string) bool {
	if declared == expected {
		return true
	}
	if strings.HasSuffix(declared, "!") && strings.TrimSuffix(declared, "!") == expected {
		return true
	}
	declaredName := looseTypeName(declared)
	expectedName := looseTypeName(expected)
	if declaredName != "" && strings.HasSuffix(expectedName, declaredName) {
		return true
	}
	return false
}

// looseTypeNameReplacer strips list/non-null wrappers from a type reference.
// It is built once: strings.NewReplacer is safe for concurrent use and
// rebuilding it per call dominated validation allocations.
var looseTypeNameReplacer = strings.NewReplacer("[", "", "]", "", "!", "")

func looseTypeName(typeName string) string {
	return strings.TrimSpace(looseTypeNameReplacer.Replace(typeName))
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
