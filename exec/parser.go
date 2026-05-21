package exec

import (
	"fmt"
	"strconv"
	"strings"
	"unicode/utf16"
	"unicode/utf8"

	"github.com/patrickkabwe/grx/core"
)

// tokenKind enumerates every lexical category recognised by the GraphQL lexer.
//
// Punctuator and ignored-token coverage follows the October 2021 spec
// (https://spec.graphql.org/October2021/#sec-Source-Text.Lexical-Tokens).
type tokenKind uint8

const (
	tokenEOF tokenKind = iota
	tokenName
	tokenString
	tokenNumber
	tokenBraceOpen
	tokenBraceClose
	tokenParenOpen
	tokenParenClose
	tokenColon
	tokenDollar
	tokenBracketOpen
	tokenBracketClose
	tokenBang
	tokenSpread
	tokenEquals
	tokenAt
	tokenAmp
	tokenPipe
)

func (k tokenKind) String() string {
	switch k {
	case tokenEOF:
		return "EOF"
	case tokenName:
		return "Name"
	case tokenString:
		return "String"
	case tokenNumber:
		return "Number"
	case tokenBraceOpen:
		return "{"
	case tokenBraceClose:
		return "}"
	case tokenParenOpen:
		return "("
	case tokenParenClose:
		return ")"
	case tokenColon:
		return ":"
	case tokenDollar:
		return "$"
	case tokenBracketOpen:
		return "["
	case tokenBracketClose:
		return "]"
	case tokenBang:
		return "!"
	case tokenSpread:
		return "..."
	case tokenEquals:
		return "="
	case tokenAt:
		return "@"
	case tokenAmp:
		return "&"
	case tokenPipe:
		return "|"
	default:
		return "<unknown>"
	}
}

type token struct {
	kind   tokenKind
	value  string
	offset int
}

type parser struct {
	tokens   []token
	index    int
	vars     map[string]any
	source   string
	maxDepth int // 0 = unlimited
}

// parseDocument parses a GraphQL source containing one or more operations and
// returns the single executable operation. When the document defines multiple
// operations the caller must pass a non-empty operationName to disambiguate.
func parseDocument(query string, variables map[string]any) (document, error) {
	return parseDocumentNamed(query, variables, "", 0)
}

// parseDocumentNamed parses every operation in the document and selects the
// one matching operationName. An empty operationName is allowed only when the
// document defines exactly one operation (matching the GraphQL spec rule for
// "GetOperation"). maxDepth limits nested selection set depth (0 = unlimited).
func parseDocumentNamed(query string, variables map[string]any, operationName string, maxDepth int) (document, error) {
	source := normalizeSource(query)
	tokens, err := lex(source)
	if err != nil {
		return document{}, err
	}

	p := parser{tokens: tokens, vars: variables, source: source, maxDepth: maxDepth}

	fragments := make(map[string]*fragmentDef)
	var operations []document

	for p.peek().kind != tokenEOF {
		if p.peek().kind == tokenName && p.peek().value == "fragment" {
			fd, err := p.parseFragmentDefinition()
			if err != nil {
				return document{}, err
			}
			if _, dup := fragments[fd.Name]; dup {
				return document{}, newParseError(p.source, fd.NameOffset, "duplicate fragment %q", fd.Name)
			}
			fragments[fd.Name] = fd
			continue
		}

		kind, name, err := p.parseOperationHeader()
		if err != nil {
			return document{}, err
		}
		selections, err := p.parseSelectionSet(1)
		if err != nil {
			return document{}, err
		}
		operations = append(operations, document{
			Kind:       kind,
			Name:       name,
			Selections: selections,
			Fragments:  fragments,
		})
	}

	if len(operations) == 0 {
		return document{}, newParseError(source, 0, "document contains no operations")
	}

	if operationName != "" {
		for _, op := range operations {
			if op.Name == operationName {
				return op, nil
			}
		}
		return document{}, newParseError(source, 0, "operation %q not found in document", operationName)
	}

	if len(operations) > 1 {
		return document{}, newParseError(source, 0, "must provide operationName when document contains multiple operations")
	}
	return operations[0], nil
}

