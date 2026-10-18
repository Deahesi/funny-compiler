package parser

import (
	"fmt"

	"compiler/internal/ast"
	"compiler/internal/lexer"
	"compiler/internal/source"
)

const maxDepth = 256

type parser struct {
	tokens      []lexer.Token
	pos         int
	depth       int
	diagnostics []source.Diagnostic
}

func Parse(tokens []lexer.Token) (*ast.Program, []source.Diagnostic) {
	end := source.Position{Line: 1, Column: 1}
	if len(tokens) > 0 {
		end = tokens[len(tokens)-1].Span.End
	}
	if len(tokens) == 0 || tokens[len(tokens)-1].Name != "EOF" {
		copyOfTokens := make([]lexer.Token, len(tokens), len(tokens)+1)
		copy(copyOfTokens, tokens)
		tokens = append(copyOfTokens, lexer.Token{Name: "EOF", Span: source.Span{Start: end, End: end}})
	}
	p := &parser{tokens: tokens}
	program := &ast.Program{
		Node:         ast.Node{Kind: "Program", Span: source.Span{Start: p.current().Span.Start, End: end}},
		Declarations: []ast.Declaration{},
	}
	if p.at("EOF") {
		p.report(p.expected("a function or formula declaration"))
	}
	for !p.at("EOF") {
		start := p.pos
		declaration, err := p.parseDeclaration()
		if err != nil {
			p.report(err)
			if p.pos == start {
				p.advance()
			}
			p.synchronizeDeclaration()
			continue
		}
		program.Declarations = append(program.Declarations, declaration)
	}
	return program, p.diagnostics
}

func (p *parser) current() lexer.Token { return p.tokens[p.pos] }
func (p *parser) at(name string) bool  { return p.current().Name == name }

func (p *parser) advance() lexer.Token {
	token := p.current()
	if !p.at("EOF") {
		p.pos++
	}
	return token
}

func (p *parser) match(name string) bool {
	if !p.at(name) {
		return false
	}
	p.advance()
	return true
}

func (p *parser) expect(name, description string) (lexer.Token, *source.Diagnostic) {
	if !p.at(name) {
		return lexer.Token{}, p.expected(description)
	}
	return p.advance(), nil
}

func (p *parser) expected(description string) *source.Diagnostic {
	found := fmt.Sprintf("%q", p.current().Value)
	if p.at("EOF") {
		found = "end of input"
	}
	return &source.Diagnostic{
		Span:    p.current().Span,
		Message: fmt.Sprintf("expected %s, got %s", description, found),
	}
}

func (p *parser) report(diagnostic *source.Diagnostic) {
	p.diagnostics = append(p.diagnostics, *diagnostic)
}

func (p *parser) enter() *source.Diagnostic {
	if p.depth >= maxDepth {
		return &source.Diagnostic{Span: p.current().Span, Message: "maximum syntax nesting depth exceeded"}
	}
	p.depth++
	return nil
}

func (p *parser) leave() { p.depth-- }

func node(kind string, start, end source.Position) ast.Node {
	return ast.Node{Kind: kind, Span: source.Span{Start: start, End: end}}
}

func (p *parser) implicitBoolean(value bool) ast.Expression {
	position := p.current().Span.Start
	base := node("Boolean", position, position)
	base.Synthetic = true
	return &ast.Boolean{Node: base, Value: value}
}

func (p *parser) parseDeclaration() (ast.Declaration, *source.Diagnostic) {
	name, err := p.expect("IDENT", "a declaration name")
	if err != nil {
		return nil, err
	}
	if _, err := p.expect("LPAREN", "'('"); err != nil {
		return nil, err
	}
	parameters := []ast.Variable{}
	if !p.at("RPAREN") {
		parameters, err = p.parseVariables(true)
		if err != nil {
			return nil, err
		}
	}
	if _, err := p.expect("RPAREN", "')'"); err != nil {
		return nil, err
	}
	if p.match("FAT_ARROW") {
		body, err := p.expression(predicate)
		if err != nil {
			return nil, err
		}
		return &ast.Formula{
			Node: node("Formula", name.Span.Start, body.GetSpan().End),
			Name: name.Value, Parameters: parameters, Body: body,
		}, nil
	}
	requires := p.implicitBoolean(true)
	if p.match("REQUIRES") {
		requires, err = p.expression(predicate)
		if err != nil {
			return nil, err
		}
	}
	if _, err := p.expect("RETURNS", "'returns'"); err != nil {
		return nil, err
	}
	returns, err := p.parseVariables(true)
	if err != nil {
		return nil, err
	}
	ensures := p.implicitBoolean(false)
	if p.match("ENSURES") {
		ensures, err = p.expression(predicate)
		if err != nil {
			return nil, err
		}
	}
	locals := []ast.Variable{}
	if p.match("USES") {
		locals, err = p.parseVariables(false)
		if err != nil {
			return nil, err
		}
	}
	body, err := p.parseStatement()
	if err != nil {
		return nil, err
	}
	return &ast.Function{
		Node: node("Function", name.Span.Start, body.GetSpan().End),
		Name: name.Value, Parameters: parameters, Returns: returns,
		Requires: requires, Ensures: ensures, Locals: locals, Body: body,
	}, nil
}

func (p *parser) parseVariables(requireType bool) ([]ast.Variable, *source.Diagnostic) {
	variables := []ast.Variable{}
	for {
		variable, err := p.parseVariable(requireType)
		if err != nil {
			return nil, err
		}
		variables = append(variables, variable)
		if !p.match("COMMA") {
			return variables, nil
		}
	}
}

func (p *parser) parseVariable(requireType bool) (ast.Variable, *source.Diagnostic) {
	name, err := p.expect("IDENT", "a variable name")
	if err != nil {
		return ast.Variable{}, err
	}
	variable := ast.Variable{Name: name.Value, Span: name.Span}
	if !requireType && !p.at("COLON") {
		return variable, nil
	}
	if _, err := p.expect("COLON", "':' before a variable type"); err != nil {
		return ast.Variable{}, err
	}
	typ, err := p.expect("TYPE_INT", "'int'")
	if err != nil {
		return ast.Variable{}, err
	}
	variable.Type = "int"
	variable.Span.End = typ.Span.End
	if p.match("LBRACKET") {
		end, err := p.expect("RBRACKET", "']' in array type")
		if err != nil {
			return ast.Variable{}, err
		}
		variable.Type = "int[]"
		variable.Span.End = end.Span.End
	}
	return variable, nil
}

func (p *parser) synchronizeDeclaration() {
	for !p.at("EOF") && !p.declarationAhead() {
		p.advance()
	}
}

// Typed parameters and the suffix distinguish declaration headers from calls.
func (p *parser) declarationAhead() bool {
	i := p.pos
	is := func(name string) bool { return i < len(p.tokens) && p.tokens[i].Name == name }
	if !is("IDENT") {
		return false
	}
	i++
	if !is("LPAREN") {
		return false
	}
	i++
	if !is("RPAREN") {
		for {
			if !is("IDENT") {
				return false
			}
			i++
			if !is("COLON") {
				return false
			}
			i++
			if !is("TYPE_INT") {
				return false
			}
			i++
			if is("LBRACKET") {
				i++
				if !is("RBRACKET") {
					return false
				}
				i++
			}
			if !is("COMMA") {
				break
			}
			i++
		}
	}
	if !is("RPAREN") {
		return false
	}
	i++
	return is("FAT_ARROW") || is("REQUIRES") || is("RETURNS")
}
