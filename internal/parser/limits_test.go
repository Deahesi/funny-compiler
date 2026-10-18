package parser_test

import (
	"strings"
	"testing"

	"compiler/internal/parser"
)

func TestRecursionLimit(t *testing.T) {
	// This is deliberately well above the approximate 256-level budget, not a
	// test of whether a particular implementation counts a level as 255 or 256.
	const depth = 1024
	wrap := func(statement string) string { return "f() returns r: int { " + statement + " }" }
	tests := []struct{ name, input string }{
		{"groups", wrap("r = " + strings.Repeat("(", depth) + "1" + strings.Repeat(")", depth) + ";")},
		{"unary minus", wrap("r = " + strings.Repeat("-", depth) + "1;")},
		{"not", wrap("if (" + strings.Repeat("not ", depth) + "true) {}")},
		{"calls", wrap("r = " + strings.Repeat("g(", depth) + "1" + strings.Repeat(")", depth) + ";")},
		{"indices", wrap("r = " + strings.Repeat("a[", depth) + "0" + strings.Repeat("]", depth) + ";")},
		{"quantifiers", "p() => " + strings.Repeat("forall(i: int | ", depth) + "true" + strings.Repeat(")", depth)},
		{"blocks", wrap(strings.Repeat("{", depth) + "r = 1;" + strings.Repeat("}", depth))},
		{"if bodies", wrap(strings.Repeat("if (true) ", depth) + "r = 1;")},
		{"implication", wrap("if (" + strings.Repeat("true -> ", depth) + "true) {}")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			program, diagnostics := parser.Parse(scan(t, tt.input))
			if program == nil {
				t.Fatal("recursion limit must return a program, not panic or return nil")
			}
			requireDiagnostics(t, tt.input, diagnostics)
		})
	}
}

func TestOrdinaryNestingIsAccepted(t *testing.T) {
	const depth = 16
	parseOK(t, "f() returns r: int { r = "+strings.Repeat("(", depth)+"1"+strings.Repeat(")", depth)+"; }")
	parseOK(t, "p() => "+strings.Repeat("forall(i: int | ", depth)+"true"+strings.Repeat(")", depth))
	parseOK(t, "f() returns r: int { "+strings.Repeat("{", depth)+"r = 1;"+strings.Repeat("}", depth)+" }")
}
