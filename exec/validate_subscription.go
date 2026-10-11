package exec

import (
	"strings"

	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/schema"
)

func validateSubscriptionRoot(op document, root *schema.Object) []validationError {
	fields := make(map[string]selection)
	visited := make(map[string]bool)
	var errs []validationError
	var collect func([]selection)
	collect = func(selections []selection) {
		for _, selected := range selections {
			for _, directive := range selected.Directives {
				if directive.Name == "skip" || directive.Name == "include" {
					errs = append(errs, newValidationError(selected.Location, "Subscription root selections must not use @%s.", directive.Name))
				}
			}
			switch {
			case selected.isField():
				fields[selected.responseKey()] = selected
				if strings.HasPrefix(selected.Name, "__") {
					errs = append(errs, newValidationError(selected.Location, "Subscription root field must not be an introspection field."))
				}
			case selected.isFragmentSpread():
				if visited[selected.FragmentSpread] {
					continue
				}
				visited[selected.FragmentSpread] = true
				fragment := op.Fragments[selected.FragmentSpread]
				if fragment != nil && fragmentTypeMatches(root, fragment.TypeCondition) {
					collect(fragment.Selections)
				}
			case selected.isInlineFragment():
				if selected.InlineFragmentOn == "" || fragmentTypeMatches(root, selected.InlineFragmentOn) {
					collect(selected.Selections)
				}
			}
		}
	}
	collect(op.Selections)
	if len(fields) != 1 {
		name := op.Name
		if name == "" {
			name = "anonymous"
		}
		location := core.Location{Line: 1, Column: 1}
		if len(op.Selections) > 0 {
			location = op.Selections[0].Location
		}
		errs = append(errs, newValidationError(location, `Subscription "%s" must select only one top level field.`, name))
	}
	return errs
}
