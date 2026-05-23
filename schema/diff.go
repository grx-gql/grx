package schema

import (
	"fmt"
	"sort"
)

// ChangeSeverity classifies how a schema change affects existing clients,
// mirroring the categories used by common GraphQL schema-diff tooling.
type ChangeSeverity int

const (
	// NonBreaking changes are always safe for existing clients (e.g. adding an
	// optional field or a new type).
	NonBreaking ChangeSeverity = iota
	// Dangerous changes may affect clients depending on their assumptions (e.g.
	// adding an enum value or a union member).
	Dangerous
	// Breaking changes will break at least some existing clients (e.g. removing
	// a field or making an argument required).
	Breaking
)

// String returns the lowercase severity label.
func (s ChangeSeverity) String() string {
	switch s {
	case Breaking:
		return "breaking"
	case Dangerous:
		return "dangerous"
	default:
		return "non-breaking"
	}
}

// Change is a single difference between two schemas.
type Change struct {
	Severity ChangeSeverity
	Path     string
	Message  string
}

// String renders a change as "severity: message".
func (c Change) String() string {
	return fmt.Sprintf("%s: %s", c.Severity, c.Message)
}

// Diff compares two built schemas and returns the ordered list of differences,
// classified by severity. It detects added/removed types, fields, arguments,
// enum values, union members, and input fields, as well as type changes and
// newly-required arguments and input fields. Built-in and introspection
// (`__`-prefixed) types are ignored.
func Diff(oldSchema, newSchema *Schema) []Change {
	var changes []Change
	oldTypes := userTypes(oldSchema)
	newTypes := userTypes(newSchema)

	for name, oldType := range oldTypes {
		newType, ok := newTypes[name]
		if !ok {
			changes = append(changes, Change{Breaking, name, fmt.Sprintf("Type %q was removed", name)})
			continue
		}
		if oldType.Kind() != newType.Kind() {
			changes = append(changes, Change{Breaking, name, fmt.Sprintf("Type %q changed from %s to %s", name, oldType.Kind(), newType.Kind())})
			continue
		}
		changes = append(changes, diffSameType(name, oldType, newType)...)
	}

	for name := range newTypes {
		if _, ok := oldTypes[name]; !ok {
			changes = append(changes, Change{NonBreaking, name, fmt.Sprintf("Type %q was added", name)})
		}
	}

	sortChanges(changes)
	return changes
}

// HasBreaking reports whether changes contains at least one breaking change.
func HasBreaking(changes []Change) bool {
	for _, c := range changes {
		if c.Severity == Breaking {
			return true
		}
	}
	return false
}

func diffSameType(name string, oldType, newType Type) []Change {
	switch o := oldType.(type) {
	case *Object:
		return diffFieldMaps(name, o.Fields, newType.(*Object).Fields)
	case *Interface:
		return diffFieldMaps(name, o.Fields, newType.(*Interface).Fields)
	case *InputObject:
		return diffInputFieldMaps(name, o.Fields, newType.(*InputObject).Fields)
	case *Enum:
		return diffEnumValues(name, o, newType.(*Enum))
	case *Union:
		return diffUnionMembers(name, o, newType.(*Union))
	default:
		return nil
	}
}

func diffFieldMaps(typeName string, oldFields, newFields map[string]*Field) []Change {
	var changes []Change
	for fieldName, oldField := range oldFields {
		path := typeName + "." + fieldName
		newField, ok := newFields[fieldName]
		if !ok {
			changes = append(changes, Change{Breaking, path, fmt.Sprintf("Field %q was removed", path)})
			continue
		}
		if typeNameOf(oldField.Type) != typeNameOf(newField.Type) {
			changes = append(changes, Change{Breaking, path, fmt.Sprintf("Field %q changed type from %s to %s", path, typeNameOf(oldField.Type), typeNameOf(newField.Type))})
		}
		changes = append(changes, diffArgs(path, oldField.Args, newField.Args)...)
	}
	for fieldName := range newFields {
		if _, ok := oldFields[fieldName]; !ok {
			path := typeName + "." + fieldName
			changes = append(changes, Change{NonBreaking, path, fmt.Sprintf("Field %q was added", path)})
		}
	}
	return changes
}