// parseOperationHeader consumes the optional operation type, name, variable
// definitions, and operation directives, leaving the parser positioned at the
// top-level selection set. It returns the operation kind and (possibly empty)
// operation name.
func (p *parser) parseOperationHeader() (operationKind, string, error) {
	kind := operationQuery
	if p.peek().kind != tokenName {
		return kind, "", nil
	}

	switch p.peek().value {
	case "query":
		p.next()
	case "mutation":
		p.next()
		kind = operationMutation
	case "subscription":
		p.next()
		kind = operationSubscription
	default:
		return kind, "", newParseError(p.source, p.peek().offset, "unexpected token %q at top of operation", p.peek().value)
	}

	var name string
	if p.peek().kind == tokenName {
		name = p.next().value
	}
	if p.peek().kind == tokenParenOpen {
		if err := p.skipBalancedParens(); err != nil {
			return kind, name, err
		}
	}
	if _, err := p.parseDirectives(); err != nil {
		return kind, name, err
	}
	return kind, name, nil
}

func (p *parser) parseSelectionSet(depth int) ([]selection, error) {
	if p.maxDepth > 0 && depth > p.maxDepth {
		return nil, newParseError(p.source, p.peek().offset, "selection depth exceeds limit of %d", p.maxDepth)
	}
	if err := p.expect(tokenBraceOpen); err != nil {
		return nil, err
	}

	selections := make([]selection, 0, 4)
	for p.peek().kind != tokenBraceClose {
		if p.peek().kind == tokenEOF {
			return nil, newParseError(p.source, p.peek().offset, "unexpected end of query inside selection set")
		}
		sel, err := p.parseSelection(depth)
		if err != nil {
			return nil, err
		}
		selections = append(selections, sel)
	}
	p.next() // consume }
	return selections, nil
}

func (p *parser) parseSelection(depth int) (selection, error) {
	if p.peek().kind == tokenSpread {
		p.next() // ...
		if p.peek().kind == tokenName && p.peek().value == "on" {
			p.next() // on
			typeName := p.next()
			if typeName.kind != tokenName {
				return selection{}, newParseError(p.source, typeName.offset, "expected type name after \"on\", got %q", typeName.value)
			}
			nested, err := p.parseSelectionSet(depth + 1)
			if err != nil {
				return selection{}, err
			}
			return selection{
				InlineFragmentOn: typeName.value,
				Selections:       nested,
				Location:         locationForOffset(p.source, typeName.offset),
			}, nil
		}
		fragName := p.next()
		if fragName.kind != tokenName {
			return selection{}, newParseError(p.source, fragName.offset, "expected fragment name, got %q", fragName.value)
		}
		return selection{
			FragmentSpread: fragName.value,
			Location:       locationForOffset(p.source, fragName.offset),
		}, nil
	}

	first := p.next()
	if first.kind != tokenName {
		return selection{}, newParseError(p.source, first.offset, "expected field name or fragment spread, got %q", first.value)
	}

	fieldName := first.value
	locOffset := first.offset
	if p.peek().kind == tokenColon {
		p.next()
		real := p.next()
		if real.kind != tokenName {
			return selection{}, newParseError(p.source, real.offset, "expected field name after alias, got %q", real.value)
		}
		fieldName = real.value
	}

	var args map[string]any
	if p.peek().kind == tokenParenOpen {
		parsed, err := p.parseArguments()
		if err != nil {
			return selection{}, err
		}
		args = parsed
	}

	dirs, err := p.parseDirectives()
	if err != nil {
		return selection{}, err
	}

	var nested []selection
	if p.peek().kind == tokenBraceOpen {
		parsed, err := p.parseSelectionSet(depth + 1)
		if err != nil {
			return selection{}, err
		}
		nested = parsed
	}

	alias := ""
	name := fieldName
	if first.value != fieldName {
		alias = first.value
	}

	return selection{
		Alias:      alias,
		Name:       name,
		Arguments:  args,
		Directives: dirs,
		Selections: nested,
		Location:   locationForOffset(p.source, locOffset),
	}, nil
}

