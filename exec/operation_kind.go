package exec

import (
	"fmt"

	"github.com/patrickkabwe/grx/core"
)

func parseOperationKind(query string, operationName string) (core.OperationKind, error) {
	source := normalizeSource(query)
	tokens, err := lex(source)
	if err != nil {
		return "", err
	}

	foundOperations := 0
	var anonymousKind core.OperationKind
	for index := 0; tokens[index].kind != tokenEOF; {
		current := tokens[index]
		if current.kind == tokenBraceOpen {
			foundOperations++
			anonymousKind = core.OperationQuery
			if operationName == "" && foundOperations == 1 {
				if err := skipSelectionTokens(tokens, &index); err != nil {
					return "", err
				}
				continue
			}
			if err := skipSelectionTokens(tokens, &index); err != nil {
				return "", err
			}
			continue
		}
		if current.kind != tokenName {
			return "", newParseError(source, current.offset, "unexpected token %q at top of operation", current.value)
		}
		if current.value == "fragment" {
			index++
			if err := skipUntilSelection(tokens, &index); err != nil {
				return "", err
			}
			continue
		}

		kind, ok := operationKindFromName(current.value)
		if !ok {
			return "", newParseError(source, current.offset, "unexpected token %q at top of operation", current.value)
		}
		foundOperations++
		index++

		name := ""
		if tokens[index].kind == tokenName {
			name = tokens[index].value
			index++
		}
		if operationName != "" && name == operationName {
			return kind, nil
		}
		if err := skipUntilSelection(tokens, &index); err != nil {
			return "", err
		}
	}

	if operationName != "" {
		return "", newParseError(source, 0, `Unknown operation named "%s".`, operationName)
	}
	if foundOperations != 1 {
		return "", newParseError(source, 0, "Must provide operation name if query contains multiple operations.")
	}
	return anonymousKind, nil
}

func operationKindFromName(name string) (core.OperationKind, bool) {
	switch name {
	case "query":
		return core.OperationQuery, true
	case "mutation":
		return core.OperationMutation, true
	case "subscription":
		return core.OperationSubscription, true
	default:
		return "", false
	}
}

func skipUntilSelection(tokens []token, index *int) error {
	parenDepth := 0
	bracketDepth := 0
	for tokens[*index].kind != tokenEOF {
		switch tokens[*index].kind {
		case tokenParenOpen:
			parenDepth++
		case tokenParenClose:
			if parenDepth > 0 {
				parenDepth--
			}
		case tokenBracketOpen:
			bracketDepth++
		case tokenBracketClose:
			if bracketDepth > 0 {
				bracketDepth--
			}
		case tokenBraceOpen:
			if parenDepth == 0 && bracketDepth == 0 {
				return skipSelectionTokens(tokens, index)
			}
		}
		(*index)++
	}
	if tokens[*index].kind != tokenBraceOpen {
		return fmt.Errorf("operation is missing selection set")
	}
	return skipSelectionTokens(tokens, index)
}

func skipSelectionTokens(tokens []token, index *int) error {
	depth := 0
	for tokens[*index].kind != tokenEOF {
		switch tokens[*index].kind {
		case tokenBraceOpen:
			depth++
		case tokenBraceClose:
			depth--
			if depth == 0 {
				(*index)++
				return nil
			}
		}
		(*index)++
	}
	return fmt.Errorf("unexpected end of query inside selection set")
}
