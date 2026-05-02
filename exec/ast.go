package exec

import "github.com/patrickkabwe/grx/core"

type operationKind string

const (
	operationQuery        operationKind = "query"
	operationMutation     operationKind = "mutation"
	operationSubscription operationKind = "subscription"
)

type document struct {
	Kind       operationKind
	Name       string
	Selections []selection
}

type selection struct {
	Name       string
	Arguments  map[string]any
	Selections []selection
	Location   core.Location
}
