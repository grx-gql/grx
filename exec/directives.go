package exec

import (
	"fmt"

	"github.com/patrickkabwe/grx/core"
	"github.com/patrickkabwe/grx/schema"
)

func evalSkipInclude(dirs []directive) (skip bool, include bool, err error) {
	skip = false
	include = true
	for _, d := range dirs {
		switch d.Name {
		case "skip":
			v, err := boolArg(d.Args, "if")
			if err != nil {
				return false, true, err
			}
			if v {
				skip = true
			}
		case "include":
			v, err := boolArg(d.Args, "if")
			if err != nil {
				return false, true, err
			}
			if !v {
				include = false
			}
		}
	}
	return skip, include, nil
}

func boolArg(args map[string]any, key string) (bool, error) {
	if args == nil {
		return false, fmt.Errorf("directive missing argument %q", key)
	}
	v, ok := args[key]
	if !ok {
		return false, fmt.Errorf("directive missing argument %q", key)
	}
	b, ok := v.(bool)
	if !ok {
		return false, fmt.Errorf("argument %q must be Boolean", key)
	}
	return b, nil
}

func fragmentTypeMatches(o *schema.Object, condition string) bool {
	if o == nil {
		return false
	}
	if o.Name() == condition {
		return true
	}
	for _, iface := range o.Interfaces {
		if iface != nil && iface.TypeName == condition {
			return true
		}
	}
	return false
}

func (e *Executor) flattenSelections(object *schema.Object, selections []selection, fragments map[string]*fragmentDef) ([]selection, []core.Error) {
	if fragments == nil {
		fragments = map[string]*fragmentDef{}
	}
	var out []selection
	var errs []core.Error
	for _, s := range selections {
		switch {
		case s.isFragmentSpread():
			fd := fragments[s.FragmentSpread]
			if fd == nil {
				errs = append(errs, newFieldError(fmt.Sprintf("unknown fragment %q", s.FragmentSpread), nil, s.Location))
				continue
			}
			if !fragmentTypeMatches(object, fd.TypeCondition) {
				continue
			}
			inner, e2 := e.flattenSelections(object, fd.Selections, fragments)
			errs = append(errs, e2...)
			out = append(out, inner...)
		case s.isInlineFragment():
			if !fragmentTypeMatches(object, s.InlineFragmentOn) {
				continue
			}
			inner, e2 := e.flattenSelections(object, s.Selections, fragments)
			errs = append(errs, e2...)
			out = append(out, inner...)
		case s.isField():
			out = append(out, s)
		default:
			errs = append(errs, newFieldError("invalid selection", nil, s.Location))
		}
	}
	return out, errs
}
