package exec

func reachableFragments(selections []selection, fragments map[string]*fragmentDef) map[string]bool {
	if len(fragments) == 0 {
		return nil
	}
	reachable := make(map[string]bool)
	var visit func([]selection)
	visit = func(selections []selection) {
		for _, sel := range selections {
			if sel.isFragmentSpread() && !reachable[sel.FragmentSpread] {
				if fragment := fragments[sel.FragmentSpread]; fragment != nil {
					reachable[sel.FragmentSpread] = true
					visit(fragment.Selections)
				}
			}
			visit(sel.Selections)
		}
	}
	visit(selections)
	return reachable
}
