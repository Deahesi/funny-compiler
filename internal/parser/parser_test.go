package parser_test

import (
	"reflect"
	"strings"
	"testing"

	"compiler/internal/ast"
	"compiler/internal/lexer"
	"compiler/internal/parser"
)

func parseOK(t testing.TB, input string) *ast.Program {
	t.Helper()
	program, diagnostics := parser.Parse(scan(t, input))
	if len(diagnostics) != 0 {
		t.Fatalf("unexpected diagnostics: %+v\n%s", diagnostics, input)
	}
	if program == nil {
		t.Fatal("Parse returned nil program on success")
	}
	return program
}

func onlyFunction(t testing.TB, input string) *ast.Function {
	t.Helper()
	program := parseOK(t, input)
	if len(program.Declarations) != 1 {
		t.Fatalf("got %d declarations, want 1", len(program.Declarations))
	}
	return nodeAs[*ast.Function](t, program.Declarations[0])
}

func functionStatements(t testing.TB, function *ast.Function) []ast.Statement {
	t.Helper()
	return nodeAs[*ast.Block](t, function.Body).Statements
}

func TestValidPrograms(t *testing.T) {
	tests := []struct {
		name, input  string
		declarations int
	}{
		{"minimal function", "f() returns r: int { r = 1; }", 1},
		{"empty block", "f() returns r: int {}", 1},
		{"formula without semicolon", "positive(x: int) => x > 0", 1},
		{"adjacent formulas", "yes() => true\nno() => false", 2},
		{"formula after function", "f() returns r: int {}\np() => true", 2},
		{"requires only", "f() requires true returns r: int {}", 1},
		{"ensures only", "f() returns r: int ensures true {}", 1},
		{"identifier named function", "function(aZ09: int) returns r2: int { r2 = aZ09; }", 1},
		{"contracts and locals", "f(x: int, a: int[]) requires x >= 0 returns r: int ensures r > x uses t, b: int[] { t = x; b = a; r = t + 1; }", 1},
		{"multiple results", "pair(x: int) returns a: int, b: int { a = x; b = x + 1; }", 1},
		{"unbraced bodies", "f() returns r: int { if (true) r = 1; else r = 2; while (r < 3) r = r + 1; }", 1},
		{"nested index read and write", "f(a: int[]) returns r: int { r = a[0][1]; a[0][1] = r; }", 1},
		{"tuple call", "f() returns r: int uses x, y { x,y = pair(1); r = x; }", 1},
		{"explicit predicates", "f() requires true returns r: int ensures false { while (false) invariant true {} assert true; assume false; }", 1},
		{"predicate references and quantifiers", "f() requires positive(1) returns r: int ensures forall(i: int | exists(j: int | j >= i)) { assert not positive(-1) or true; assume exists(i: int | positive(i)); }", 1},
		{"condition implication", "f() returns r: int { if (true -> false -> true) {} while (not r < 0 and r != 2 or false) {} }", 1},
		{"no name or type checking", "f(x: int, x: int) returns r: int { missing = unknown(x, a[0]); r = 1 / 0; assert unknown(x); }", 1},
		{"arbitrary size integer", "f() returns r: int { r = 999999999999999999999999999999999999999999; }", 1},
		{"comments and CRLF", "// header\r\nf() returns r: int {\r\n\tr = 1; // value\r\n}\r\n", 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program := parseOK(t, tt.input)
			if len(program.Declarations) != tt.declarations {
				t.Errorf("got %d declarations, want %d", len(program.Declarations), tt.declarations)
			}
		})
	}
}

