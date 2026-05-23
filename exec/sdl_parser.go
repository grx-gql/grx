package exec

import (
	"fmt"
	"strings"

	"github.com/patrickkabwe/grx/schema"
)

// ParseSDL parses a GraphQL SDL document and returns a Schema containing
// all the defined types. The returned schema has no resolvers; it is
// suitable for tooling, validation, and round-trip testing.
//
// Covers: type definitions, directive definitions, description strings,
// implements lists, union members, @specifiedBy, @oneOf, @deprecated,
// repeatable directive declarations, and extend definitions.
func ParseSDL(source string) (*schema.Schema, error) {
	source = normalizeSource(source)
	tokens, err := lexNormalizedSource(source)
	if err != nil {
		return nil, err
	}
	p := &sdlParser{tokens: tokens, source: source}
	return p.parse()
}

// ---- internal SDL parser ----

type sdlParser struct {
	tokens []token
	index  int
	source string
}

func (p *sdlParser) peek() token         { return p.tokens[p.index] }
func (p *sdlParser) next() token         { t := p.tokens[p.index]; p.index++; return t }
func (p *sdlParser) expect(k tokenKind) (token, error) {
	t := p.next()
	if t.kind != k {
		return token{}, newParseError(p.source, t.offset, "expected %s, got %q", k, t.value)
	}
	return t, nil
}

// sdlTypeRef is the intermediate representation of a parsed type reference.
type sdlTypeRef struct {
	name     string     // named type
	isList   bool       // [inner]
	isNonNull bool      // T!
	inner    *sdlTypeRef // for list types
}

func (r sdlTypeRef) resolve(types map[string]schema.Type) (schema.Type, error) {
	var base schema.Type
	if r.isList {
		inner, err := r.inner.resolve(types)
		if err != nil {
			return nil, err
		}
		base = &schema.List{OfType: inner}
	} else {
		t, ok := types[r.name]
		if !ok {
			return nil, fmt.Errorf("unknown type %q referenced in SDL", r.name)
		}
		base = t
	}
	if r.isNonNull {
		return &schema.NonNull{OfType: base}, nil
	}
	return base, nil
}

// sdlFieldDef is the intermediate representation of a field or input-field.
type sdlFieldDef struct {
	name        string
	description string
	typeRef     sdlTypeRef
	args        []sdlArgDef
	directives  []directive
}

type sdlArgDef struct {
	name         string
	description  string
	typeRef      sdlTypeRef
	defaultValue any
	hasDefault   bool
}

type sdlEnumValueDef struct {
	name        string
	description string
	directives  []directive
}

// sdlDef holds one parsed SDL definition (or extend).
type sdlDef struct {
	kind        string // "scalar","type","interface","union","enum","input","schema","directive"
	isExtend    bool
	name        string
	description string
	fields      []sdlFieldDef
	interfaces  []string
	unionTypes  []string
	enumValues  []sdlEnumValueDef
	directives  []directive // directives applied to the definition itself
	// directive-definition specific:
	isRepeatable bool
	locations    []string
	// schema-definition specific:
	schemaQuery        string
	schemaMutation     string
	schemaSubscription string
}

// ---- top-level parse ----

func (p *sdlParser) parse() (*schema.Schema, error) {
	var defs []sdlDef
	for p.peek().kind != tokenEOF {
		def, err := p.parseDefinition()
		if err != nil {
			return nil, err
		}
		defs = append(defs, def)
	}
	return buildSchemaFromDefs(defs)
}

