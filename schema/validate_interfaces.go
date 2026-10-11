package schema

import "fmt"

func validateImplementedInterfaces(name string, fields map[string]*Field, interfaces []*Interface) []SchemaError {
	var errs []SchemaError
	implemented := make(map[string]bool, len(interfaces))
	for _, iface := range interfaces {
		if iface.TypeName == name || implemented[iface.TypeName] {
			errs = append(errs, SchemaError{Message: fmt.Sprintf("Type %s must not implement itself or repeat interface %s.", name, iface.TypeName)})
		}
		implemented[iface.TypeName] = true
	}
	for _, iface := range interfaces {
		for _, inherited := range iface.Interfaces {
			if inherited.TypeName == name || !implemented[inherited.TypeName] {
				errs = append(errs, SchemaError{Message: fmt.Sprintf("Type %s must implement inherited interface %s without a cycle.", name, inherited.TypeName)})
			}
		}
		for fieldName, expected := range iface.Fields {
			field := fields[fieldName]
			if field == nil {
				errs = append(errs, SchemaError{Message: fmt.Sprintf("Type %s must define interface field %s.%s.", name, iface.TypeName, fieldName)})
				continue
			}
			if !isOutputSubtype(field.Type, expected.Type) {
				errs = append(errs, SchemaError{Message: fmt.Sprintf("Field %s.%s must be a subtype of interface field %s.%s.", name, fieldName, iface.TypeName, fieldName)})
			}
			args := make(map[string]InputValue, len(field.Args))
			for _, arg := range field.Args {
				args[arg.Name] = arg
			}
			for _, arg := range expected.Args {
				actual, exists := args[arg.Name]
				if !exists || !sameSchemaType(actual.Type, arg.Type) {
					errs = append(errs, SchemaError{Message: fmt.Sprintf("Field %s.%s argument %s must match interface %s.", name, fieldName, arg.Name, iface.TypeName)})
				}
				delete(args, arg.Name)
			}
			for _, extra := range args {
				if _, required := extra.Type.(*NonNull); required && extra.DefaultValue == nil {
					errs = append(errs, SchemaError{Message: fmt.Sprintf("Field %s.%s must not add required argument %s to interface %s.", name, fieldName, extra.Name, iface.TypeName)})
				}
			}
		}
	}
	return errs
}

func sameSchemaType(a, b Type) bool {
	return a != nil && b != nil && a.Name() == b.Name()
}

func isOutputSubtype(actual, expected Type) bool {
	if sameSchemaType(actual, expected) {
		return true
	}
	if nonnull, ok := expected.(*NonNull); ok {
		actualNonnull, ok := actual.(*NonNull)
		return ok && isOutputSubtype(actualNonnull.OfType, nonnull.OfType)
	}
	if nonnull, ok := actual.(*NonNull); ok {
		return isOutputSubtype(nonnull.OfType, expected)
	}
	if list, ok := expected.(*List); ok {
		actualList, ok := actual.(*List)
		return ok && isOutputSubtype(actualList.OfType, list.OfType)
	}
	switch abstract := expected.(type) {
	case *Interface:
		var interfaces []*Interface
		switch concrete := actual.(type) {
		case *Object:
			interfaces = concrete.Interfaces
		case *Interface:
			interfaces = concrete.Interfaces
		}
		for _, iface := range interfaces {
			if iface.TypeName == abstract.TypeName {
				return true
			}
		}
	case *Union:
		for _, member := range abstract.Types {
			if member == actual {
				return true
			}
		}
	}
	return false
}
