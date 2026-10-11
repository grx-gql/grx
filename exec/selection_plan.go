package exec

import (
	"github.com/grx-gql/grx/core"
	"github.com/grx-gql/grx/schema"
)

type selectionPlanKey struct {
	object        *schema.Object
	first         *selection
	count         int
	collectDefers bool
}

type selectionPlan struct {
	fields []selection
	defers []pendingDefer
	errors []core.Error
}

// Keep the first expanded layout inline so caching a fragment used only once
// does not allocate a map.
type selectionPlanCache struct {
	key        selectionPlanKey
	plan       selectionPlan
	additional map[selectionPlanKey]selectionPlan
}

// prepareSelections reuses field layouts within a request, never resolver values.
// Variables and fragments are fixed for the lifetime of the request's arena;
// concrete types and deferred delivery can select different fields.
func (e *Executor) prepareSelections(arena *pathArena, object *schema.Object, selections []selection, fragments map[string]*fragmentDef) ([]selection, []pendingDefer, []core.Error) {
	if arena == nil {
		return e.flattenSelections(false, object, selections, fragments)
	}
	if len(selections) == 0 {
		return selections, nil, nil
	}
	key := selectionPlanKey{object: object, first: &selections[0], count: len(selections), collectDefers: arena.collector != nil}
	cache := &arena.selectionPlans
	if cache.key == key {
		return cache.plan.fields, cache.plan.defers, cache.plan.errors
	}
	if plan, ok := cache.additional[key]; ok {
		return plan.fields, plan.defers, plan.errors
	}
	if simpleSelections(selections) {
		return selections, nil, nil
	}
	fields, defers, errs := e.flattenSelections(key.collectDefers, object, selections, fragments)
	plan := selectionPlan{fields: fields, defers: defers, errors: errs}
	if cache.key.first == nil {
		cache.key, cache.plan = key, plan
	} else {
		if cache.additional == nil {
			cache.additional = make(map[selectionPlanKey]selectionPlan)
		}
		cache.additional[key] = plan
	}
	return fields, defers, errs
}