// parseDefinition parses one top-level SDL definition.
func (p *sdlParser) parseDefinition() (sdlDef, error) {
	// Optional leading description (block string or regular string).
	description := ""
	if p.peek().kind == tokenString {
		description = p.next().value
	}

	isExtend := false
	if p.peek().kind == tokenName && p.peek().value == "extend" {
		p.next()
		isExtend = true
	}

	tok := p.peek()
	if tok.kind != tokenName {
		return sdlDef{}, newParseError(p.source, tok.offset, "expected type keyword, got %q", tok.value)
	}

	switch tok.value {
	case "scalar":
		return p.parseScalarDef(description, isExtend)
	case "type":
		return p.parseObjectDef(description, isExtend)
	case "interface":
		return p.parseInterfaceDef(description, isExtend)
	case "union":
		return p.parseUnionDef(description, isExtend)
	case "enum":
		return p.parseEnumDef(description, isExtend)
	case "input":
		return p.parseInputDef(description, isExtend)
	case "schema":
		return p.parseSchemaDef(description, isExtend)
	case "directive":
		return p.parseDirectiveDef(description)
	default:
		return sdlDef{}, newParseError(p.source, tok.offset, "unexpected keyword %q in SDL", tok.value)
	}
}

func (p *sdlParser) parseScalarDef(description string, isExtend bool) (sdlDef, error) {
	p.next() // consume "scalar"
	nameTok, err := p.expect(tokenName)
	if err != nil {
		return sdlDef{}, err
	}
	dirs, err := p.parseSDLDirectives()
	if err != nil {
		return sdlDef{}, err
	}
	return sdlDef{kind: "scalar", isExtend: isExtend, name: nameTok.value, description: description, directives: dirs}, nil
}

func (p *sdlParser) parseObjectDef(description string, isExtend bool) (sdlDef, error) {
	p.next() // consume "type"
	nameTok, err := p.expect(tokenName)
	if err != nil {
		return sdlDef{}, err
	}
	// optional implements
	var implements []string
	if p.peek().kind == tokenName && p.peek().value == "implements" {
		p.next()
		// optional leading &
		if p.peek().kind == tokenAmp {
			p.next()
		}
		for p.peek().kind == tokenName {
			implements = append(implements, p.next().value)
			if p.peek().kind == tokenAmp {
				p.next()
			} else {
				break
			}
		}
	}
	dirs, err := p.parseSDLDirectives()
	if err != nil {
		return sdlDef{}, err
	}
	var fields []sdlFieldDef
	if p.peek().kind == tokenBraceOpen {
		fields, err = p.parseFieldDefs()
		if err != nil {
			return sdlDef{}, err
		}
	}
	return sdlDef{kind: "type", isExtend: isExtend, name: nameTok.value, description: description, interfaces: implements, directives: dirs, fields: fields}, nil
}

func (p *sdlParser) parseInterfaceDef(description string, isExtend bool) (sdlDef, error) {
	p.next() // consume "interface"
	nameTok, err := p.expect(tokenName)
	if err != nil {
		return sdlDef{}, err
	}
	// optional implements
	var implements []string
	if p.peek().kind == tokenName && p.peek().value == "implements" {
		p.next()
		if p.peek().kind == tokenAmp {
			p.next()
		}
		for p.peek().kind == tokenName {
			implements = append(implements, p.next().value)
			if p.peek().kind == tokenAmp {
				p.next()
			} else {
				break
			}
		}
	}
	dirs, err := p.parseSDLDirectives()
	if err != nil {
		return sdlDef{}, err
	}
	fields, err := p.parseFieldDefs()
	if err != nil {
		return sdlDef{}, err
	}
	return sdlDef{kind: "interface", isExtend: isExtend, name: nameTok.value, description: description, interfaces: implements, directives: dirs, fields: fields}, nil
}

func (p *sdlParser) parseUnionDef(description string, isExtend bool) (sdlDef, error) {
	p.next() // consume "union"
	nameTok, err := p.expect(tokenName)
	if err != nil {
		return sdlDef{}, err
	}
	dirs, err := p.parseSDLDirectives()
	if err != nil {
		return sdlDef{}, err
	}
	var members []string
	if p.peek().kind == tokenEquals {
		p.next() // consume =
		// optional leading |
		if p.peek().kind == tokenPipe {
			p.next()
		}
		for p.peek().kind == tokenName {
			members = append(members, p.next().value)
			if p.peek().kind == tokenPipe {
				p.next()
			} else {
				break
			}
		}
	}
	return sdlDef{kind: "union", isExtend: isExtend, name: nameTok.value, description: description, directives: dirs, unionTypes: members}, nil
}

