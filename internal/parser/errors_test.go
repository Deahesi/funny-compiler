package parser_test

import (
	"strings"
	"testing"

	"compiler/internal/ast"
	"compiler/internal/parser"
)

func TestInvalidSyntax(t *testing.T) {
	tests := []struct{ name, input string }{
		{"function keyword is not a prefix", "function f() returns r: int {}"},
		{"missing parameter type", "f(x) returns r: int {}"},
		{"missing return type", "f() returns r {}"},
		{"missing returns", "f() {}"},
		{"trailing parameter comma", "f(x: int,) returns r: int {}"},
		{"trailing argument comma", "f() returns r: int { r = g(1,); }"},
		{"formula semicolon", "positive(x: int) => x > 0;"},
		{"missing formula body", "positive(x: int) =>"},
		{"unexpected top level statement", "x = 1;"},
		{"missing semicolon", "f() returns r: int { r = 1 }"},
		{"unclosed block", "f() returns r: int { r = 1;"},
		{"empty rhs", "f() returns r: int { r = ; }"},
		{"unmatched else", "f() returns r: int { else r = 1; }"},
		{"condition needs parentheses", "f() returns r: int { if true r = 1; }"},
		{"tuple rhs arithmetic", "f() returns r: int { x,y = a + b; }"},
		{"tuple rhs identifier", "f() returns r: int { x,y = a; }"},
		{"tuple rhs grouped call", "f() returns r: int { x,y = (pair(a)); }"},
		{"tuple rhs call plus", "f() returns r: int { x,y = pair(a) + 1; }"},
		{"indexed tuple target", "f() returns r: int { a[0],y = pair(a); }"},
		{"call as statement", "f() returns r: int { g(); }"},
		{"call as lvalue", "f() returns r: int { g() = 1; }"},
		{"empty index", "f() returns r: int { r = a[]; }"},
		{"index rooted at call", "f() returns r: int { r = g()[0]; }"},
		{"index rooted at group", "f() returns r: int { r = (a)[0]; }"},
		{"index rooted at integer", "f() returns r: int { r = 1[0]; }"},
		{"quantifier missing pipe", "f() returns r: int { assert forall(i: int i > 0); }"},
		{"quantifier missing type", "f() returns r: int { assert forall(i | true); }"},
		{"quantifier missing body", "f() returns r: int { assert exists(i: int | ); }"},
		{"chained comparison", "f() returns r: int { if (a < b < c) {} }"},
		{"mixed chained comparison", "f() returns r: int { assert a == b != c; }"},
		{"grouped comparison as arithmetic operand", "f() returns r: int { assert (a < b) < c; }"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, diagnostics := parser.Parse(scan(t, tt.input))
			requireDiagnostics(t, tt.input, diagnostics)
		})
	}
}

func TestExpressionContextRestrictions(t *testing.T) {
	// Apply the same invalid arithmetic to every arithmetic-only position. In
	// particular, grouping must not hide a boolean expression from validation.
	arithmeticContexts := []struct{ name, template string }{
		{"assignment", "f() returns r: int { r = EXPR; }"},
		{"call argument", "f() returns r: int { r = g(EXPR); }"},
		{"index read", "f() returns r: int { r = a[EXPR]; }"},
		{"index write", "f() returns r: int { a[EXPR] = 1; }"},
		{"formula argument", "f() returns r: int { assert p(EXPR); }"},
	}
	for _, context := range arithmeticContexts {
		t.Run(context.name, func(t *testing.T) {
			for _, expression := range []string{"true", "false", "a < b", "(a < b)", "not true", "true and false", "true -> false", "forall(i: int | true)"} {
				t.Run(expression, func(t *testing.T) {
					input := strings.Replace(context.template, "EXPR", expression, 1)
					_, diagnostics := parser.Parse(scan(t, input))
					requireDiagnostics(t, input, diagnostics)
				})
			}
		})
	}

	for _, statement := range []string{"if (EXPR) {}", "while (EXPR) {}"} {
		t.Run(statement, func(t *testing.T) {
			for _, expression := range []string{"x", "1", "x + 1", "p(x)", "(p(x))", "not p(x)", "true and p(x)", "forall(i: int | true)", "exists(i: int | true)"} {
				t.Run(expression, func(t *testing.T) {
					input := "f() returns r: int { " + strings.Replace(statement, "EXPR", expression, 1) + " }"
					_, diagnostics := parser.Parse(scan(t, input))
					requireDiagnostics(t, input, diagnostics)
				})
			}
		})
	}

	predicateContexts := []struct{ name, template string }{
		{"formula", "p() => EXPR"},
		{"requires", "f() requires EXPR returns r: int {}"},
		{"ensures", "f() returns r: int ensures EXPR {}"},
		{"invariant", "f() returns r: int { while (true) invariant EXPR {} }"},
		{"assert", "f() returns r: int { assert EXPR; }"},
		{"assume", "f() returns r: int { assume EXPR; }"},
	}
	for _, context := range predicateContexts {
		t.Run(context.name, func(t *testing.T) {
			for _, expression := range []string{"x", "1", "x + 1", "true -> false", "(true -> false)", "forall(i: int | true -> false)"} {
				t.Run(expression, func(t *testing.T) {
					input := strings.Replace(context.template, "EXPR", expression, 1)
					_, diagnostics := parser.Parse(scan(t, input))
					requireDiagnostics(t, input, diagnostics)
				})
			}
		})
	}
}

