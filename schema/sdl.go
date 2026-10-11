package schema

import (
	"fmt"
	"sort"
	"strconv"
	"strings"
)

// PrintSDL renders a minimal, deterministic SDL document for the built type
// registry. It is intended for developer tooling and optional HTTP export, not
// as a byte-for-byte round-trip of reflection metadata.
func PrintSDL(s *Schema) string {
	if s == nil || s.Types == nil {
		return ""
	}

	names := make([]string, 0, len(s.Types))
	for n := range s.Types {
		if isBuiltInScalarName(n) || strings.HasPrefix(n, "__") {
			continue
		}
		names = append(names, n)
	}
	sort.Strings(names)

	var b strings.Builder
	writeDirectiveDefinitions(&b, s.DirectiveDefinitions)
	for _, name := range names {
		t := s.Types[name]
		switch typed := t.(type) {
		case *Scalar:
			writeDescription(&b, typed.Description)
			fmt.Fprintf(&b, "scalar %s", typed.Name())
			if typed.SpecifiedByURL != "" && !hasAppliedDirective(typed.Directives, "specifiedBy") {
				fmt.Fprintf(&b, " @specifiedBy(url: %s)", strconv.Quote(typed.SpecifiedByURL))
			}
			writeAppliedDirectives(&b, typed.Directives, s.DirectiveDefinitions)
			b.WriteString("\n\n")
		case *Enum:
			writeDescription(&b, typed.Description)
			fmt.Fprintf(&b, "enum %s", typed.Name())
			writeAppliedDirectives(&b, typed.Directives, s.DirectiveDefinitions)
			b.WriteString(" {\n")
			ev := append([]EnumValue(nil), typed.Values...)
			sort.Slice(ev, func(i, j int) bool { return ev[i].Name < ev[j].Name })
			for _, v := range ev {
				if v.Description != "" {
					fmt.Fprintf(&b, "  \"\"\"%s\"\"\"\n", v.Description)
				}
				fmt.Fprintf(&b, "  %s", v.Name)
				if v.IsDeprecated && !hasAppliedDirective(v.Directives, "deprecated") {
					b.WriteString(" @deprecated")
					if v.DeprecationReason != nil && *v.DeprecationReason != "" {
						fmt.Fprintf(&b, "(reason: %s)", strconv.Quote(*v.DeprecationReason))
					}
				}
				writeAppliedDirectives(&b, v.Directives, s.DirectiveDefinitions)
				b.WriteByte('\n')
			}
			b.WriteString("}\n\n")
		case *Union:
			writeDescription(&b, typed.Description)
			parts := make([]string, 0, len(typed.Types))
			for _, o := range typed.Types {
				if o != nil {
					parts = append(parts, o.Name())
				}
			}
			sort.Strings(parts)
			fmt.Fprintf(&b, "union %s", typed.Name())
			writeAppliedDirectives(&b, typed.Directives, s.DirectiveDefinitions)
			fmt.Fprintf(&b, " = %s\n\n", strings.Join(parts, " | "))
		case *Interface:
			writeDescription(&b, typed.Description)
			fmt.Fprintf(&b, "interface %s", typed.Name())
			if len(typed.Interfaces) > 0 {
				ifaces := append([]*Interface(nil), typed.Interfaces...)
				sort.Slice(ifaces, func(i, j int) bool { return ifaces[i].Name() < ifaces[j].Name() })
				b.WriteString(" implements ")
				for i, it := range ifaces {
					if i > 0 {
						b.WriteString(" & ")
					}
					b.WriteString(it.Name())
				}
			}
			writeAppliedDirectives(&b, typed.Directives, s.DirectiveDefinitions)
			writeInterfaceFields(&b, typed, s.DirectiveDefinitions)
		case *InputObject:
			writeDescription(&b, typed.Description)
			fmt.Fprintf(&b, "input %s", typed.Name())
			if typed.IsOneOf && !hasAppliedDirective(typed.Directives, "oneOf") {
				b.WriteString(" @oneOf")
			}
			writeAppliedDirectives(&b, typed.Directives, s.DirectiveDefinitions)
			writeInputFields(&b, typed, s.DirectiveDefinitions)
		case *Object:
			writeDescription(&b, typed.Description)
			fmt.Fprintf(&b, "type %s", typed.Name())
			if len(typed.Interfaces) > 0 {
				iface := append([]*Interface(nil), typed.Interfaces...)
				sort.Slice(iface, func(i, j int) bool { return iface[i].Name() < iface[j].Name() })
				b.WriteString(" implements ")
				for i, it := range iface {
					if i > 0 {
						b.WriteString(" & ")
					}
					b.WriteString(it.Name())
				}
			}
			writeAppliedDirectives(&b, typed.Directives, s.DirectiveDefinitions)
			writeObjectFields(&b, typed, s.DirectiveDefinitions)
		default:
			continue
		}
	}

	b.WriteString("schema")
	writeAppliedDirectives(&b, s.Directives, s.DirectiveDefinitions)
	b.WriteString(" {\n")
	if s.Query != nil {
		fmt.Fprintf(&b, "  query: %s\n", s.Query.Name())
	}
	if s.Mutation != nil {
		fmt.Fprintf(&b, "  mutation: %s\n", s.Mutation.Name())
	}
	if s.Subscription != nil {
		fmt.Fprintf(&b, "  subscription: %s\n", s.Subscription.Name())
	}
	b.WriteString("}\n")
	return b.String()
}