func (p *parser) parseFragmentDefinition() (*fragmentDef, error) {
	if p.peek().kind != tokenName || p.peek().value != "fragment" {
		return nil, newParseError(p.source, p.peek().offset, "expected \"fragment\"")
	}
	p.next()
	nameTok := p.next()
	if nameTok.kind != tokenName {
		return nil, newParseError(p.source, nameTok.offset, "expected fragment name, got %q", nameTok.value)
	}
	if p.peek().kind != tokenName || p.peek().value != "on" {
		return nil, newParseError(p.source, p.peek().offset, "expected \"on\" in fragment definition")
	}
	p.next()
	typeTok := p.next()
	if typeTok.kind != tokenName {
		return nil, newParseError(p.source, typeTok.offset, "expected type condition name, got %q", typeTok.value)
	}
	sel, err := p.parseSelectionSet(1)
	if err != nil {
		return nil, err
	}
	return &fragmentDef{
		Name:          nameTok.value,
		TypeCondition: typeTok.value,
		Selections:    sel,
		NameOffset:    nameTok.offset,
	}, nil
}

func (p *parser) parseDirectives() ([]directive, error) {
	var out []directive
	for p.peek().kind == tokenAt {
		p.next()
		name := p.next()
		if name.kind != tokenName {
			return nil, newParseError(p.source, name.offset, "expected directive name, got %q", name.value)
		}
		d := directive{Name: name.value}
		if p.peek().kind == tokenParenOpen {
			args, err := p.parseArguments()
			if err != nil {
				return nil, err
			}
			d.Args = args
		}
		out = append(out, d)
	}
	return out, nil
}

func (p *parser) parseArguments() (map[string]any, error) {
	p.next() // consume (
	args := make(map[string]any, 4)
	for p.peek().kind != tokenParenClose {
		if p.peek().kind == tokenEOF {
			return nil, newParseError(p.source, p.peek().offset, "unexpected end of query inside arguments")
		}
		name := p.next()
		if name.kind != tokenName {
			return nil, newParseError(p.source, name.offset, "expected argument name, got %q", name.value)
		}
		if err := p.expect(tokenColon); err != nil {
			return nil, err
		}
		value, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		args[name.value] = value
	}
	p.next() // consume )
	return args, nil
}

func (p *parser) parseValue() (any, error) {
	current := p.next()
	switch current.kind {
	case tokenString:
		return current.value, nil
	case tokenNumber:
		// Fast-path: contains '.', 'e', or 'E' implies float.
		if strings.ContainsAny(current.value, ".eE") {
			f, err := strconv.ParseFloat(current.value, 64)
			if err != nil {
				return nil, newParseError(p.source, current.offset, "invalid float literal %q: %s", current.value, err.Error())
			}
			return f, nil
		}
		i, err := strconv.Atoi(current.value)
		if err == nil {
			return i, nil
		}
		// Fall back to int64 for values that overflow int on 32-bit platforms.
		i64, err64 := strconv.ParseInt(current.value, 10, 64)
		if err64 != nil {
			return nil, newParseError(p.source, current.offset, "invalid integer literal %q: %s", current.value, err64.Error())
		}
		return i64, nil
	case tokenName:
		switch current.value {
		case "true":
			return true, nil
		case "false":
			return false, nil
		case "null":
			return nil, nil
		default:
			// Treat as enum value; resolver-side coercion converts it to a typed value.
			return current.value, nil
		}
	case tokenDollar:
		name := p.next()
		if name.kind != tokenName {
			return nil, newParseError(p.source, current.offset, "expected variable name after $")
		}
		value, ok := p.vars[name.value]
		if !ok {
			return nil, newParseError(p.source, name.offset, "missing variable %q", name.value)
		}
		return value, nil
	case tokenBraceOpen:
		return p.parseObjectLiteral()
	case tokenBracketOpen:
		return p.parseListLiteral()
	default:
		return nil, newParseError(p.source, current.offset, "unexpected value token %q", current.value)
	}
}

