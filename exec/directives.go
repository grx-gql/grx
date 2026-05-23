package exec

import (
	"fmt"

	"github.com/patrickkabwe/grx/core"
	"github.com/patrickkabwe/grx/schema"
)

// pendingDefer captures a fragment whose execution was deferred by an active
// @defer directive. The owning selection set supplies the object, source, and
// path when it registers the deferral with the request's incremental collector.
type pendingDefer struct {
	label      string
	selections []selection
}

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
	if ref, ok := v.(variableRef); ok {
		if !ref.HasValue {
			return false, fmt.Errorf("directive argument %q variable $%s is missing", key, ref.Name)
		}
		v = ref.Value
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

func (e *Executor) flattenSelections(collectDefers bool, object *schema.Object, selections []selection, fragments map[string]*fragmentDef) ([]selection, []pendingDefer, []core.Error) {
	var out []selection
	var defers []pendingDefer
	var errs []core.Error
	for _, s := range selections {
		skip, include, err := evalSkipInclude(s.Directives)
		if err != nil {
			errs = append(errs, newFieldError(err.Error(), nil, s.Location))
			continue
		}
		if skip || !include {
			continue
		}

		switch {
		case s.isFragmentSpread():
			fd := fragments[s.FragmentSpread]
			if fd == nil {
				errs = append(errs, newFieldError(fmt.Sprintf(`Unknown fragment "%s".`, s.FragmentSpread), nil, s.Location))
				continue
			}
			if !fragmentTypeMatches(object, fd.TypeCondition) {
				continue
			}
			if collectDefers {
				if active, label := deferDirective(s.Directives); active {
					defers = append(defers, pendingDefer{label: label, selections: fd.Selections})
					continue
				}
			}
			inner, innerDefers, e2 := e.flattenSelections(collectDefers, object, fd.Selections, fragments)
			errs = append(errs, e2...)
			defers = append(defers, innerDefers...)
			out = appendMergedSelections(out, inner)
		case s.isInlineFragment():
			if s.InlineFragmentOn != "" && !fragmentTypeMatches(object, s.InlineFragmentOn) {
				continue
			}
			if collectDefers {
				if active, label := deferDirective(s.Directives); active {
					defers = append(defers, pendingDefer{label: label, selections: s.Selections})
					continue
				}
			}
			inner, innerDefers, e2 := e.flattenSelections(collectDefers, object, s.Selections, fragments)
			errs = append(errs, e2...)
			defers = append(defers, innerDefers...)
			out = appendMergedSelections(out, inner)
		case s.isField():
			out = appendMergedSelection(out, s)
		default:
			errs = append(errs, newFieldError("invalid selection", nil, s.Location))
		}
	}
	return out, defers, errs
}

// deferDirective reports whether a selection carries an active @defer directive
// and returns its label. @defer with `if: false` is treated as absent.
func deferDirective(dirs []directive) (active bool, label string) {
	for _, d := range dirs {
		if d.Name != "defer" {
			continue
		}
		active = true
		if raw, ok := d.Args["if"]; ok {
			if b, ok := resolveDirectiveValue(raw).(bool); ok && !b {
				return false, ""
			}
		}
		if raw, ok := d.Args["label"]; ok {
			if s, ok := resolveDirectiveValue(raw).(string); ok {
				label = s
			}
		}
	}
	return active, label
}

// streamDirective reports whether a selection carries an active @stream
// directive and returns its initialCount and label. @stream with `if: false`
// is treated as absent.
func streamDirective(dirs []directive) (active bool, initialCount int, label string) {
	for _, d := range dirs {
		if d.Name != "stream" {
			continue
		}
		active = true
		if raw, ok := d.Args["if"]; ok {
			if b, ok := resolveDirectiveValue(raw).(bool); ok && !b {
				return false, 0, ""
			}
		}
		if raw, ok := d.Args["initialCount"]; ok {
			switch n := resolveDirectiveValue(raw).(type) {
			case int:
				initialCount = n
			case int64:
				initialCount = int(n)
			case float64:
				initialCount = int(n)
			}
		}
		if raw, ok := d.Args["label"]; ok {
			if s, ok := resolveDirectiveValue(raw).(string); ok {
				label = s
			}
		}
	}
	if initialCount < 0 {
		initialCount = 0
	}
	return active, initialCount, label
}

func resolveDirectiveValue(v any) any {
	if ref, ok := v.(variableRef); ok {
		if ref.HasValue {
			return ref.Value
		}
		return nil
	}
	return v
}

func appendMergedSelections(base []selection, values []selection) []selection {
	for _, value := range values {
		base = appendMergedSelection(base, value)
	}
	return base
}

func appendMergedSelection(values []selection, next selection) []selection {
	keyNext := next.responseKey()
	for index := range values {
		if values[index].responseKey() != keyNext {
			continue
		}
		values[index].Selections = append(values[index].Selections, next.Selections...)
		return values
	}
	return append(values, next)
}
