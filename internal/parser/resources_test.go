package parser_test

import (
	"encoding/json"
	"strings"
	"testing"

	"compiler/internal/ast"
	"compiler/internal/parser"
)

func TestArrayUpdateExpansionLimit(t *testing.T) {
	for _, depth := range []int{32, 33, 255} {
		input := "f(a: int[]) returns r: int { a" + strings.Repeat("[0]", depth) + " = 1; r = 2; }"
		program, diagnostics := parser.Parse(scan(t, input))
		if depth == 32 {
			if len(diagnostics) != 0 {
				t.Fatalf("32 indices rejected: %+v", diagnostics)
			}
		} else {
			requireDiagnostics(t, input, diagnostics)
			if !strings.Contains(diagnostics[0].Message, "array update depth") {
				t.Fatalf("unexpected diagnostic: %+v", diagnostics[0])
			}
		}
		function := nodeAs[*ast.Function](t, program.Declarations[0])
		body := functionStatements(t, function)
		last := nodeAs[*ast.Assignment](t, body[len(body)-1])
		requireShape(t, last.Value, "2")
		data, err := json.MarshalIndent(program, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if len(data) > 2<<20 {
			t.Fatalf("unbounded AST expansion at depth %d: %d bytes", depth, len(data))
		}
	}
}

func TestDepthRecoveryPreservesBlockBoundary(t *testing.T) {
	input := "f() returns r: int {" + strings.Repeat("{", 256) + strings.Repeat("}", 256) + "r=2;}\ngood() => true"
	program, diagnostics := parser.Parse(scan(t, input))
	requireDiagnostics(t, input, diagnostics)
	if len(diagnostics) != 1 {
		t.Fatalf("unexpected cascading errors: %+v", diagnostics)
	}
	if len(program.Declarations) != 2 {
		t.Fatalf("recovery lost declarations: %+v", program)
	}
	function := nodeAs[*ast.Function](t, program.Declarations[0])
	body := functionStatements(t, function)
	last := nodeAs[*ast.Assignment](t, body[len(body)-1])
	requireShape(t, last.Value, "2")
	end := strings.Index(input, "\ngood")
	requireSpan(t, function.Span, spanAt(input, 0, end))
	if formula := nodeAs[*ast.Formula](t, program.Declarations[1]); formula.Name != "good" {
		t.Errorf("unexpected next declaration: %+v", formula)
	}
}
