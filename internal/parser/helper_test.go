package parser_test

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"testing"

	"compiler/internal/ast"
	"compiler/internal/automats"
	"compiler/internal/lexer"
	"compiler/internal/source"
)

var (
	lexerOnce sync.Once
	sharedDFA *automats.DFA
	lexerErr  error
)

func testDFA(t testing.TB) *automats.DFA {
	t.Helper()
	lexerOnce.Do(func() {
		_, sharedDFA, lexerErr = lexer.Build()
	})
	if lexerErr != nil {
		t.Fatalf("build lexer: %v", lexerErr)
	}
	return sharedDFA
}

func scan(t testing.TB, input string) []lexer.Token {
	t.Helper()
	tokens, err := lexer.Scan(testDFA(t), input)
	if err != nil {
		t.Fatalf("fixture must be lexically valid: %v\n%s", err, input)
	}
	if len(tokens) == 0 || tokens[len(tokens)-1].Name != "EOF" {
		t.Fatal("successful lexer.Scan must end with EOF")
	}
	return tokens
}

func fixture(t testing.TB, name string) []byte {
	t.Helper()
	data, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func nodeAs[T any](t testing.TB, value any) T {
	t.Helper()
	result, ok := value.(T)
	if !ok {
		var want T
		t.Fatalf("got %T, want %T", value, want)
	}
	return result
}

// expressionShape deliberately preserves Group and the operand order, but not
// source metadata, so precedence tests remain independent of span tests.
func expressionShape(t testing.TB, expression ast.Expression) string {
	t.Helper()
	switch n := expression.(type) {
	case *ast.Identifier:
		return n.Name
	case *ast.Integer:
		return n.Value
	case *ast.Boolean:
		return fmt.Sprint(n.Value)
	case *ast.Unary:
		return n.Operator + "(" + expressionShape(t, n.Operand) + ")"
	case *ast.Binary:
		return "(" + expressionShape(t, n.Left) + " " + n.Operator + " " + expressionShape(t, n.Right) + ")"
	case *ast.Group:
		return "group(" + expressionShape(t, n.Expression) + ")"
	case *ast.Call:
		args := make([]string, len(n.Arguments))
		for i, arg := range n.Arguments {
			args[i] = expressionShape(t, arg)
		}
		return n.Name + "(" + strings.Join(args, ", ") + ")"
	case *ast.Index:
		return "index(" + expressionShape(t, n.Array) + ", " + expressionShape(t, n.Index) + ")"
	case *ast.ArrayUpdate:
		return "update(" + expressionShape(t, n.Array) + ", " + expressionShape(t, n.Index) + ", " + expressionShape(t, n.Value) + ")"
	case *ast.Quantifier:
		return n.Operator + "(" + n.Variable.Name + ":" + n.Variable.Type + ", " + expressionShape(t, n.Body) + ")"
	default:
		t.Fatalf("unexpected expression %T", expression)
		return ""
	}
}

func requireShape(t testing.TB, expression ast.Expression, want string) {
	t.Helper()
	if got := expressionShape(t, expression); got != want {
		t.Errorf("expression tree:\n got: %s\nwant: %s", got, want)
	}
}

// Fixtures use ASCII; these position rules also match the lexer's CRLF and tab
// handling. Keeping this independent of lexer tokens catches displaced AST spans.
func positionAt(input string, offset int) source.Position {
	position := source.Position{Offset: offset, Line: 1, Column: 1}
	for i := 0; i < offset; i++ {
		switch input[i] {
		case '\r':
			position.Line++
			position.Column = 1
		case '\n':
			if i == 0 || input[i-1] != '\r' {
				position.Line++
			}
			position.Column = 1
		default:
			position.Column++
		}
	}
	return position
}

func spanAt(input string, start, end int) source.Span {
	return source.Span{Start: positionAt(input, start), End: positionAt(input, end)}
}

func requireSpan(t testing.TB, got, want source.Span) {
	t.Helper()
	if got != want {
		t.Errorf("span = %+v, want %+v", got, want)
	}
}

func requireDiagnostics(t testing.TB, input string, diagnostics []source.Diagnostic) {
	t.Helper()
	if len(diagnostics) == 0 {
		t.Fatal("expected syntax diagnostics")
	}
	for _, diagnostic := range diagnostics {
		if strings.TrimSpace(diagnostic.Message) == "" {
			t.Error("diagnostic has an empty message")
		}
		start, end := diagnostic.Span.Start.Offset, diagnostic.Span.End.Offset
		if start < 0 || end < start || end > len(input) {
			t.Errorf("diagnostic span outside source: %+v (length %d)", diagnostic, len(input))
			continue
		}
		requireSpan(t, diagnostic.Span, spanAt(input, start, end))
	}
}