func (p *sdlParser) parseEnumDef(description string, isExtend bool) (sdlDef, error) {
	p.next() // consume "enum"
	nameTok, err := p.expect(tokenName)
	if err != nil {
		return sdlDef{}, err
	}
	dirs, err := p.parseSDLDirectives()
	if err != nil {
		return sdlDef{}, err
	}
	if _, err := p.expect(tokenBraceOpen); err != nil {
		return sdlDef{}, err
	}
	var values []sdlEnumValueDef
	for p.peek().kind != tokenBraceClose {
		if p.peek().kind == tokenEOF {
			return sdlDef{}, newParseError(p.source, p.peek().offset, "unexpected EOF inside enum")
		}
		desc := ""
		if p.peek().kind == tokenString {
			desc = p.next().value
		}
		valueTok, err := p.expect(tokenName)
		if err != nil {
			return sdlDef{}, err
		}
		valDirs, err := p.parseSDLDirectives()
		if err != nil {
			return sdlDef{}, err
		}
		values = append(values, sdlEnumValueDef{name: valueTok.value, description: desc, directives: valDirs})
	}
	p.next() // consume }
	return sdlDef{kind: "enum", isExtend: isExtend, name: nameTok.value, description: description, directives: dirs, enumValues: values}, nil
}

func (p *sdlParser) parseInputDef(description string, isExtend bool) (sdlDef, error) {
	p.next() // consume "input"
	nameTok, err := p.expect(tokenName)
	if err != nil {
		return sdlDef{}, err
	}
	dirs, err := p.parseSDLDirectives()
	if err != nil {
		return sdlDef{}, err
	}
	fields, err := p.parseInputFieldDefs()
	if err != nil {
		return sdlDef{}, err
	}
	return sdlDef{kind: "input", isExtend: isExtend, name: nameTok.value, description: description, directives: dirs, fields: fields}, nil
}

func (p *sdlParser) parseSchemaDef(description string, isExtend bool) (sdlDef, error) {
	p.next() // consume "schema"
	dirs, err := p.parseSDLDirectives()
	if err != nil {
		return sdlDef{}, err
	}
	def := sdlDef{kind: "schema", isExtend: isExtend, description: description, directives: dirs}
	if p.peek().kind != tokenBraceOpen {
		return def, nil
	}
	p.next() // consume {
	for p.peek().kind != tokenBraceClose {
		if p.peek().kind == tokenEOF {
			return sdlDef{}, newParseError(p.source, p.peek().offset, "unexpected EOF inside schema")
		}
		keyTok, err := p.expect(tokenName)
		if err != nil {
			return sdlDef{}, err
		}
		if _, err := p.expect(tokenColon); err != nil {
			return sdlDef{}, err
		}
		valTok, err := p.expect(tokenName)
		if err != nil {
			return sdlDef{}, err
		}
		switch keyTok.value {
		case "query":
			def.schemaQuery = valTok.value
		case "mutation":
			def.schemaMutation = valTok.value
		case "subscription":
			def.schemaSubscription = valTok.value
		}
	}
	p.next() // consume }
	return def, nil
}

func (p *sdlParser) parseDirectiveDef(description string) (sdlDef, error) {
	p.next() // consume "directive"
	if _, err := p.expect(tokenAt); err != nil {
		return sdlDef{}, err
	}
	nameTok, err := p.expect(tokenName)
	if err != nil {
		return sdlDef{}, err
	}
	// optional arguments
	var args []sdlArgDef
	if p.peek().kind == tokenParenOpen {
		args, err = p.parseSDLArgDefs()
		if err != nil {
			return sdlDef{}, err
		}
	}
	isRepeatable := false
	if p.peek().kind == tokenName && p.peek().value == "repeatable" {
		p.next()
		isRepeatable = true
	}
	if p.peek().kind != tokenName || p.peek().value != "on" {
		return sdlDef{}, newParseError(p.source, p.peek().offset, "expected \"on\" in directive definition")
	}
	p.next() // consume "on"
	// optional leading |
	if p.peek().kind == tokenPipe {
		p.next()
	}
	var locations []string
	for p.peek().kind == tokenName {
		locations = append(locations, p.next().value)
		if p.peek().kind == tokenPipe {
			p.next()
		} else {
			break
		}
	}
	// Build args as sdlFieldDefs for storage in the sdlDef
	argFields := make([]sdlFieldDef, 0, len(args))
	for _, a := range args {
		argFields = append(argFields, sdlFieldDef{name: a.name, description: a.description, typeRef: a.typeRef})
	}
	return sdlDef{
		kind:         "directive",
		name:         nameTok.value,
		description:  description,
		isRepeatable: isRepeatable,
		locations:    locations,
		fields:       argFields,
	}, nil
}

