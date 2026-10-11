package exec

import (
	"fmt"

	"github.com/grx-gql/grx/schema"
)

func extendSDLType(def sdlDef, types map[string]schema.Type) error {
	typ, exists := types[def.name]
	if !exists {
		return fmt.Errorf("cannot extend unknown %s %q", def.kind, def.name)
	}
	if len(def.fields)+len(def.interfaces)+len(def.unionTypes)+len(def.enumValues)+len(def.directives) == 0 {
		return fmt.Errorf("extension of %s %q must add a member, interface, or directive", def.kind, def.name)
	}
	switch def.kind {
	case "type":
		if obj, ok := typ.(*schema.Object); ok {
			if err := validateExtensionFields(obj.Fields, def); err != nil {
				return err
			}
			if err := populateObjectFields(obj, def, types); err != nil {
				return err
			}
			obj.Directives = append(obj.Directives, appliedSDLDirectives(def.directives)...)
			return nil
		}
	case "interface":
		if iface, ok := typ.(*schema.Interface); ok {
			if err := validateExtensionFields(iface.Fields, def); err != nil {
				return err
			}
			if err := populateInterfaceFields(iface, def, types); err != nil {
				return err
			}
			iface.Directives = append(iface.Directives, appliedSDLDirectives(def.directives)...)
			return nil
		}
	case "input":
		if input, ok := typ.(*schema.InputObject); ok {
			if err := validateExtensionFields(input.Fields, def); err != nil {
				return err
			}
			if err := populateInputFields(input, def, types); err != nil {
				return err
			}
			input.Directives = append(input.Directives, appliedSDLDirectives(def.directives)...)
			for _, d := range def.directives {
				if d.Name == "oneOf" {
					return fmt.Errorf("input extension %q must not add @oneOf", def.name)
				}
			}
			return nil
		}
	case "enum":
		if enum, ok := typ.(*schema.Enum); ok {
			seen := make(map[string]bool, len(enum.Values)+len(def.enumValues))
			for _, value := range enum.Values {
				seen[value.Name] = true
			}
			for _, value := range def.enumValues {
				if seen[value.name] {
					return fmt.Errorf("enum extension %q repeats value %q", def.name, value.name)
				}
				seen[value.name] = true
			}
			enum.Values = append(enum.Values, buildEnumValues(def.enumValues)...)
			enum.Directives = append(enum.Directives, appliedSDLDirectives(def.directives)...)
			return nil
		}
	case "union":
		if union, ok := typ.(*schema.Union); ok {
			seen := make(map[string]bool, len(union.Types)+len(def.unionTypes))
			for _, member := range union.Types {
				seen[member.Name()] = true
			}
			for _, name := range def.unionTypes {
				obj, ok := types[name].(*schema.Object)
				if !ok {
					return fmt.Errorf("union extension %q member %q must name an object type", def.name, name)
				}
				if seen[name] {
					return fmt.Errorf("union extension %q repeats member %q", def.name, name)
				}
				seen[name] = true
				union.Types = append(union.Types, obj)
			}
			union.Directives = append(union.Directives, appliedSDLDirectives(def.directives)...)
			return nil
		}
	case "scalar":
		if scalar, ok := typ.(*schema.Scalar); ok {
			scalar.Directives = append(scalar.Directives, appliedSDLDirectives(def.directives)...)
			for _, d := range def.directives {
				if d.Name == "specifiedBy" {
					if url, ok := stringArg(d.Args, "url"); ok {
						scalar.SpecifiedByURL = url
					}
				}
			}
			return nil
		}
	}
	return fmt.Errorf("cannot extend %s %q using %s extension", typ.Kind(), def.name, def.kind)
}

func validateExtensionFields(fields map[string]*schema.Field, def sdlDef) error {
	seen := make(map[string]bool, len(def.fields))
	for _, field := range def.fields {
		if fields[field.name] != nil || seen[field.name] {
			return fmt.Errorf("extension of %q repeats field %q", def.name, field.name)
		}
		seen[field.name] = true
	}
	return nil
}

func appliedSDLDirectives(directives []directive) []schema.AppliedDirective {
	if len(directives) == 0 {
		return nil
	}
	out := make([]schema.AppliedDirective, len(directives))
	for i, d := range directives {
		out[i] = schema.AppliedDirective{Name: d.Name, Args: d.Args}
	}
	return out
}