func (p *parser) parseObjectLiteral() (map[string]any, error) {
	object := make(map[string]any, 4)
	for p.peek().kind != tokenBraceClose {
		if p.peek().kind == tokenEOF {
			return nil, newParseError(p.source, p.peek().offset, "unexpected end of query inside object literal")
		}
		name := p.next()
		if name.kind != tokenName {
			return nil, newParseError(p.source, name.offset, "expected object field name, got %q", name.value)
		}
		if err := p.expect(tokenColon); err != nil {
			return nil, err
		}
		value, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		object[name.value] = value
	}
	p.next() // consume }
	return object, nil
}

func (p *parser) parseListLiteral() ([]any, error) {
	list := make([]any, 0, 4)
	for p.peek().kind != tokenBracketClose {
		if p.peek().kind == tokenEOF {
			return nil, newParseError(p.source, p.peek().offset, "unexpected end of query inside list literal")
		}
		value, err := p.parseValue()
		if err != nil {
			return nil, err
		}
		list = append(list, value)
	}
	p.next() // consume ]
	return list, nil
}

// skipBalancedParens consumes a parenthesised section without interpreting it.
// Used to drop variable definition lists in the operation header until full
// variable validation lands.
func (p *parser) skipBalancedParens() error {
	depth := 0
	for {
		current := p.next()
		switch current.kind {
		case tokenParenOpen:
			depth++
		case tokenParenClose:
			depth--
			if depth == 0 {
				return nil
			}
		case tokenEOF:
			return newParseError(p.source, current.offset, "unexpected end of query inside operation variables")
		}
	}
}

func (p *parser) expect(kind tokenKind) error {
	current := p.next()
	if current.kind != kind {
		return newParseError(p.source, current.offset, "expected token kind %s, got %q", kind, current.value)
	}
	return nil
}

func (p *parser) peek() token {
	return p.tokens[p.index]
}

func (p *parser) next() token {
	current := p.tokens[p.index]
	p.index++
	return current
}

// lex tokenises a GraphQL source string. It is the single hot path; the
// implementation favours raw byte indexing over rune iteration and only falls
// back to UTF-8 decoding when a non-ASCII byte is encountered.
func lex(input string) ([]token, error) {
	input = normalizeSource(input)
	// Heuristic: roughly one token per ~4 source bytes for typical operations.
	tokens := make([]token, 0, len(input)/4+1)
	n := len(input)
	for i := 0; i < n; {
		c := input[i]

		// Insignificant whitespace and commas.
		switch c {
		case ' ', '\t', '\n', '\r', ',':
			i++
			continue
		}

		// Line comments: # ... <line terminator>
		if c == '#' {
			j := i + 1
			for j < n && input[j] != '\n' && input[j] != '\r' {
				j++
			}
			i = j
			continue
		}

		// Single-character punctuators.
		switch c {
		case '{':
			tokens = append(tokens, token{kind: tokenBraceOpen, value: "{", offset: i})
			i++
			continue
		case '}':
			tokens = append(tokens, token{kind: tokenBraceClose, value: "}", offset: i})
			i++
			continue
		case '(':
			tokens = append(tokens, token{kind: tokenParenOpen, value: "(", offset: i})
			i++
			continue
		case ')':
			tokens = append(tokens, token{kind: tokenParenClose, value: ")", offset: i})
			i++
			continue
		case ':':
			tokens = append(tokens, token{kind: tokenColon, value: ":", offset: i})
			i++
			continue
		case '$':
			tokens = append(tokens, token{kind: tokenDollar, value: "$", offset: i})
			i++
			continue
		case '[':
			tokens = append(tokens, token{kind: tokenBracketOpen, value: "[", offset: i})
			i++
			continue
		case ']':
			tokens = append(tokens, token{kind: tokenBracketClose, value: "]", offset: i})
			i++
			continue
		case '!':
			tokens = append(tokens, token{kind: tokenBang, value: "!", offset: i})
			i++
			continue
		case '=':
			tokens = append(tokens, token{kind: tokenEquals, value: "=", offset: i})
			i++
			continue
		case '@':
			tokens = append(tokens, token{kind: tokenAt, value: "@", offset: i})
			i++
			continue
		case '&':
			tokens = append(tokens, token{kind: tokenAmp, value: "&", offset: i})
			i++
			continue
		case '|':
			tokens = append(tokens, token{kind: tokenPipe, value: "|", offset: i})
			i++
			continue
		case '.':
			if i+2 < n && input[i+1] == '.' && input[i+2] == '.' {
				tokens = append(tokens, token{kind: tokenSpread, value: "...", offset: i})
				i += 3
				continue
			}
			return nil, newParseError(input, i, "unexpected character %q", '.')
		case '"':
			// Block string `"""..."""`?
			if i+2 < n && input[i+1] == '"' && input[i+2] == '"' {
				value, next, err := readBlockString(input, i)
				if err != nil {
					return nil, newParseError(input, i, "%s", err.Error())
				}
				tokens = append(tokens, token{kind: tokenString, value: value, offset: i})
				i = next
				continue
			}
			value, next, err := readString(input, i)
			if err != nil {
				return nil, newParseError(input, i, "%s", err.Error())
			}
			tokens = append(tokens, token{kind: tokenString, value: value, offset: i})
			i = next
			continue
		}

		// Names: /[_A-Za-z][_0-9A-Za-z]*/
		if isNameStartByte(c) {
			j := i + 1
			for j < n && isNamePartByte(input[j]) {
				j++
			}
			tokens = append(tokens, token{kind: tokenName, value: input[i:j], offset: i})
			i = j
			continue
		}

		// Numbers: optional '-', integer part, optional fraction, optional exponent.
		if c == '-' || (c >= '0' && c <= '9') {
			value, next, err := readNumber(input, i)
			if err != nil {
				return nil, newParseError(input, i, "%s", err.Error())
			}
			tokens = append(tokens, token{kind: tokenNumber, value: value, offset: i})
			i = next
			continue
		}

		// Non-ASCII byte: decode and report a precise rune in the error.
		if c >= 0x80 {
			r, _ := utf8.DecodeRuneInString(input[i:])
			return nil, newParseError(input, i, "unexpected character %q", r)
		}
		return nil, newParseError(input, i, "unexpected character %q", rune(c))
	}

	tokens = append(tokens, token{kind: tokenEOF, offset: len(input)})
	return tokens, nil
}