// parseFieldDefs parses { field: Type @dir ... } for object/interface types.
func (p *sdlParser) parseFieldDefs() ([]sdlFieldDef, error) {
	if _, err := p.expect(tokenBraceOpen); err != nil {
		return nil, err
	}
	var fields []sdlFieldDef
	for p.peek().kind != tokenBraceClose {
		if p.peek().kind == tokenEOF {
			return nil, newParseError(p.source, p.peek().offset, "unexpected EOF inside field definitions")
		}
		desc := ""
		if p.peek().kind == tokenString {
			desc = p.next().value
		}
		nameTok, err := p.expect(tokenName)
		if err != nil {
			return nil, err
		}
		var args []sdlArgDef
		if p.peek().kind == tokenParenOpen {
			args, err = p.parseSDLArgDefs()
			if err != nil {
				return nil, err
			}
		}
		if _, err := p.expect(tokenColon); err != nil {
			return nil, err
		}
		typeRef, err := p.parseSDLTypeRef()
		if err != nil {
			return nil, err
		}
		dirs, err := p.parseSDLDirectives()
		if err != nil {
			return nil, err
		}
		fields = append(fields, sdlFieldDef{
			name:        nameTok.value,
			description: desc,
			typeRef:     typeRef,
			args:        args,
			directives:  dirs,
		})
	}
	p.next() // consume }
	return fields, nil
}

// parseInputFieldDefs parses { field: Type = default @dir ... } for input types.
func (p *sdlParser) parseInputFieldDefs() ([]sdlFieldDef, error) {
	if _, err := p.expect(tokenBraceOpen); err != nil {
		return nil, err
	}
	var fields []sdlFieldDef
	for p.peek().kind != tokenBraceClose {
		if p.peek().kind == tokenEOF {
			return nil, newParseError(p.source, p.peek().offset, "unexpected EOF inside input field definitions")
		}
		desc := ""
		if p.peek().kind == tokenString {
			desc = p.next().value
		}
		nameTok, err := p.expect(tokenName)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tokenColon); err != nil {
			return nil, err
		}
		typeRef, err := p.parseSDLTypeRef()
		if err != nil {
			return nil, err
		}
		var defaultVal any
		hasDefault := false
		if p.peek().kind == tokenEquals {
			p.next()
			// Reuse the executable parser's const-value parser via a temp parser.
			ep := &parser{tokens: p.tokens, index: p.index, source: p.source}
			defaultVal, err = ep.parseConstValue()
			if err != nil {
				return nil, err
			}
			p.index = ep.index
			hasDefault = true
		}
		dirs, err := p.parseSDLDirectives()
		if err != nil {
			return nil, err
		}
		fields = append(fields, sdlFieldDef{
			name:        nameTok.value,
			description: desc,
			typeRef:     typeRef,
			directives:  dirs,
			args: func() []sdlArgDef {
				if hasDefault {
					return []sdlArgDef{{defaultValue: defaultVal, hasDefault: true}}
				}
				return nil
			}(),
		})
	}
	p.next() // consume }
	return fields, nil
}

