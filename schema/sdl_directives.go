package schema

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

func writeDirectiveDefinitions(b *strings.Builder, definitions map[string]*DirectiveDefinition) {
	names := make([]string, 0, len(definitions))
	for name := range definitions {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		definition := definitions[name]
		writeDescription(b, definition.Description)
		fmt.Fprintf(b, "directive @%s", name)
		if len(definition.Args) > 0 {
			b.WriteByte('(')
			for i, arg := range definition.Args {
				if i > 0 {
					b.WriteString(", ")
				}
				writeInputValue(b, arg, definitions)
			}
			b.WriteByte(')')
		}
		if definition.IsRepeatable {
			b.WriteString(" repeatable")
		}
		fmt.Fprintf(b, " on %s\n\n", strings.Join(definition.Locations, " | "))
	}
}

func writeInputValue(b *strings.Builder, value InputValue, definitions map[string]*DirectiveDefinition) {
	if value.Description != "" {
		fmt.Fprintf(b, "%s ", strconv.Quote(value.Description))
	}
	fmt.Fprintf(b, "%s: %s", value.Name, value.Type.Name())
	if value.DefaultValue != nil {
		fmt.Fprintf(b, " = %s", formatDirectiveValue(value.DefaultValue, value.Type))
	}
	if value.IsDeprecated && !hasAppliedDirective(value.Directives, "deprecated") {
		b.WriteString(" @deprecated")
		if value.DeprecationReason != nil {
			fmt.Fprintf(b, "(reason: %s)", strconv.Quote(*value.DeprecationReason))
		}
	}
	writeAppliedDirectives(b, value.Directives, definitions)
}

func writeAppliedDirectives(b *strings.Builder, directives []AppliedDirective, definitions map[string]*DirectiveDefinition) {
	for _, directive := range directives {
		fmt.Fprintf(b, " @%s", directive.Name)
		if len(directive.Args) == 0 {
			continue
		}
		names := make([]string, 0, len(directive.Args))
		for name := range directive.Args {
			names = append(names, name)
		}
		sort.Strings(names)
		b.WriteByte('(')
		for i, name := range names {
			if i > 0 {
				b.WriteString(", ")
			}
			var typ Type
			if definition := definitions[directive.Name]; definition != nil {
				for _, arg := range definition.Args {
					if arg.Name == name {
						typ = arg.Type
						break
					}
				}
			}
			fmt.Fprintf(b, "%s: %s", name, formatDirectiveValue(directive.Args[name], typ))
		}
		b.WriteByte(')')
	}
}

func hasAppliedDirective(directives []AppliedDirective, name string) bool {
	for _, directive := range directives {
		if directive.Name == name {
			return true
		}
	}
	return false
}

func formatDirectiveValue(value any, typ Type) string {
	if value == nil {
		return "null"
	}
	if nonNull, ok := typ.(*NonNull); ok {
		return formatDirectiveValue(value, nonNull.OfType)
	}
	if _, ok := typ.(*Enum); ok {
		if name, ok := value.(string); ok {
			return name
		}
	}
	switch value := value.(type) {
	case []any:
		var itemType Type
		if list, ok := typ.(*List); ok {
			itemType = list.OfType
		}
		parts := make([]string, len(value))
		for i, item := range value {
			parts[i] = formatDirectiveValue(item, itemType)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case map[string]any:
		names := make([]string, 0, len(value))
		for name := range value {
			names = append(names, name)
		}
		sort.Strings(names)
		parts := make([]string, len(names))
		for i, name := range names {
			var fieldType Type
			if input, ok := typ.(*InputObject); ok && input.Fields[name] != nil {
				fieldType = input.Fields[name].Type
			}
			parts[i] = name + ": " + formatDirectiveValue(value[name], fieldType)
		}
		return "{" + strings.Join(parts, ", ") + "}"
	default:
		if literal, ok := formatSDLDefault(value); ok {
			return literal
		}
		panic(fmt.Sprintf("cannot print SDL directive value of type %T", value))
	}
}
