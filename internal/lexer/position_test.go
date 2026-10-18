package lexer

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"compiler/internal/automats"
	"compiler/internal/source"
)

func testSpan(startOffset, startLine, startColumn, endOffset, endLine, endColumn int) source.Span {
	return source.Span{
		Start: source.Position{Offset: startOffset, Line: startLine, Column: startColumn},
		End:   source.Position{Offset: endOffset, Line: endLine, Column: endColumn},
	}
}

func TestTokenSpans(t *testing.T) {
	full, minimal, err := Build()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		Name  string
		Input string
		Want  []Token
	}{
		{Name: "empty", Input: "", Want: []Token{
			{Name: "EOF", Span: testSpan(0, 1, 1, 0, 1, 1)},
		}},
		{Name: "whitespace", Input: " \t\r\n \t", Want: []Token{
			{Name: "EOF", Span: testSpan(6, 2, 3, 6, 2, 3)},
		}},
		{Name: "mixed newlines and comment", Input: " \tx\r\n// hi\rz\n\t=>0\r", Want: []Token{
			{Name: "IDENT", Value: "x", Span: testSpan(2, 1, 3, 3, 1, 4)},
			{Name: "IDENT", Value: "z", Span: testSpan(11, 3, 1, 12, 3, 2)},
			{Name: "FAT_ARROW", Value: "=>", Span: testSpan(14, 4, 2, 16, 4, 4)},
			{Name: "INT", Value: "0", Span: testSpan(16, 4, 4, 17, 4, 5)},
			{Name: "EOF", Span: testSpan(18, 5, 1, 18, 5, 1)},
		}},
		{Name: "comment at eof", Input: "x// text", Want: []Token{
			{Name: "IDENT", Value: "x", Span: testSpan(0, 1, 1, 1, 1, 2)},
			{Name: "EOF", Span: testSpan(8, 1, 9, 8, 1, 9)},
		}},
		{Name: "empty comment", Input: "//", Want: []Token{
			{Name: "EOF", Span: testSpan(2, 1, 3, 2, 1, 3)},
		}},
		{Name: "comments with all line endings", Input: "//a\r\n//b\r//c\nx//d", Want: []Token{
			{Name: "IDENT", Value: "x", Span: testSpan(13, 4, 1, 14, 4, 2)},
			{Name: "EOF", Span: testSpan(17, 4, 5, 17, 4, 5)},
		}},
		{Name: "consecutive line endings", Input: "\r\r\n\n\rx", Want: []Token{
			{Name: "IDENT", Value: "x", Span: testSpan(5, 5, 1, 6, 5, 2)},
			{Name: "EOF", Span: testSpan(6, 5, 2, 6, 5, 2)},
		}},
		{Name: "tab is one column", Input: "a\tb", Want: []Token{
			{Name: "IDENT", Value: "a", Span: testSpan(0, 1, 1, 1, 1, 2)},
			{Name: "IDENT", Value: "b", Span: testSpan(2, 1, 3, 3, 1, 4)},
			{Name: "EOF", Span: testSpan(3, 1, 4, 3, 1, 4)},
		}},
		{Name: "adjacent tokens", Input: "x:0=>1->2|", Want: []Token{
			{Name: "IDENT", Value: "x", Span: testSpan(0, 1, 1, 1, 1, 2)},
			{Name: "COLON", Value: ":", Span: testSpan(1, 1, 2, 2, 1, 3)},
			{Name: "INT", Value: "0", Span: testSpan(2, 1, 3, 3, 1, 4)},
			{Name: "FAT_ARROW", Value: "=>", Span: testSpan(3, 1, 4, 5, 1, 6)},
			{Name: "INT", Value: "1", Span: testSpan(5, 1, 6, 6, 1, 7)},
			{Name: "IMPLIES", Value: "->", Span: testSpan(6, 1, 7, 8, 1, 9)},
			{Name: "INT", Value: "2", Span: testSpan(8, 1, 9, 9, 1, 10)},
			{Name: "PIPE", Value: "|", Span: testSpan(9, 1, 10, 10, 1, 11)},
			{Name: "EOF", Span: testSpan(10, 1, 11, 10, 1, 11)},
		}},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			for _, dfa := range []*automats.DFA{full, minimal} {
				got, err := Scan(dfa, test.Input)
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, test.Want) {
					t.Errorf("got %#v; want %#v", got, test.Want)
				}
			}
		})
	}
}

