package exec

// documentBundle holds every operation and fragment definition in a parsed
// GraphQL document before an operation is selected for execution.
type documentBundle struct {
	source          string
	variables       map[string]any
	hasVariableRefs bool
	operations      []document
	fragments       map[string]*fragmentDef
}

// parseDocumentBundle parses query into a bundle of executable operations plus
// fragment definitions before an operation name is resolved.
//
// Prefer [parseDocumentBundleWithLexer] inside the executor so lexer token
// streams can participate in LRU reuse.
func parseDocumentBundle(query string, variables map[string]any, maxDepth int) (documentBundle, error) {
	return parseDocumentBundleWithLexer(nil, query, variables, maxDepth)
}

func parseDocumentBundleWithLexer(e *Executor, query string, variables map[string]any, maxDepth int) (documentBundle, error) {
	source := normalizeSource(query)

	var tokens []token
	var err error
	if e != nil {
		tokens, err = e.sharedLexNormalized(source)
	} else {
		tokens, err = lexNormalizedSource(source)
	}
	if err != nil {
		return documentBundle{}, err
	}

	p := parser{tokens: tokens, source: source, maxDepth: maxDepth}

	fragments := make(map[string]*fragmentDef)
	var operations []document

	for p.peek().kind != tokenEOF {
		useStart := len(p.variableUses)
		if p.peek().kind == tokenName && p.peek().value == "fragment" {
			fd, err := p.parseFragmentDefinition()
			if err != nil {
				return documentBundle{}, err
			}
			if _, dup := fragments[fd.Name]; dup {
				return documentBundle{}, newParseError(p.source, fd.NameOffset,
					`There can be only one fragment named "%s".`, fd.Name)
			}
			fd.VariableUses = p.variableUses[useStart:]
			fragments[fd.Name] = fd
			continue
		}

		op, err := p.parseOperationHeader()
		if err != nil {
			return documentBundle{}, err
		}
		selections, err := p.parseSelectionSet(1)
		if err != nil {
			return documentBundle{}, err
		}
		op.VariableUses = p.variableUses[useStart:]
		op.Selections = selections
		op.Fragments = fragments
		operations = append(operations, op)
	}

	if len(operations) == 0 {
		return documentBundle{}, newParseError(source, 0, "document contains no operations")
	}

	return documentBundle{source: source, variables: variables, hasVariableRefs: len(p.variableUses) > 0, operations: operations, fragments: fragments}, nil
}

func selectOperation(bundle documentBundle, operationName string) (document, error) {
	doc, err := selectOperationDefinition(bundle, operationName)
	if err != nil {
		return document{}, err
	}
	return bindSelectedOperation(bundle, doc), nil
}

func selectOperationDefinition(bundle documentBundle, operationName string) (document, error) {
	if operationName != "" {
		for _, op := range bundle.operations {
			if op.Name == operationName {
				op.Fragments = bundle.fragments
				return op, nil
			}
		}
		return document{}, newParseError(bundle.source, 0,
			`Unknown operation named "%s".`, operationName)
	}

	if len(bundle.operations) > 1 {
		return document{}, newParseError(bundle.source, 0,
			"Must provide operation name if query contains multiple operations.")
	}

	doc := bundle.operations[0]
	doc.Fragments = bundle.fragments
	return doc, nil
}

func resolveDocumentVariableRefs(doc document) document {
	doc.Selections = resolveSelectionVariableRefs(doc.Selections)
	return doc
}

func resolveSelectionVariableRefs(selections []selection) []selection {
	out := make([]selection, len(selections))
	for index, selected := range selections {
		out[index] = selected
		out[index].Arguments = resolveValueMapVariableRefs(selected.Arguments)
		if len(selected.Directives) > 0 {
			out[index].Directives = make([]directive, len(selected.Directives))
			for dirIndex, dir := range selected.Directives {
				out[index].Directives[dirIndex] = dir
				out[index].Directives[dirIndex].Args = resolveValueMapVariableRefs(dir.Args)
			}
		}
		out[index].Selections = resolveSelectionVariableRefs(selected.Selections)
	}
	return out
}

func resolveValueMapVariableRefs(values map[string]any) map[string]any {
	if values == nil {
		return nil
	}
	out := make(map[string]any, len(values))
	for name, value := range values {
		out[name] = resolveValueVariableRef(value)
	}
	return out
}

func resolveValueVariableRef(value any) any {
	switch typed := value.(type) {
	case enumLiteral:
		return string(typed)
	case variableRef:
		if typed.HasValue {
			return resolveValueVariableRef(typed.Value)
		}
		return nil
	case map[string]any:
		return resolveValueMapVariableRefs(typed)
	case []any:
		out := make([]any, len(typed))
		for index, item := range typed {
			out[index] = resolveValueVariableRef(item)
		}
		return out
	default:
		return value
	}
}
