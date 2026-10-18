package parser

import (
	"fmt"

	"compiler/internal/ast"
	"compiler/internal/source"
)

type expressionContext int

const (
	arithmetic expressionContext = iota
	condition
	predicate
)

func precedence(name string) int {
	switch name {
	case "IMPLIES":
		return 1
	case "OR":
		return 2
	case "AND":
		return 3
	case "EQ", "NE", "LT", "LE", "GT", "GE":
		return 5
	case "PLUS", "MINUS":
		return 6
	case "STAR", "SLASH":
		return 7
	default:
		return 0
	}
}

func (p *parser) expression(context expressionContext) (ast.Expression, *source.Diagnostic) {
	expression, err := p.parseBinary(1)
	if err != nil {
		return nil, err
	}
	if err := validateExpression(expression, context, 0); err != nil {
		return nil, err
	}
	return expression, nil
}

func (p *parser) parseBinary(minimum int) (ast.Expression, *source.Diagnostic) {
	if err := p.enter(); err != nil {
		return nil, err
	}
	defer p.leave()

	left, err := p.parsePrimary()
	if err != nil {
		return nil, err
	}
	for {
		token := p.current()
		level := precedence(token.Name)
		if level < minimum {
			return left, nil
		}
		if previous, ok := left.(*ast.Binary); ok && level == 5 && comparisonOperator(previous.Operator) {
			return nil, &source.Diagnostic{Span: token.Span, Message: "comparison operators cannot be chained"}
		}
		p.advance()
		next := level + 1
		if token.Name == "IMPLIES" {
			next = level
		}
		right, err := p.parseBinary(next)
		if err != nil {
			return nil, err
		}
		left = &ast.Binary{
			Node:     node("Binary", left.GetSpan().Start, right.GetSpan().End),
			Operator: token.Value, Left: left, Right: right,
		}
	}
}

func (p *parser) parsePrimary() (ast.Expression, *source.Diagnostic) {
	token := p.current()
	switch token.Name {
	case "INT":
		p.advance()
		return &ast.Integer{Node: node("Integer", token.Span.Start, token.Span.End), Value: token.Value}, nil
	case "TRUE", "FALSE":
		p.advance()
		return &ast.Boolean{Node: node("Boolean", token.Span.Start, token.Span.End), Value: token.Name == "TRUE"}, nil
	case "IDENT":
		p.advance()
		if p.match("LPAREN") {
			arguments := []ast.Expression{}
			if !p.at("RPAREN") {
				for {
					argument, err := p.expression(arithmetic)
					if err != nil {
						return nil, err
					}
					arguments = append(arguments, argument)
					if !p.match("COMMA") {
						break
					}
				}
			}
			end, err := p.expect("RPAREN", "')' after arguments")
			if err != nil {
				return nil, err
			}
			return &ast.Call{
				Node: node("Call", token.Span.Start, end.Span.End), Name: token.Value, Arguments: arguments,
			}, nil
		}
		identifier := &ast.Identifier{Node: node("Identifier", token.Span.Start, token.Span.End), Name: token.Value}
		return p.parseIndices(identifier)
	case "MINUS", "NOT":
		p.advance()
		level := 8
		if token.Name == "NOT" {
			level = 4
		}
		operand, err := p.parseBinary(level)
		if err != nil {
			return nil, err
		}
		return &ast.Unary{
			Node: node("Unary", token.Span.Start, operand.GetSpan().End), Operator: token.Value, Operand: operand,
		}, nil
	case "LPAREN":
		p.advance()
		expression, err := p.parseBinary(1)
		if err != nil {
			return nil, err
		}
		end, err := p.expect("RPAREN", "')' after expression")
		if err != nil {
			return nil, err
		}
		return &ast.Group{Node: node("Group", token.Span.Start, end.Span.End), Expression: expression}, nil
	case "FORALL", "EXISTS":
		return p.parseQuantifier()
	default:
		return nil, p.expected("an expression")
	}
}