func writeDescription(b *strings.Builder, desc string) {
	if desc == "" {
		return
	}
	if !strings.Contains(desc, "\n") {
		fmt.Fprintf(b, "\"\"\"%s\"\"\"\n", desc)
	} else {
		fmt.Fprintf(b, "\"\"\"\n%s\n\"\"\"\n", desc)
	}
}

func writeInterfaceFields(b *strings.Builder, iface *Interface, definitions map[string]*DirectiveDefinition) {
	b.WriteString(" {\n")
	keys := sortedFieldKeys(iface.Fields)
	for _, k := range keys {
		f := iface.Fields[k]
		fmt.Fprintf(b, "  %s", formatField(f, false, definitions))
	}
	b.WriteString("}\n\n")
}

func writeObjectFields(b *strings.Builder, o *Object, definitions map[string]*DirectiveDefinition) {
	b.WriteString(" {\n")
	keys := sortedFieldKeys(o.Fields)
	for _, k := range keys {
		f := o.Fields[k]
		fmt.Fprintf(b, "  %s", formatField(f, false, definitions))
	}
	b.WriteString("}\n\n")
}

func writeInputFields(b *strings.Builder, in *InputObject, definitions map[string]*DirectiveDefinition) {
	b.WriteString(" {\n")
	keys := sortedFieldKeys(in.Fields)
	for _, k := range keys {
		f := in.Fields[k]
		fmt.Fprintf(b, "  %s", formatField(f, true, definitions))
	}
	b.WriteString("}\n\n")
}

func sortedFieldKeys(fields map[string]*Field) []string {
	keys := make([]string, 0, len(fields))
	for k := range fields {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func formatField(f *Field, input bool, definitions map[string]*DirectiveDefinition) string {
	var sb strings.Builder
	if f.Description != "" {
		fmt.Fprintf(&sb, "  \"\"\"%s\"\"\"\n  ", f.Description)
	}
	sb.WriteString(f.Name)
	if !input && len(f.Args) > 0 {
		sb.WriteByte('(')
		args := append([]InputValue(nil), f.Args...)
		sort.Slice(args, func(i, j int) bool { return args[i].Name < args[j].Name })
		for i, a := range args {
			if i > 0 {
				sb.WriteString(", ")
			}
			writeInputValue(&sb, a, definitions)
		}
		sb.WriteByte(')')
	}
	sb.WriteString(": ")
	sb.WriteString(f.Type.Name())
	if input {
		if s, ok := formatSDLDefault(f.DefaultValue); ok {
			sb.WriteString(" = ")
			sb.WriteString(s)
		}
	}
	if f.IsDeprecated && !hasAppliedDirective(f.Directives, "deprecated") {
		sb.WriteString(" @deprecated")
		if f.DeprecationReason != nil && *f.DeprecationReason != "" {
			fmt.Fprintf(&sb, "(reason: %s)", strconv.Quote(*f.DeprecationReason))
		}
	}
	writeAppliedDirectives(&sb, f.Directives, definitions)
	sb.WriteByte('\n')
	return sb.String()
}

func FormatSDLDefault(v any) (string, bool) { return formatSDLDefault(v) }

func formatSDLDefault(v any) (string, bool) {
	if v == nil {
		return "", false
	}
	switch x := v.(type) {
	case string:
		return strconv.Quote(x), true
	case bool:
		if x {
			return "true", true
		}
		return "false", true
	case float64:
		return strconv.FormatFloat(x, 'f', -1, 64), true
	case int:
		return strconv.Itoa(x), true
	case int32:
		return strconv.FormatInt(int64(x), 10), true
	case int64:
		return strconv.FormatInt(x, 10), true
	default:
		return "", false
	}
}