// parseSDLArgDefs parses (arg: Type = default ...) for field arguments and directive arguments.
func (p *sdlParser) parseSDLArgDefs() ([]sdlArgDef, error) {
	p.next() // consume (
	var args []sdlArgDef
	for p.peek().kind != tokenParenClose {
		if p.peek().kind == tokenEOF {
			return nil, newParseError(p.source, p.peek().offset, "unexpected EOF inside argument definitions")
		}
		desc := ""
		if p.peek().kind == tokenString {
			desc = p.next().value
		}
		nameTok, err := p.expect(tokenName)
		if err != nil {
			return nil, err
		}
		if _, err := p.expect(tokenColon); err != nil {
			return nil, err
		}
		typeRef, err := p.parseSDLTypeRef()
		if err != nil {
			return nil, err
		}
		var defaultVal any
		hasDefault := false
		if p.peek().kind == tokenEquals {
			p.next()
			ep := &parser{tokens: p.tokens, index: p.index, source: p.source}
			defaultVal, err = ep.parseConstValue()
			if err != nil {
				return nil, err
			}
			p.index = ep.index
			hasDefault = true
		}
		// Discard directives on arg definitions.
		dirs := make([]directive, 0)
		for p.peek().kind == tokenAt {
			p.next()
			if _, err2 := p.expect(tokenName); err2 != nil {
				return nil, err2
			}
			if p.peek().kind == tokenParenOpen {
				ep := &parser{tokens: p.tokens, index: p.index, source: p.source}
				if _, err2 := ep.parseArguments(); err2 != nil {
					return nil, err2
				}
				p.index = ep.index
			}
		}
		_ = dirs
		args = append(args, sdlArgDef{name: nameTok.value, description: desc, typeRef: typeRef, defaultValue: defaultVal, hasDefault: hasDefault})
	}
	p.next() // consume )
	return args, nil
}

// parseSDLTypeRef parses a GraphQL type reference: Name, [Name], Name!, [Name!]!
func (p *sdlParser) parseSDLTypeRef() (sdlTypeRef, error) {
	if p.peek().kind == tokenBracketOpen {
		p.next() // consume [
		inner, err := p.parseSDLTypeRef()
		if err != nil {
			return sdlTypeRef{}, err
		}
		if _, err := p.expect(tokenBracketClose); err != nil {
			return sdlTypeRef{}, err
		}
		ref := sdlTypeRef{isList: true, inner: &inner}
		if p.peek().kind == tokenBang {
			p.next()
			ref.isNonNull = true
		}
		return ref, nil
	}
	nameTok, err := p.expect(tokenName)
	if err != nil {
		return sdlTypeRef{}, err
	}
	ref := sdlTypeRef{name: nameTok.value}
	if p.peek().kind == tokenBang {
		p.next()
		ref.isNonNull = true
	}
	return ref, nil
}

// parseSDLDirectives parses zero or more @name(args) directive applications.
func (p *sdlParser) parseSDLDirectives() ([]directive, error) {
	var out []directive
	for p.peek().kind == tokenAt {
		p.next()
		nameTok, err := p.expect(tokenName)
		if err != nil {
			return nil, err
		}
		d := directive{Name: nameTok.value}
		if p.peek().kind == tokenParenOpen {
			ep := &parser{tokens: p.tokens, index: p.index, source: p.source}
			args, err := ep.parseArguments()
			if err != nil {
				return nil, err
			}
			p.index = ep.index
			d.Args = args
		}
		out = append(out, d)
	}
	return out, nil
}

// ---- schema builder ----