func TestDeclarationStructure(t *testing.T) {
	program := parseOK(t, string(fixture(t, "features.funny")))
	if len(program.Declarations) != 3 {
		t.Fatalf("got %d declarations, want 3", len(program.Declarations))
	}
	formula := nodeAs[*ast.Formula](t, program.Declarations[0])
	if formula.Name != "positive" || len(formula.Parameters) != 1 {
		t.Fatalf("unexpected formula: %+v", formula)
	}
	if p := formula.Parameters[0]; p.Name != "x" || p.Type != "int" {
		t.Errorf("formula parameter = %+v", p)
	}
	requireShape(t, formula.Body, "(x > 0)")

	function := nodeAs[*ast.Function](t, program.Declarations[1])
	if function.Name != "advance" {
		t.Errorf("name = %q, want advance", function.Name)
	}
	if len(function.Parameters) != 2 || len(function.Returns) != 1 || len(function.Locals) != 2 {
		t.Fatalf("unexpected variable lists: %+v", function)
	}
	for i, want := range []struct{ name, typ string }{{"x", "int"}, {"a", "int[]"}} {
		if got := function.Parameters[i]; got.Name != want.name || got.Type != want.typ {
			t.Errorf("parameter %d = %+v, want %+v", i, got, want)
		}
	}
	if got := function.Returns[0]; got.Name != "r" || got.Type != "int" {
		t.Errorf("return = %+v", got)
	}
	if got := function.Locals[0]; got.Name != "t" || got.Type != "" {
		t.Errorf("untyped local = %+v, want t with empty Type", got)
	}
	if got := function.Locals[1]; got.Name != "b" || got.Type != "int[]" {
		t.Errorf("typed local = %+v", got)
	}
	requireShape(t, function.Requires, "(x >= 0)")
	requireShape(t, function.Ensures, "(r > x)")
	body := functionStatements(t, function)
	if len(body) != 7 {
		t.Fatalf("got %d statements, want 7", len(body))
	}
	nodeAs[*ast.Assignment](t, body[0])
	nodeAs[*ast.Assignment](t, body[1])
	nodeAs[*ast.Assignment](t, body[2])
	loop := nodeAs[*ast.While](t, body[3])
	requireShape(t, loop.Condition, "(t < 2)")
	requireShape(t, loop.Invariant, "forall(i:int, (i >= 0))")
	nodeAs[*ast.If](t, body[4])
	requireShape(t, nodeAs[*ast.Assert](t, body[5]).Predicate, "positive(r)")
	requireShape(t, nodeAs[*ast.Assume](t, body[6]).Predicate, "exists(j:int, (j == r))")
	if last := nodeAs[*ast.Function](t, program.Declarations[2]); last.Name != "pair" || len(last.Returns) != 2 {
		t.Errorf("unexpected final declaration: %+v", last)
	}
}

func TestArithmeticStructure(t *testing.T) {
	tests := []struct{ input, want string }{
		{"a - b - c", "((a - b) - c)"},
		{"a + b - c + d", "(((a + b) - c) + d)"},
		{"a / b / c", "((a / b) / c)"},
		{"a * b / c * d", "(((a * b) / c) * d)"},
		{"a + b * c - d / e", "((a + (b * c)) - (d / e))"},
		{"-a * b + --c", "((-(a) * b) + -(-(c)))"},
		{"(a + b) * c", "(group((a + b)) * c)"},
		{"((a))", "group(group(a))"},
		{"outer(a + 1, inner(), b[i + 2][j])", "outer((a + 1), inner(), index(index(b, (i + 2)), j))"},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			function := onlyFunction(t, "f() returns r: int { r = "+tt.input+"; }")
			body := functionStatements(t, function)
			if len(body) != 1 {
				t.Fatalf("got %d statements, want 1", len(body))
			}
			requireShape(t, nodeAs[*ast.Assignment](t, body[0]).Value, tt.want)
		})
	}
}

func TestBooleanStructure(t *testing.T) {
	tests := []struct{ name, statement, want string }{
		{"comparison binds before not", "if (not x < y and y <= z or false) {}", "((not((x < y)) and (y <= z)) or false)"},
		{"implication is right associative", "if (true -> false -> true) {}", "(true -> (false -> true))"},
		{"and before or before implication", "if (true or false and true -> false) {}", "((true or (false and true)) -> false)"},
		{"arithmetic in comparison", "if (a + b * c >= -d) {}", "((a + (b * c)) >= -(d))"},
		{"boolean group retained", "if ((true or false) and true) {}", "(group((true or false)) and true)"},
		{"predicate reference", "assert not positive(x) and true or false;", "((not(positive(x)) and true) or false)"},
		{"quantifiers", "assert forall(i: int | exists(j: int | j > i and positive(j)));", "forall(i:int, exists(j:int, ((j > i) and positive(j))))"},
		{"not covers comparison in predicate", "assume not x == y;", "not((x == y))"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			function := onlyFunction(t, "f() returns r: int { "+tt.statement+" }")
			body := functionStatements(t, function)
			if len(body) != 1 {
				t.Fatalf("got %d statements, want 1", len(body))
			}
			var expression ast.Expression
			switch statement := body[0].(type) {
			case *ast.If:
				expression = statement.Condition
			case *ast.Assert:
				expression = statement.Predicate
			case *ast.Assume:
				expression = statement.Predicate
			default:
				t.Fatalf("unexpected statement %T", statement)
			}
			requireShape(t, expression, tt.want)
		})
	}
}