type parseError struct {
	message   string
	locations []core.Location
}

func (e parseError) Error() string {
	return e.message
}

func (e parseError) GraphQLLocations() []core.Location {
	return e.locations
}

func normalizeSource(source string) string {
	if len(source) >= 3 && source[0] == 0xEF && source[1] == 0xBB && source[2] == 0xBF {
		return source[3:]
	}
	return source
}

func newParseError(source string, offset int, format string, args ...any) error {
	location := locationForOffset(source, offset)
	return parseError{
		message:   fmt.Sprintf(format, args...),
		locations: []core.Location{location},
	}
}

func locationForOffset(source string, offset int) core.Location {
	line := 1
	column := 1
	if offset < 0 {
		offset = 0
	}
	if offset > len(source) {
		offset = len(source)
	}

	for index := 0; index < offset; index++ {
		switch source[index] {
		case '\n':
			line++
			column = 1
		case '\r':
			line++
			column = 1
			if index+1 < offset && source[index+1] == '\n' {
				index++
			}
		default:
			column++
		}
	}

	return core.Location{Line: line, Column: column}
}

// readString lexes a regular (non-block) string literal. The fast path returns
// a substring of the source when no escape sequences are present, avoiding any
// allocation. The slow path is only entered on the first backslash.
func readString(input string, start int) (string, int, error) {
	n := len(input)
	for j := start + 1; j < n; j++ {
		switch input[j] {
		case '"':
			return input[start+1 : j], j + 1, nil
		case '\\':
			return readStringSlow(input, start+1, j)
		case '\n', '\r':
			return "", 0, fmt.Errorf("unterminated string literal")
		}
	}
	return "", 0, fmt.Errorf("unterminated string literal")
}