func buildSchemaFromDefs(defs []sdlDef) (*schema.Schema, error) {
	// Built-in scalar types always available.
	types := map[string]schema.Type{
		"String":  &schema.Scalar{TypeName: "String"},
		"Int":     &schema.Scalar{TypeName: "Int"},
		"Float":   &schema.Scalar{TypeName: "Float"},
		"Boolean": &schema.Scalar{TypeName: "Boolean"},
		"ID":      &schema.Scalar{TypeName: "ID"},
	}
	dirDefs := map[string]*schema.DirectiveDefinition{}

	// Pass 1: register all named types (without resolving references).
	for _, def := range defs {
		if def.kind == "directive" || def.kind == "schema" || def.isExtend {
			continue
		}
		if _, exists := types[def.name]; exists {
			return nil, fmt.Errorf("type %q is defined more than once", def.name)
		}
		switch def.kind {
		case "scalar":
			s := &schema.Scalar{TypeName: def.name, Description: def.description}
			for _, d := range def.directives {
				if d.Name == "specifiedBy" {
					if url, ok := stringArg(d.Args, "url"); ok {
						s.SpecifiedByURL = url
					}
				}
			}
			types[def.name] = s
		case "type":
			types[def.name] = &schema.Object{TypeName: def.name, Description: def.description, Fields: map[string]*schema.Field{}}
		case "interface":
			types[def.name] = &schema.Interface{TypeName: def.name, Description: def.description, Fields: map[string]*schema.Field{}}
		case "union":
			types[def.name] = &schema.Union{TypeName: def.name, Description: def.description}
		case "enum":
			ev := buildEnumValues(def.enumValues)
			e := &schema.Enum{TypeName: def.name, Description: def.description, Values: ev}
			types[def.name] = e
		case "input":
			types[def.name] = &schema.InputObject{TypeName: def.name, Description: def.description, Fields: map[string]*schema.Field{}}
		}
	}

	// Pass 2: resolve fields, interfaces, unions, directives.
	for _, def := range defs {
		if def.isExtend || def.kind == "schema" {
			continue
		}
		switch def.kind {
		case "type":
			obj := types[def.name].(*schema.Object)
			if err := populateObjectFields(obj, def, types); err != nil {
				return nil, err
			}
			for _, d := range def.directives {
				obj.Directives = append(obj.Directives, schema.AppliedDirective{Name: d.Name, Args: d.Args})
			}
		case "interface":
			iface := types[def.name].(*schema.Interface)
			if err := populateInterfaceFields(iface, def, types); err != nil {
				return nil, err
			}
		case "union":
			union := types[def.name].(*schema.Union)
			for _, member := range def.unionTypes {
				t, ok := types[member]
				if !ok {
					return nil, fmt.Errorf("unknown type %q in union %q", member, def.name)
				}
				obj, ok := t.(*schema.Object)
				if !ok {
					return nil, fmt.Errorf("union member %q must be an object type", member)
				}
				union.Types = append(union.Types, obj)
			}
		case "input":
			inputObj := types[def.name].(*schema.InputObject)
			if err := populateInputFields(inputObj, def, types); err != nil {
				return nil, err
			}
			for _, d := range def.directives {
				if d.Name == "oneOf" {
					inputObj.IsOneOf = true
				}
				inputObj.Directives = append(inputObj.Directives, schema.AppliedDirective{Name: d.Name, Args: d.Args})
			}
		case "directive":
			dd := &schema.DirectiveDefinition{
				Name:         def.name,
				Description:  def.description,
				Locations:    def.locations,
				IsRepeatable: def.isRepeatable,
			}
			for _, f := range def.fields {
				argType, err := f.typeRef.resolve(types)
				if err != nil {
					return nil, err
				}
				dd.Args = append(dd.Args, schema.InputValue{Name: f.name, Description: f.description, Type: argType})
			}
			dirDefs[def.name] = dd
		}
	}

	// Pass 3: process extend definitions.
	for _, def := range defs {
		if !def.isExtend {
			continue
		}
		switch def.kind {
		case "type":
			obj, ok := types[def.name].(*schema.Object)
			if !ok {
				return nil, fmt.Errorf("cannot extend unknown type %q", def.name)
			}
			if err := populateObjectFields(obj, def, types); err != nil {
				return nil, err
			}
		case "input":
			inputObj, ok := types[def.name].(*schema.InputObject)
			if !ok {
				return nil, fmt.Errorf("cannot extend unknown input type %q", def.name)
			}
			if err := populateInputFields(inputObj, def, types); err != nil {
				return nil, err
			}
		}
	}

	// Determine query/mutation/subscription roots.
	s := &schema.Schema{Types: types, DirectiveDefinitions: dirDefs}

	// First check for explicit schema definition.
	for _, def := range defs {
		if def.kind == "schema" {
			if def.schemaQuery != "" {
				t, ok := types[def.schemaQuery]
				if !ok {
					return nil, fmt.Errorf("unknown query root type %q", def.schemaQuery)
				}
				obj, ok := t.(*schema.Object)
				if !ok {
					return nil, fmt.Errorf("query root %q must be an object type", def.schemaQuery)
				}
				s.Query = obj
			}
			if def.schemaMutation != "" {
				t, ok := types[def.schemaMutation]
				if !ok {
					return nil, fmt.Errorf("unknown mutation root type %q", def.schemaMutation)
				}
				obj, ok := t.(*schema.Object)
				if !ok {
					return nil, fmt.Errorf("mutation root %q must be an object type", def.schemaMutation)
				}
				s.Mutation = obj
			}
			if def.schemaSubscription != "" {
				t, ok := types[def.schemaSubscription]
				if !ok {
					return nil, fmt.Errorf("unknown subscription root type %q", def.schemaSubscription)
				}
				obj, ok := t.(*schema.Object)
				if !ok {
					return nil, fmt.Errorf("subscription root %q must be an object type", def.schemaSubscription)
				}
				s.Subscription = obj
			}
		}
	}

	// Apply extend schema.
	for _, def := range defs {
		if def.kind == "schema" && def.isExtend {
			if def.schemaQuery != "" && s.Query == nil {
				t, ok := types[def.schemaQuery]
				if !ok {
					return nil, fmt.Errorf("unknown query root type %q", def.schemaQuery)
				}
				if obj, ok := t.(*schema.Object); ok {
					s.Query = obj
				}
			}
			if def.schemaMutation != "" && s.Mutation == nil {
				t, ok := types[def.schemaMutation]
				if !ok {
					return nil, fmt.Errorf("unknown mutation root type %q", def.schemaMutation)
				}
				if obj, ok := t.(*schema.Object); ok {
					s.Mutation = obj
				}
			}
			if def.schemaSubscription != "" && s.Subscription == nil {
				t, ok := types[def.schemaSubscription]
				if !ok {
					return nil, fmt.Errorf("unknown subscription root type %q", def.schemaSubscription)
				}
				if obj, ok := t.(*schema.Object); ok {
					s.Subscription = obj
				}
			}
		}
	}

	// Default roots by convention (Query, Mutation, Subscription).
	if s.Query == nil {
		if t, ok := types["Query"]; ok {
			if obj, ok := t.(*schema.Object); ok {
				s.Query = obj
			}
		}
	}
	if s.Mutation == nil {
		if t, ok := types["Mutation"]; ok {
			if obj, ok := t.(*schema.Object); ok {
				s.Mutation = obj
			}
		}
	}
	if s.Subscription == nil {
		if t, ok := types["Subscription"]; ok {
			if obj, ok := t.(*schema.Object); ok {
				s.Subscription = obj
			}
		}
	}

	// Validate all type references are resolved (only non-builtin).
	if err := validateSDLTypeRefs(defs, types); err != nil {
		return nil, err
	}

	return s, nil
}

