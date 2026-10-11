package exec

func collectReachableFragments(selections []selection, fragments map[string]*fragmentDef, reachable map[string]bool) {
	if len(fragments) == 0 {
		return
	}
	var visit func([]selection)
	visit = func(selections []selection) {
		for index := range selections {
			sel := &selections[index]
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
}