func readStringSlow(input string, contentStart, escapeAt int) (string, int, error) {
	n := len(input)
	var b strings.Builder
	b.Grow(n - contentStart)
	b.WriteString(input[contentStart:escapeAt])

	for j := escapeAt; j < n; {
		c := input[j]
		switch c {
		case '"':
			return b.String(), j + 1, nil
		case '\n', '\r':
			return "", 0, fmt.Errorf("unterminated string literal")
		case '\\':
			if j+1 >= n {
				return "", 0, fmt.Errorf("unterminated string literal")
			}
			esc := input[j+1]
			switch esc {
			case '"':
				b.WriteByte('"')
				j += 2
			case '\\':
				b.WriteByte('\\')
				j += 2
			case '/':
				b.WriteByte('/')
				j += 2
			case 'b':
				b.WriteByte('\b')
				j += 2
			case 'f':
				b.WriteByte('\f')
				j += 2
			case 'n':
				b.WriteByte('\n')
				j += 2
			case 'r':
				b.WriteByte('\r')
				j += 2
			case 't':
				b.WriteByte('\t')
				j += 2
			case 'u':
				r, consumed, err := readUnicodeEscape(input, j)
				if err != nil {
					return "", 0, err
				}
				// Combine UTF-16 surrogate pair `\uD83D\uDE00` if present.
				if utf16.IsSurrogate(r) && j+consumed+1 < n && input[j+consumed] == '\\' && input[j+consumed+1] == 'u' {
					r2, consumed2, err2 := readUnicodeEscape(input, j+consumed)
					if err2 == nil && utf16.IsSurrogate(r2) {
						if combined := utf16.DecodeRune(r, r2); combined != utf8.RuneError {
							b.WriteRune(combined)
							j += consumed + consumed2
							continue
						}
					}
				}
				b.WriteRune(r)
				j += consumed
			default:
				return "", 0, fmt.Errorf("invalid string escape %q", input[j:j+2])
			}
		default:
			b.WriteByte(c)
			j++
		}
	}
	return "", 0, fmt.Errorf("unterminated string literal")
}

// readUnicodeEscape parses a `\uXXXX` or `\u{HHHH...}` escape starting at
// input[start] (where start points at the leading backslash). It returns the
// decoded rune and the number of source bytes consumed.
func readUnicodeEscape(input string, start int) (rune, int, error) {
	n := len(input)
	if start+1 >= n || input[start] != '\\' || input[start+1] != 'u' {
		return 0, 0, fmt.Errorf("invalid unicode escape")
	}
	if start+2 < n && input[start+2] == '{' {
		end := start + 3
		for end < n && input[end] != '}' {
			end++
		}
		if end >= n {
			return 0, 0, fmt.Errorf("invalid unicode escape: missing '}'")
		}
		hex := input[start+3 : end]
		if len(hex) == 0 || len(hex) > 8 {
			return 0, 0, fmt.Errorf("invalid unicode escape %q", input[start:end+1])
		}
		code, err := strconv.ParseUint(hex, 16, 32)
		if err != nil {
			return 0, 0, fmt.Errorf("invalid unicode escape %q", input[start:end+1])
		}
		return rune(code), (end + 1) - start, nil
	}
	if start+6 > n {
		return 0, 0, fmt.Errorf("invalid unicode escape")
	}
	hex := input[start+2 : start+6]
	code, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return 0, 0, fmt.Errorf("invalid unicode escape %q", input[start:start+6])
	}
	return rune(code), 6, nil
}

// readBlockString lexes a `"""..."""` block string. The returned value already
// has common-indentation stripping and leading/trailing blank line removal
// applied per the GraphQL specification.
func readBlockString(input string, start int) (string, int, error) {
	n := len(input)
	bodyStart := start + 3
	var raw strings.Builder
	raw.Grow(64)
	for j := bodyStart; j < n; {
		// `\"""` → literal `"""` inside the value.
		if j+3 < n && input[j] == '\\' && input[j+1] == '"' && input[j+2] == '"' && input[j+3] == '"' {
			raw.WriteString(`"""`)
			j += 4
			continue
		}
		if j+2 < n && input[j] == '"' && input[j+1] == '"' && input[j+2] == '"' {
			return blockStringValue(raw.String()), j + 3, nil
		}
		// Allow inner closing `"""` only when escaped; stray `"` are fine.
		if j+1 < n && input[j] == '"' && input[j+1] == '"' && (j+2 >= n || input[j+2] != '"') {
			raw.WriteByte('"')
			raw.WriteByte('"')
			j += 2
			continue
		}
		raw.WriteByte(input[j])
		j++
	}
	return "", 0, fmt.Errorf("unterminated block string")
}