func TestRecoveryKeepsLaterStatements(t *testing.T) {
	input := string(fixture(t, "recovery.funny"))
	program, diagnostics := parser.Parse(scan(t, input))
	requireDiagnostics(t, input, diagnostics)
	if program == nil || len(program.Declarations) != 2 {
		t.Fatalf("recovery lost declarations: %+v", program)
	}
	function := nodeAs[*ast.Function](t, program.Declarations[0])
	if function.Name != "broken" {
		t.Fatalf("first declaration = %q, want broken", function.Name)
	}
	// Recovery may retain an error node/partial assignment. Only require that
	// the following valid statements survive, in their original order.
	var kept []ast.Statement
	for _, statement := range functionStatements(t, function) {
		switch n := statement.(type) {
		case *ast.Assignment:
			if value, ok := n.Value.(*ast.Integer); ok && value.Value == "2" {
				kept = append(kept, n)
			}
		case *ast.Assert:
			kept = append(kept, n)
		}
	}
	if len(kept) != 2 {
		t.Fatalf("recovery lost valid assignment or assert: %+v", kept)
	}
	assignment := nodeAs[*ast.Assignment](t, kept[0])
	if len(assignment.Targets) != 1 || assignment.Targets[0].Name != "r" {
		t.Fatalf("wrong recovered assignment: %+v", assignment)
	}
	requireShape(t, nodeAs[*ast.Assert](t, kept[1]).Predicate, "(r == 2)")
	last := nodeAs[*ast.Function](t, program.Declarations[1])
	if last.Name != "later" {
		t.Fatalf("last declaration = %q, want later", last.Name)
	}
	statements := functionStatements(t, last)
	if len(statements) != 1 {
		t.Fatalf("later body has %d statements, want 1", len(statements))
	}
	requireShape(t, nodeAs[*ast.Assignment](t, statements[0]).Value, "3")
}

func TestRecoveryKeepsLaterDeclarations(t *testing.T) {
	const input = "broken(x: ) => true\ngood() => true\nlater() returns r: int { r = 7; }"
	program, diagnostics := parser.Parse(scan(t, input))
	requireDiagnostics(t, input, diagnostics)
	if program == nil {
		t.Fatal("recovery returned nil program")
	}
	var names []string
	for _, declaration := range program.Declarations {
		switch n := declaration.(type) {
		case *ast.Formula:
			if n.Name == "good" {
				requireShape(t, n.Body, "true")
				names = append(names, n.Name)
			}
		case *ast.Function:
			if n.Name == "later" {
				names = append(names, n.Name)
			}
		}
	}
	if strings.Join(names, ",") != "good,later" {
		t.Errorf("recovery retained %v, want [good later]", names)
	}
}

func TestEveryTruncatedTokenPrefixIsSafe(t *testing.T) {
	const input = "p(x: int) => x > 0\nf(a: int[]) returns r: int { if (a[0] > 0) r = g(a[0]); else r = 0; }"
	tokens := scan(t, input)
	for end := 0; end <= len(tokens); end++ {
		program, _ := parser.Parse(tokens[:end])
		if program == nil {
			t.Errorf("prefix of %d tokens returned nil program", end)
		}
	}
}
