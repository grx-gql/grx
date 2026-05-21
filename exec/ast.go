package exec

import "github.com/patrickkabwe/grx/core"

type operationKind string

const (
	operationQuery        operationKind = "query"
	operationMutation     operationKind = "mutation"
	operationSubscription operationKind = "subscription"
)

// directive is a parsed @name invocation with optional arguments.
type directive struct {
	Name string
	Args map[string]any
}

// fragmentDef is a named fragment definition from the document.
type fragmentDef struct {
	Name          string
	TypeCondition string
	Selections    []selection
	NameOffset    int // source offset of fragment name (for diagnostics)
}

type document struct {
	Kind          operationKind
	Name          string
	Variables     []string
	VariableTypes map[string]string
	Selections    []selection
	// Fragments holds fragment definitions in scope for this operation.
	Fragments map[string]*fragmentDef
}

type variableUse struct {
	Name     string
	Location core.Location
}

type variableRef struct {
	Name     string
	Value    any
	HasValue bool
}

type selection struct {
	Alias            string
	Name             string // GraphQL field name (empty for spreads)
	FragmentSpread   string // non-empty for ...FragmentName
	InlineFragmentOn string // non-empty for ... on Type { ... }
	Arguments        map[string]any
	Directives       []directive
	Selections       []selection
	Location         core.Location
}

func (s selection) responseKey() string {
	if s.Alias != "" {
		return s.Alias
	}
	return s.Name
}

func (s selection) isField() bool {
	return s.Name != "" && s.FragmentSpread == "" && s.InlineFragmentOn == ""
}

func (s selection) isFragmentSpread() bool {
	return s.FragmentSpread != ""
}

func (s selection) isInlineFragment() bool {
	return s.Name == "" && s.FragmentSpread == "" && len(s.Selections) > 0
}