// blockStringValue applies the spec-defined transformation for block strings:
// split on line terminators, compute the common indent across non-first
// non-blank lines, strip it, then drop leading and trailing blank lines.
func blockStringValue(raw string) string {
	lines := splitBlockStringLines(raw)

	commonIndent := -1
	for i := 1; i < len(lines); i++ {
		line := lines[i]
		leading := 0
		for leading < len(line) && (line[leading] == ' ' || line[leading] == '\t') {
			leading++
		}
		if leading == len(line) {
			continue // line is all whitespace; skip
		}
		if commonIndent < 0 || leading < commonIndent {
			commonIndent = leading
		}
	}
	if commonIndent > 0 {
		for i := 1; i < len(lines); i++ {
			if len(lines[i]) >= commonIndent {
				lines[i] = lines[i][commonIndent:]
			}
		}
	}

	for len(lines) > 0 && isBlankLine(lines[0]) {
		lines = lines[1:]
	}
	for len(lines) > 0 && isBlankLine(lines[len(lines)-1]) {
		lines = lines[:len(lines)-1]
	}
	return strings.Join(lines, "\n")
}

func splitBlockStringLines(raw string) []string {
	lines := make([]string, 0, 4)
	start := 0
	for i := 0; i < len(raw); i++ {
		switch raw[i] {
		case '\r':
			lines = append(lines, raw[start:i])
			if i+1 < len(raw) && raw[i+1] == '\n' {
				i++
			}
			start = i + 1
		case '\n':
			lines = append(lines, raw[start:i])
			start = i + 1
		}
	}
	lines = append(lines, raw[start:])
	return lines
}

func isBlankLine(line string) bool {
	for i := 0; i < len(line); i++ {
		if line[i] != ' ' && line[i] != '\t' {
			return false
		}
	}
	return true
}

// readNumber implements the GraphQL spec grammar for IntValue and FloatValue,
// including sign, leading-zero rules, fraction, and exponent. It returns the
// matched substring plus the next read offset.
func readNumber(input string, start int) (string, int, error) {
	n := len(input)
	j := start

	if input[j] == '-' {
		j++
	}
	if j >= n || !isDigitByte(input[j]) {
		return "", 0, fmt.Errorf("invalid number literal")
	}
	if input[j] == '0' {
		j++
		if j < n && isDigitByte(input[j]) {
			return "", 0, fmt.Errorf("invalid number literal: leading zeros are not allowed")
		}
	} else {
		for j < n && isDigitByte(input[j]) {
			j++
		}
	}

	if j < n && input[j] == '.' {
		j++
		if j >= n || !isDigitByte(input[j]) {
			return "", 0, fmt.Errorf("invalid float literal: expected digits after '.'")
		}
		for j < n && isDigitByte(input[j]) {
			j++
		}
	}

	if j < n && (input[j] == 'e' || input[j] == 'E') {
		j++
		if j < n && (input[j] == '+' || input[j] == '-') {
			j++
		}
		if j >= n || !isDigitByte(input[j]) {
			return "", 0, fmt.Errorf("invalid float literal: expected digits in exponent")
		}
		for j < n && isDigitByte(input[j]) {
			j++
		}
	}

	// A name character immediately following a number is a lex error per spec
	// (e.g. `123abc` should not lex as Number followed by Name).
	if j < n && (isNameStartByte(input[j]) || input[j] == '.') {
		return "", 0, fmt.Errorf("invalid number literal %q", input[start:j+1])
	}
	return input[start:j], j, nil
}

func isNameStartByte(c byte) bool {
	return c == '_' || (c >= 'A' && c <= 'Z') || (c >= 'a' && c <= 'z')
}

func isNamePartByte(c byte) bool {
	return isNameStartByte(c) || isDigitByte(c)
}

func isDigitByte(c byte) bool {
	return c >= '0' && c <= '9'
}