func diffArgs(fieldPath string, oldArgList, newArgs []InputValue) []Change {
	var changes []Change
	oldArgs := make(map[string]InputValue, len(oldArgList))
	for _, arg := range oldArgList {
		oldArgs[arg.Name] = arg
	}
	newByName := make(map[string]InputValue, len(newArgs))
	for _, arg := range newArgs {
		newByName[arg.Name] = arg
	}
	for argName, oldArg := range oldArgs {
		path := fieldPath + "(" + argName + ")"
		newArg, ok := newByName[argName]
		if !ok {
			changes = append(changes, Change{Breaking, path, fmt.Sprintf("Argument %q was removed from %q", argName, fieldPath)})
			continue
		}
		if typeNameOf(oldArg.Type) != typeNameOf(newArg.Type) {
			changes = append(changes, Change{Breaking, path, fmt.Sprintf("Argument %q on %q changed type from %s to %s", argName, fieldPath, typeNameOf(oldArg.Type), typeNameOf(newArg.Type))})
		}
	}
	for _, newArg := range newArgs {
		if _, ok := oldArgs[newArg.Name]; ok {
			continue
		}
		path := fieldPath + "(" + newArg.Name + ")"
		if isRequiredInput(newArg.Type, newArg.DefaultValue) {
			changes = append(changes, Change{Breaking, path, fmt.Sprintf("Required argument %q was added to %q", newArg.Name, fieldPath)})
		} else {
			changes = append(changes, Change{NonBreaking, path, fmt.Sprintf("Optional argument %q was added to %q", newArg.Name, fieldPath)})
		}
	}
	return changes
}

func diffInputFieldMaps(typeName string, oldFields, newFields map[string]*Field) []Change {
	var changes []Change
	for fieldName, oldField := range oldFields {
		path := typeName + "." + fieldName
		newField, ok := newFields[fieldName]
		if !ok {
			changes = append(changes, Change{Breaking, path, fmt.Sprintf("Input field %q was removed", path)})
			continue
		}
		if typeNameOf(oldField.Type) != typeNameOf(newField.Type) {
			changes = append(changes, Change{Breaking, path, fmt.Sprintf("Input field %q changed type from %s to %s", path, typeNameOf(oldField.Type), typeNameOf(newField.Type))})
		}
	}
	for fieldName, newField := range newFields {
		if _, ok := oldFields[fieldName]; ok {
			continue
		}
		path := typeName + "." + fieldName
		if isRequiredInput(newField.Type, newField.DefaultValue) {
			changes = append(changes, Change{Breaking, path, fmt.Sprintf("Required input field %q was added", path)})
		} else {
			changes = append(changes, Change{NonBreaking, path, fmt.Sprintf("Optional input field %q was added", path)})
		}
	}
	return changes
}

func diffEnumValues(typeName string, oldEnum, newEnum *Enum) []Change {
	var changes []Change
	oldValues := enumValueSet(oldEnum)
	newValues := enumValueSet(newEnum)
	for value := range oldValues {
		if !newValues[value] {
			changes = append(changes, Change{Breaking, typeName, fmt.Sprintf("Enum value %q was removed from %q", value, typeName)})
		}
	}
	for value := range newValues {
		if !oldValues[value] {
			changes = append(changes, Change{Dangerous, typeName, fmt.Sprintf("Enum value %q was added to %q", value, typeName)})
		}
	}
	return changes
}

func diffUnionMembers(typeName string, oldUnion, newUnion *Union) []Change {
	var changes []Change
	oldMembers := unionMemberSet(oldUnion)
	newMembers := unionMemberSet(newUnion)
	for member := range oldMembers {
		if !newMembers[member] {
			changes = append(changes, Change{Breaking, typeName, fmt.Sprintf("Member %q was removed from union %q", member, typeName)})
		}
	}
	for member := range newMembers {
		if !oldMembers[member] {
			changes = append(changes, Change{Dangerous, typeName, fmt.Sprintf("Member %q was added to union %q", member, typeName)})
		}
	}
	return changes
}

func userTypes(s *Schema) map[string]Type {
	out := make(map[string]Type)
	if s == nil {
		return out
	}
	for name, t := range s.Types {
		if len(name) >= 2 && name[0] == '_' && name[1] == '_' {
			continue // introspection meta types
		}
		if _, ok := t.(*Scalar); ok && isBuiltInScalarName(name) {
			continue
		}
		out[name] = t
	}
	return out
}

func isBuiltInScalarName(name string) bool {
	switch name {
	case "String", "Int", "Float", "Boolean", "ID":
		return true
	default:
		return false
	}
}

func typeNameOf(t Type) string {
	if t == nil {
		return "<nil>"
	}
	return t.Name()
}

func isRequiredInput(t Type, defaultValue any) bool {
	if defaultValue != nil {
		return false
	}
	_, nonNull := t.(*NonNull)
	return nonNull
}

func enumValueSet(e *Enum) map[string]bool {
	out := make(map[string]bool, len(e.Values))
	for _, v := range e.Values {
		out[v.Name] = true
	}
	return out
}

func unionMemberSet(u *Union) map[string]bool {
	out := make(map[string]bool, len(u.Types))
	for _, member := range u.Types {
		if member != nil {
			out[member.Name()] = true
		}
	}
	return out
}

func sortChanges(changes []Change) {
	sort.SliceStable(changes, func(i, j int) bool {
		if changes[i].Severity != changes[j].Severity {
			return changes[i].Severity > changes[j].Severity // breaking first
		}
		if changes[i].Path != changes[j].Path {
			return changes[i].Path < changes[j].Path
		}
		return changes[i].Message < changes[j].Message
	})
}
