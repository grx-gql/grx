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
		names = append(names, n)
	}
	sort.Strings(names)

	var b strings.Builder
	for _, name := range names {
		t := s.Types[name]
		switch typed := t.(type) {
		case *Scalar:
			fmt.Fprintf(&b, "scalar %s\n\n", typed.Name())
		case *Enum:
			fmt.Fprintf(&b, "enum %s {\n", typed.Name())
			ev := append([]EnumValue(nil), typed.Values...)
			sort.Slice(ev, func(i, j int) bool { return ev[i].Name < ev[j].Name })
			for _, v := range ev {
				fmt.Fprintf(&b, "  %s\n", v.Name)
			}
			b.WriteString("}\n\n")
		case *Union:
			parts := make([]string, 0, len(typed.Types))
			for _, o := range typed.Types {
				if o != nil {
					parts = append(parts, o.Name())
				}
			}
			sort.Strings(parts)
			fmt.Fprintf(&b, "union %s = %s\n\n", typed.Name(), strings.Join(parts, " | "))
		case *Interface:
			fmt.Fprintf(&b, "interface %s", typed.Name())
			writeInterfaceFields(&b, typed)
		case *InputObject:
			fmt.Fprintf(&b, "input %s", typed.Name())
			writeInputFields(&b, typed)
		case *Object:
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
			writeObjectFields(&b, typed)
		default:
			continue
		}
	}

	b.WriteString("schema {\n")
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

func writeInterfaceFields(b *strings.Builder, iface *Interface) {
	b.WriteString(" {\n")
	keys := sortedFieldKeys(iface.Fields)
	for _, k := range keys {
		f := iface.Fields[k]
		fmt.Fprintf(b, "  %s", formatField(f, false))
	}
	b.WriteString("}\n\n")
}

func writeObjectFields(b *strings.Builder, o *Object) {
	b.WriteString(" {\n")
	keys := sortedFieldKeys(o.Fields)
	for _, k := range keys {
		f := o.Fields[k]
		fmt.Fprintf(b, "  %s", formatField(f, false))
	}
	b.WriteString("}\n\n")
}

func writeInputFields(b *strings.Builder, in *InputObject) {
	b.WriteString(" {\n")
	keys := sortedFieldKeys(in.Fields)
	for _, k := range keys {
		f := in.Fields[k]
		fmt.Fprintf(b, "  %s", formatField(f, true))
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

func formatField(f *Field, input bool) string {
	var sb strings.Builder
	sb.WriteString(f.Name)
	if !input && len(f.Args) > 0 {
		sb.WriteByte('(')
		args := append([]InputValue(nil), f.Args...)
		sort.Slice(args, func(i, j int) bool { return args[i].Name < args[j].Name })
		for i, a := range args {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(a.Name)
			sb.WriteString(": ")
			sb.WriteString(a.Type.Name())
			if s, ok := formatSDLDefault(a.DefaultValue); ok {
				sb.WriteString(" = ")
				sb.WriteString(s)
			}
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
	sb.WriteByte('\n')
	return sb.String()
}

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
