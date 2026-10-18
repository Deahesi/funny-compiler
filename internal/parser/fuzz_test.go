package parser_test

import (
	"testing"

	"compiler/internal/lexer"
	"compiler/internal/parser"
)

func FuzzParse(f *testing.F) {
	for _, seed := range []string{
		"",
		"f() returns r: int { r = 1; }",
		"p(x: int) => x > 0",
		"f() returns r: int { r = ; r = 2; }",
		"f() returns r: int { if (true) {} else {} }",
		"f() returns r: int { a[0][1] = -2; }",
		"p() => forall(i: int | true)",
		"(",
		"@",
	} {
		f.Add(seed)
	}
	dfa := testDFA(f)
	f.Fuzz(func(t *testing.T, input string) {
		// Bound lexer cost without excluding the deep nesting covered separately
		// by TestRecursionLimit. Let testing report any parser panic directly.
		if len(input) > 16*1024 {
			t.Skip()
		}
		tokens, err := lexer.Scan(dfa, input)
		if err != nil {
			t.Skip()
		}
		parser.Parse(tokens)
	})
}
