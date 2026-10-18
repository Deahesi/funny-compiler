package parser

import (
	"compiler/internal/ast"
	"compiler/internal/source"
)

func (p *parser) parseStatement() (ast.Statement, *source.Diagnostic) {
	if err := p.enter(); err != nil {
		return nil, err
	}
	defer p.leave()

	switch p.current().Name {
	case "LBRACE":
		return p.parseBlock()
	case "IDENT":
		return p.parseAssignment()
	case "IF":
		return p.parseIf()
	case "WHILE":
		return p.parseWhile()
	case "ASSERT", "ASSUME":
		return p.parseAssertion()
	default:
		return nil, p.expected("an assignment, 'if', 'while', '{', 'assert' or 'assume'")
	}
}

func (p *parser) parseBlock() (*ast.Block, *source.Diagnostic) {
	start := p.advance()
	block := &ast.Block{Statements: []ast.Statement{}}
	for !p.at("RBRACE") && !p.at("EOF") {
		if p.declarationAhead() {
			p.report(p.expected("'}' before the next declaration"))
			block.Node = node("Block", start.Span.Start, p.current().Span.Start)
			return block, nil
		}
		position := p.pos
		statement, err := p.parseStatement()
		if err != nil {
			p.report(err)
			if p.pos == position {
				if p.at("LBRACE") {
					p.skipBlock()
					continue
				}
				p.advance()
			}
			p.synchronizeStatement()
			continue
		}
		block.Statements = append(block.Statements, statement)
	}
	end, err := p.expect("RBRACE", "'}'")
	if err != nil {
		p.report(err)
		block.Node = node("Block", start.Span.Start, p.current().Span.Start)
		return block, nil
	}
	block.Node = node("Block", start.Span.Start, end.Span.End)
	return block, nil
}

// A depth-limited block must be skipped as a unit so its closing brace does
// not accidentally close its parent during recovery.
func (p *parser) skipBlock() {
	depth := 0
	for !p.at("EOF") {
		switch p.advance().Name {
		case "LBRACE":
			depth++
		case "RBRACE":
			depth--
			if depth == 0 {
				return
			}
		}
	}
}

func (p *parser) synchronizeStatement() {
	for !p.at("EOF") && !p.at("RBRACE") {
		if p.match("SEMICOLON") {
			return
		}
		switch p.current().Name {
		case "IDENT", "IF", "WHILE", "LBRACE", "ASSERT", "ASSUME":
			return
		}
		p.advance()
	}
}

func (p *parser) parseAssignment() (*ast.Assignment, *source.Diagnostic) {
	name := p.advance()
	identifier := &ast.Identifier{Node: node("Identifier", name.Span.Start, name.Span.End), Name: name.Value}
	targets := []*ast.Identifier{identifier}
	var indexed ast.Expression = identifier
	var err *source.Diagnostic
	if p.at("LBRACKET") {
		indexed, err = p.parseIndices(identifier)
		if err != nil {
			return nil, err
		}
	} else {
		for p.match("COMMA") {
			name, err := p.expect("IDENT", "an assignment target")
			if err != nil {
				return nil, err
			}
			targets = append(targets, &ast.Identifier{
				Node: node("Identifier", name.Span.Start, name.Span.End), Name: name.Value,
			})
		}
	}
	if _, err := p.expect("ASSIGN", "'='"); err != nil {
		return nil, err
	}
	value, err := p.expression(arithmetic)
	if err != nil {
		return nil, err
	}
	if len(targets) > 1 {
		if _, ok := value.(*ast.Call); !ok {
			return nil, &source.Diagnostic{Span: value.GetSpan(), Message: "multiple assignment requires a function call"}
		}
	}
	end, err := p.expect("SEMICOLON", "';' after assignment")
	if err != nil {
		return nil, err
	}
	if _, ok := indexed.(*ast.Index); ok {
		value, err = arrayUpdate(indexed, value, source.Span{Start: name.Span.Start, End: end.Span.End})
		if err != nil {
			return nil, err
		}
	}
	return &ast.Assignment{
		Node: node("Assignment", name.Span.Start, end.Span.End), Targets: targets, Value: value,
	}, nil
}

// Nested updates repeat indexed prefixes when serialized as a tree. Bound
// this expansion separately from the parser's recursion limit.
const maxArrayUpdateDepth = 32

func arrayUpdate(target, value ast.Expression, span source.Span) (ast.Expression, *source.Diagnostic) {
	for depth := 0; ; depth++ {
		index, ok := target.(*ast.Index)
		if !ok {
			return value, nil
		}
		if depth >= maxArrayUpdateDepth {
			return nil, &source.Diagnostic{Span: span, Message: "maximum array update depth (32) exceeded"}
		}
		value = &ast.ArrayUpdate{
			Node:  ast.Node{Kind: "ArrayUpdate", Span: span, Synthetic: true},
			Array: index.Array, Index: index.Index, Value: value,
		}
		target = index.Array
	}
}

func (p *parser) parseCondition() (ast.Expression, *source.Diagnostic) {
	if _, err := p.expect("LPAREN", "'(' before condition"); err != nil {
		return nil, err
	}
	expression, err := p.expression(condition)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect("RPAREN", "')' after condition"); err != nil {
		return nil, err
	}
	return expression, nil
}

func (p *parser) parseIf() (*ast.If, *source.Diagnostic) {
	start := p.advance()
	condition, err := p.parseCondition()
	if err != nil {
		return nil, err
	}
	then, err := p.parseStatement()
	if err != nil {
		return nil, err
	}
	statement := &ast.If{Condition: condition, Then: then}
	end := then.GetSpan().End
	if p.match("ELSE") {
		statement.Else, err = p.parseStatement()
		if err != nil {
			return nil, err
		}
		end = statement.Else.GetSpan().End
	}
	statement.Node = node("If", start.Span.Start, end)
	return statement, nil
}

func (p *parser) parseWhile() (*ast.While, *source.Diagnostic) {
	start := p.advance()
	condition, err := p.parseCondition()
	if err != nil {
		return nil, err
	}
	invariant := p.implicitBoolean(true)
	if p.match("INVARIANT") {
		invariant, err = p.expression(predicate)
		if err != nil {
			return nil, err
		}
	}
	body, err := p.parseStatement()
	if err != nil {
		return nil, err
	}
	return &ast.While{
		Node:      node("While", start.Span.Start, body.GetSpan().End),
		Condition: condition, Invariant: invariant, Body: body,
	}, nil
}

func (p *parser) parseAssertion() (ast.Statement, *source.Diagnostic) {
	start := p.advance()
	expression, err := p.expression(predicate)
	if err != nil {
		return nil, err
	}
	end, err := p.expect("SEMICOLON", "';' after predicate")
	if err != nil {
		return nil, err
	}
	if start.Name == "ASSERT" {
		return &ast.Assert{Node: node("Assert", start.Span.Start, end.Span.End), Predicate: expression}, nil
	}
	return &ast.Assume{Node: node("Assume", start.Span.Start, end.Span.End), Predicate: expression}, nil
}