func populateObjectFields(obj *schema.Object, def sdlDef, types map[string]schema.Type) error {
	for _, f := range def.fields {
		fieldType, err := f.typeRef.resolve(types)
		if err != nil {
			return err
		}
		args, err := buildFieldArgs(f.args, types)
		if err != nil {
			return err
		}
		schemaField := &schema.Field{
			Name:        f.name,
			Description: f.description,
			Type:        fieldType,
			Args:        args,
		}
		for _, d := range f.directives {
			if d.Name == "deprecated" {
				schemaField.IsDeprecated = true
				if r, ok := stringArg(d.Args, "reason"); ok {
					schemaField.DeprecationReason = &r
				}
			}
			schemaField.Directives = append(schemaField.Directives, schema.AppliedDirective{Name: d.Name, Args: d.Args})
		}
		obj.Fields[f.name] = schemaField
	}
	// Wire up implements.
	for _, ifaceName := range def.interfaces {
		t, ok := types[ifaceName]
		if !ok {
			return fmt.Errorf("unknown interface %q in implements list", ifaceName)
		}
		iface, ok := t.(*schema.Interface)
		if !ok {
			return fmt.Errorf("type %q in implements list is not an interface", ifaceName)
		}
		obj.Interfaces = append(obj.Interfaces, iface)
		iface.PossibleTypes = append(iface.PossibleTypes, obj)
	}
	return nil
}