func (p *parser) parseIndices(array ast.Expression) (ast.Expression, *source.Diagnostic) {
	for p.match("LBRACKET") {
		index, err := p.expression(arithmetic)
		if err != nil {
			return nil, err
		}
		end, err := p.expect("RBRACKET", "']' after array index")
		if err != nil {
			return nil, err
		}
		array = &ast.Index{
			Node: node("Index", array.GetSpan().Start, end.Span.End), Array: array, Index: index,
		}
	}
	if err := validateExpression(array, arithmetic, 0); err != nil {
		return nil, err
	}
	return array, nil
}

func (p *parser) parseQuantifier() (ast.Expression, *source.Diagnostic) {
	start := p.advance()
	if _, err := p.expect("LPAREN", "'(' after quantifier"); err != nil {
		return nil, err
	}
	variable, err := p.parseVariable(true)
	if err != nil {
		return nil, err
	}
	if _, err := p.expect("PIPE", "'|' after quantified variable"); err != nil {
		return nil, err
	}
	body, err := p.expression(predicate)
	if err != nil {
		return nil, err
	}
	end, err := p.expect("RPAREN", "')' after quantifier body")
	if err != nil {
		return nil, err
	}
	return &ast.Quantifier{
		Node:     node("Quantifier", start.Span.Start, end.Span.End),
		Operator: start.Value, Variable: variable, Body: body,
	}, nil
}

func comparisonOperator(operator string) bool {
	switch operator {
	case "==", "!=", "<", "<=", ">", ">=":
		return true
	default:
		return false
	}
}

// Parentheses and calls share prefixes across expr/condition/predicate. Parse
// once, then enforce their syntactic categories without symbol or type lookup.
func validateExpression(expression ast.Expression, context expressionContext, depth int) *source.Diagnostic {
	if depth >= maxDepth {
		return &source.Diagnostic{Span: expression.GetSpan(), Message: "maximum expression tree depth exceeded"}
	}
	check := func(child ast.Expression, childContext expressionContext) *source.Diagnostic {
		return validateExpression(child, childContext, depth+1)
	}
	invalid := func() *source.Diagnostic {
		expected := "an arithmetic expression"
		if context == condition {
			expected = "a condition (boolean constant or comparison)"
		} else if context == predicate {
			expected = "a predicate (boolean constant, comparison or formula reference)"
		}
		return &source.Diagnostic{Span: expression.GetSpan(), Message: fmt.Sprintf("expected %s", expected)}
	}
	switch expression := expression.(type) {
	case *ast.Group:
		return check(expression.Expression, context)
	case *ast.Identifier, *ast.Integer:
		if context != arithmetic {
			return invalid()
		}
	case *ast.Boolean:
		if context == arithmetic {
			return invalid()
		}
	case *ast.Call:
		if context == condition {
			return invalid()
		}
		for _, argument := range expression.Arguments {
			if err := check(argument, arithmetic); err != nil {
				return err
			}
		}
	case *ast.Index:
		if context != arithmetic {
			return invalid()
		}
		if err := check(expression.Array, arithmetic); err != nil {
			return err
		}
		return check(expression.Index, arithmetic)
	case *ast.Unary:
		if expression.Operator == "-" {
			if context != arithmetic {
				return invalid()
			}
			return check(expression.Operand, arithmetic)
		}
		if context == arithmetic {
			return invalid()
		}
		return check(expression.Operand, context)
	case *ast.Binary:
		childContext := context
		switch expression.Operator {
		case "+", "-", "*", "/":
			if context != arithmetic {
				return invalid()
			}
		case "==", "!=", "<", "<=", ">", ">=":
			if context == arithmetic {
				return invalid()
			}
			childContext = arithmetic
		case "and", "or":
			if context == arithmetic {
				return invalid()
			}
		case "->":
			if context != condition {
				return &source.Diagnostic{Span: expression.Span, Message: "implication '->' is allowed only in conditions"}
			}
		default:
			return invalid()
		}
		if err := check(expression.Left, childContext); err != nil {
			return err
		}
		return check(expression.Right, childContext)
	case *ast.Quantifier:
		if context != predicate {
			return invalid()
		}
		return check(expression.Body, predicate)
	default:
		return invalid()
	}
	return nil
}