func TestAssignmentStructure(t *testing.T) {
	function := onlyFunction(t, "f() returns r: int { x,y = pair(a); a[i] = x; a[i][j] = y; }")
	body := functionStatements(t, function)
	if len(body) != 3 {
		t.Fatalf("got %d statements, want 3", len(body))
	}
	tests := []struct {
		targets []string
		value   string
	}{
		{[]string{"x", "y"}, "pair(a)"},
		{[]string{"a"}, "update(a, i, x)"},
		{[]string{"a"}, "update(a, i, update(index(a, i), j, y))"},
	}
	for i, tt := range tests {
		assignment := nodeAs[*ast.Assignment](t, body[i])
		names := make([]string, len(assignment.Targets))
		for j, target := range assignment.Targets {
			names[j] = target.Name
		}
		if !reflect.DeepEqual(names, tt.targets) {
			t.Errorf("statement %d targets = %v, want %v", i, names, tt.targets)
		}
		requireShape(t, assignment.Value, tt.value)
	}
}

func TestElseBindsToNearestIf(t *testing.T) {
	function := onlyFunction(t, "f() returns r: int { if (true) if (false) r = 1; else r = 2; }")
	body := functionStatements(t, function)
	if len(body) != 1 {
		t.Fatalf("got %d statements, want 1", len(body))
	}
	outer := nodeAs[*ast.If](t, body[0])
	inner := nodeAs[*ast.If](t, outer.Then)
	if outer.Else != nil || inner.Else == nil {
		t.Fatalf("else must belong to inner if: outer.Else=%T, inner.Else=%T", outer.Else, inner.Else)
	}
	requireShape(t, nodeAs[*ast.Assignment](t, inner.Then).Value, "1")
	requireShape(t, nodeAs[*ast.Assignment](t, inner.Else).Value, "2")
}

func TestImplicitPredicates(t *testing.T) {
	const input = "f() returns r: int { while (r < 1) r = r + 1; }"
	function := onlyFunction(t, input)
	body := functionStatements(t, function)
	if len(body) != 1 {
		t.Fatalf("got %d statements, want 1", len(body))
	}
	loop := nodeAs[*ast.While](t, body[0])
	tests := []struct {
		name       string
		expression ast.Expression
		value      bool
		insertion  int
	}{
		{"requires", function.Requires, true, strings.Index(input, "returns")},
		{"ensures", function.Ensures, false, strings.Index(input, "{")},
		{"invariant", loop.Invariant, true, strings.Index(input, "r =")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			boolean := nodeAs[*ast.Boolean](t, tt.expression)
			if boolean.Value != tt.value || !boolean.Synthetic {
				t.Errorf("default = %+v, want value=%v, Synthetic=true", boolean, tt.value)
			}
			requireSpan(t, boolean.Span, spanAt(input, tt.insertion, tt.insertion))
		})
	}
}

func TestDefaultsWithLocalsAndBlockBody(t *testing.T) {
	const input = "f() returns r: int uses t { while (true) { t = 1; } }"
	function := onlyFunction(t, input)
	ensures := nodeAs[*ast.Boolean](t, function.Ensures)
	if ensures.Value || !ensures.Synthetic {
		t.Errorf("missing ensures must be synthetic false: %+v", ensures)
	}
	insertion := strings.Index(input, "uses")
	requireSpan(t, ensures.Span, spanAt(input, insertion, insertion))
	body := functionStatements(t, function)
	if len(body) != 1 {
		t.Fatalf("got %d statements, want 1", len(body))
	}
	loop := nodeAs[*ast.While](t, body[0])
	invariant := nodeAs[*ast.Boolean](t, loop.Invariant)
	if !invariant.Value || !invariant.Synthetic {
		t.Errorf("missing invariant must be synthetic true: %+v", invariant)
	}
	insertion = strings.Index(input, "{ t")
	requireSpan(t, invariant.Span, spanAt(input, insertion, insertion))
}