func populateInterfaceFields(iface *schema.Interface, def sdlDef, types map[string]schema.Type) error {
	for _, f := range def.fields {
		fieldType, err := f.typeRef.resolve(types)
		if err != nil {
			return err
		}
		iface.Fields[f.name] = &schema.Field{Name: f.name, Description: f.description, Type: fieldType}
	}
	// Wire up interface implements interface.
	for _, ifaceName := range def.interfaces {
		t, ok := types[ifaceName]
		if !ok {
			return fmt.Errorf("unknown interface %q in interface implements list", ifaceName)
		}
		parent, ok := t.(*schema.Interface)
		if !ok {
			return fmt.Errorf("type %q in interface implements list is not an interface", ifaceName)
		}
		iface.Interfaces = append(iface.Interfaces, parent)
	}
	return nil
}

func populateInputFields(inputObj *schema.InputObject, def sdlDef, types map[string]schema.Type) error {
	for _, f := range def.fields {
		fieldType, err := f.typeRef.resolve(types)
		if err != nil {
			return err
		}
		schemaField := &schema.Field{Name: f.name, Description: f.description, Type: fieldType}
		// Default value stored in args[0] for input fields (see parseInputFieldDefs).
		if len(f.args) > 0 && f.args[0].hasDefault {
			schemaField.DefaultValue = f.args[0].defaultValue
		}
		for _, d := range f.directives {
			if d.Name == "deprecated" {
				schemaField.IsDeprecated = true
				if r, ok := stringArg(d.Args, "reason"); ok {
					schemaField.DeprecationReason = &r
				}
			}
		}
		inputObj.Fields[f.name] = schemaField
	}
	return nil
}

func buildFieldArgs(args []sdlArgDef, types map[string]schema.Type) ([]schema.InputValue, error) {
	result := make([]schema.InputValue, 0, len(args))
	for _, a := range args {
		argType, err := a.typeRef.resolve(types)
		if err != nil {
			return nil, err
		}
		iv := schema.InputValue{Name: a.name, Description: a.description, Type: argType}
		if a.hasDefault {
			iv.DefaultValue = a.defaultValue
		}
		result = append(result, iv)
	}
	return result, nil
}

func buildEnumValues(defs []sdlEnumValueDef) []schema.EnumValue {
	values := make([]schema.EnumValue, 0, len(defs))
	for _, d := range defs {
		ev := schema.EnumValue{Name: d.name, Description: d.description}
		for _, dir := range d.directives {
			if dir.Name == "deprecated" {
				ev.IsDeprecated = true
				if r, ok := stringArg(dir.Args, "reason"); ok {
					ev.DeprecationReason = &r
				}
			}
		}
		values = append(values, ev)
	}
	return values
}

func stringArg(args map[string]any, key string) (string, bool) {
	if args == nil {
		return "", false
	}
	v, ok := args[key]
	if !ok {
		return "", false
	}
	s, ok := v.(string)
	return s, ok
}

// validateSDLTypeRefs checks that every referenced type name exists.
func validateSDLTypeRefs(defs []sdlDef, types map[string]schema.Type) error {
	// Collect all names that appear as type references.
	var missing []string
	check := func(name string) {
		if _, ok := types[name]; !ok {
			missing = append(missing, name)
		}
	}
	var checkRef func(r sdlTypeRef)
	checkRef = func(r sdlTypeRef) {
		if r.isList {
			checkRef(*r.inner)
		} else {
			check(r.name)
		}
	}
	for _, def := range defs {
		for _, f := range def.fields {
			if def.kind == "directive" {
				checkRef(f.typeRef)
			} else {
				checkRef(f.typeRef)
			}
			for _, a := range f.args {
				checkRef(a.typeRef)
			}
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("SDL references unknown type(s): %s", strings.Join(missing, ", "))
	}
	return nil
}