func TestLexicalDiagnostics(t *testing.T) {
	full, minimal, err := Build()
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		Name    string
		Input   string
		Want    []Token
		Span    source.Span
		Message string
		Error   string
	}{
		{
			Name: "unknown ascii", Input: "@",
			Span:    testSpan(0, 1, 1, 1, 1, 2),
			Message: "undefined symbol '@'", Error: "1:1: undefined symbol '@'",
		},
		{
			Name: "incomplete operator", Input: "!",
			Span:    testSpan(0, 1, 1, 1, 1, 2),
			Message: "undefined symbol '!'", Error: "1:1: undefined symbol '!'",
		},
		{
			Name: "leading zero", Input: "0123+4",
			Span:    testSpan(0, 1, 1, 4, 1, 5),
			Message: `invalid int "0123"`, Error: `1:1: invalid int "0123"`,
		},
		{
			Name: "prefix before invalid int", Input: "x\r\n\t007 y",
			Want:    []Token{{Name: "IDENT", Value: "x", Span: testSpan(0, 1, 1, 1, 1, 2)}},
			Span:    testSpan(4, 2, 2, 7, 2, 5),
			Message: `invalid int "007"`, Error: `2:2: invalid int "007"`,
		},
		{
			Name: "error after comment and crlf", Input: "x// note\r\n\t@ y",
			Want:    []Token{{Name: "IDENT", Value: "x", Span: testSpan(0, 1, 1, 1, 1, 2)}},
			Span:    testSpan(11, 2, 2, 12, 2, 3),
			Message: "undefined symbol '@'", Error: "2:2: undefined symbol '@'",
		},
		{
			Name: "error after lone cr", Input: "x\r@",
			Want:    []Token{{Name: "IDENT", Value: "x", Span: testSpan(0, 1, 1, 1, 1, 2)}},
			Span:    testSpan(2, 2, 1, 3, 2, 2),
			Message: "undefined symbol '@'", Error: "2:1: undefined symbol '@'",
		},
		{
			Name: "identifier underscore", Input: "abc_def",
			Want:    []Token{{Name: "IDENT", Value: "abc", Span: testSpan(0, 1, 1, 3, 1, 4)}},
			Span:    testSpan(3, 1, 4, 4, 1, 5),
			Message: "undefined symbol '_'", Error: "1:4: undefined symbol '_'",
		},
		{
			Name: "two byte rune", Input: "xéz",
			Want:    []Token{{Name: "IDENT", Value: "x", Span: testSpan(0, 1, 1, 1, 1, 2)}},
			Span:    testSpan(1, 1, 2, 3, 1, 3),
			Message: "undefined symbol 'é'", Error: "1:2: undefined symbol 'é'",
		},
		{
			Name: "three byte rune", Input: "\t界",
			Span:    testSpan(1, 1, 2, 4, 1, 3),
			Message: "undefined symbol '界'", Error: "1:2: undefined symbol '界'",
		},
		{
			Name: "four byte rune", Input: "\r\n😀",
			Span:    testSpan(2, 2, 1, 6, 2, 2),
			Message: "undefined symbol '😀'", Error: "2:1: undefined symbol '😀'",
		},
		{
			Name: "non ascii comment", Input: "x// é",
			Want:    []Token{{Name: "IDENT", Value: "x", Span: testSpan(0, 1, 1, 1, 1, 2)}},
			Span:    testSpan(4, 1, 5, 6, 1, 6),
			Message: "undefined symbol 'é'", Error: "1:5: undefined symbol 'é'",
		},
		{
			Name: "invalid utf8", Input: "x\xff",
			Want:    []Token{{Name: "IDENT", Value: "x", Span: testSpan(0, 1, 1, 1, 1, 2)}},
			Span:    testSpan(1, 1, 2, 2, 1, 3),
			Message: "undefined symbol '�'", Error: "1:2: undefined symbol '�'",
		},
		{
			Name: "first error wins", Input: "01é",
			Span:    testSpan(0, 1, 1, 2, 1, 3),
			Message: `invalid int "01"`, Error: `1:1: invalid int "01"`,
		},
	}
	for _, test := range tests {
		t.Run(test.Name, func(t *testing.T) {
			for _, dfa := range []*automats.DFA{full, minimal} {
				got, err := Scan(dfa, test.Input)
				if !reflect.DeepEqual(got, test.Want) {
					t.Errorf("got prefix %#v; want %#v", got, test.Want)
				}
				var diagnostic *source.Diagnostic
				if !errors.As(err, &diagnostic) {
					t.Fatalf("expected *source.Diagnostic, got %T: %v", err, err)
				}
				if diagnostic.Span != test.Span || diagnostic.Message != test.Message {
					t.Errorf("got diagnostic %#v; want span %#v, message %q", diagnostic, test.Span, test.Message)
				}
				if err.Error() != test.Error {
					t.Errorf("got error %q; want %q", err.Error(), test.Error)
				}
			}
		})
	}
}

func TestSourceJSON(t *testing.T) {
	token := Token{Name: "INT", Value: "42", Span: testSpan(3, 2, 1, 5, 2, 3)}
	data, err := json.Marshal(token)
	if err != nil {
		t.Fatal(err)
	}
	const wantToken = `{"name":"INT","value":"42","span":{"start":{"offset":3,"line":2,"column":1},"end":{"offset":5,"line":2,"column":3}}}`
	if string(data) != wantToken {
		t.Errorf("got %s; want %s", data, wantToken)
	}
	var decoded Token
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}
	if decoded != token {
		t.Errorf("token JSON round trip: got %#v; want %#v", decoded, token)
	}
	diagnostic := &source.Diagnostic{Span: token.Span, Message: "bad token"}
	data, err = json.Marshal(diagnostic)
	if err != nil {
		t.Fatal(err)
	}
	const wantDiagnostic = `{"span":{"start":{"offset":3,"line":2,"column":1},"end":{"offset":5,"line":2,"column":3}},"message":"bad token"}`
	if string(data) != wantDiagnostic {
		t.Errorf("got %s; want %s", data, wantDiagnostic)
	}
	if diagnostic.Error() != "2:1: bad token" {
		t.Errorf("unexpected error: %s", diagnostic.Error())
	}
}