func TestExplicitPredicatesAreNotSynthetic(t *testing.T) {
	const input = "f() requires true returns r: int ensures false { while (true) invariant true {} }"
	function := onlyFunction(t, input)
	body := functionStatements(t, function)
	if len(body) != 1 {
		t.Fatalf("got %d statements, want 1", len(body))
	}
	loop := nodeAs[*ast.While](t, body[0])
	for _, expression := range []ast.Expression{function.Requires, function.Ensures, loop.Condition, loop.Invariant} {
		boolean := nodeAs[*ast.Boolean](t, expression)
		if boolean.Synthetic || boolean.Span.Start.Offset == boolean.Span.End.Offset {
			t.Errorf("explicit boolean must have a source span and not be synthetic: %+v", boolean)
		}
	}
}

func TestVariableSpans(t *testing.T) {
	const input = "f(a: int[]) returns r: int uses t, b: int[] {}"
	function := onlyFunction(t, input)
	if len(function.Parameters) != 1 || len(function.Returns) != 1 || len(function.Locals) != 2 {
		t.Fatalf("unexpected variable lists: %+v", function)
	}
	for _, tt := range []struct {
		variable ast.Variable
		text     string
	}{
		{function.Parameters[0], "a: int[]"},
		{function.Returns[0], "r: int"},
		{function.Locals[0], "t,"},
		{function.Locals[1], "b: int[]"},
	} {
		start := strings.Index(input, tt.text)
		end := start + len(strings.TrimSuffix(tt.text, ","))
		requireSpan(t, tt.variable.Span, spanAt(input, start, end))
	}
}

func TestSpans(t *testing.T) {
	const input = "\r\nf(x: int) returns r: int {\r\n\tr = (x + 12);\r\n}"
	function := onlyFunction(t, input)
	start := strings.Index(input, "f(")
	requireSpan(t, function.Span, spanAt(input, start, len(input)))
	block := nodeAs[*ast.Block](t, function.Body)
	requireSpan(t, block.Span, spanAt(input, strings.Index(input, "{"), len(input)))
	if len(block.Statements) != 1 {
		t.Fatalf("got %d statements, want 1", len(block.Statements))
	}
	assignment := nodeAs[*ast.Assignment](t, block.Statements[0])
	assignmentStart := strings.Index(input, "r =")
	requireSpan(t, assignment.Span, spanAt(input, assignmentStart, strings.Index(input, ";")+1))
	if len(assignment.Targets) != 1 {
		t.Fatalf("got %d targets, want 1", len(assignment.Targets))
	}
	requireSpan(t, assignment.Targets[0].Span, spanAt(input, assignmentStart, assignmentStart+1))
	group := nodeAs[*ast.Group](t, assignment.Value)
	groupStart := strings.Index(input, "(x +")
	requireSpan(t, group.Span, spanAt(input, groupStart, strings.Index(input, ";")))
	binary := nodeAs[*ast.Binary](t, group.Expression)
	requireSpan(t, binary.Span, spanAt(input, groupStart+1, strings.Index(input, ";")-1))
	integer := nodeAs[*ast.Integer](t, binary.Right)
	i := strings.Index(input, "12")
	requireSpan(t, integer.Span, spanAt(input, i, i+2))
	if integer.Value != "12" {
		t.Errorf("integer.Value = %q", integer.Value)
	}
}

func TestMissingEOFIsNormalized(t *testing.T) {
	const input = "f() returns r: int { r = 1; }"
	tokens := scan(t, input)
	want, wantDiagnostics := parser.Parse(tokens)
	got, diagnostics := parser.Parse(tokens[:len(tokens)-1])
	if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(diagnostics, wantDiagnostics) {
		t.Fatalf("missing EOF changes result:\n got: %+v, %+v\nwant: %+v, %+v", got, diagnostics, want, wantDiagnostics)
	}
	if len(diagnostics) != 0 || got == nil {
		t.Fatalf("valid tokens without EOF rejected: %+v", diagnostics)
	}
}

func TestEmptyInput(t *testing.T) {
	for _, tt := range []struct {
		name   string
		input  string
		tokens []lexer.Token
	}{
		{"nil tokens", "", nil},
		{"empty tokens", "", []lexer.Token{}},
		{"EOF only", "", scan(t, "")},
		{"whitespace and comment", " \n// empty", scan(t, " \n// empty")},
	} {
		t.Run(tt.name, func(t *testing.T) {
			program, diagnostics := parser.Parse(tt.tokens)
			if program == nil || len(program.Declarations) != 0 {
				t.Fatalf("empty input must return an empty non-nil program: %+v", program)
			}
			requireDiagnostics(t, tt.input, diagnostics)
		})
	}
}
