package lexer

type tokenValue struct {
	Name  string
	Value string
}

var keywords = []tokenValue{
	{Name: "RETURNS", Value: "returns"},
	{Name: "WHILE", Value: "while"},
	{Name: "IF", Value: "if"},
	{Name: "ELSE", Value: "else"},
	{Name: "ASSERT", Value: "assert"},
	{Name: "ASSUME", Value: "assume"},
	{Name: "INVARIANT", Value: "invariant"},
	{Name: "REQUIRES", Value: "requires"},
	{Name: "ENSURES", Value: "ensures"},
	{Name: "USES", Value: "uses"},
	{Name: "TYPE_INT", Value: "int"},
	{Name: "TRUE", Value: "true"},
	{Name: "FALSE", Value: "false"},
	{Name: "NOT", Value: "not"},
	{Name: "AND", Value: "and"},
	{Name: "OR", Value: "or"},
	{Name: "FORALL", Value: "forall"},
	{Name: "EXISTS", Value: "exists"},
}

var cases = []struct {
	Name      string
	Input     string
	Want      []tokenValue
	WantError bool
}{
	{Name: "empty", Input: "", Want: []tokenValue{{Name: "EOF"}}},
	{Name: "whitespace", Input: " \t\r\n \t", Want: []tokenValue{{Name: "EOF"}}},
	{Name: "zero", Input: "0", Want: []tokenValue{{Name: "INT", Value: "0"}, {Name: "EOF"}}},
	{Name: "double zero", Input: "00", WantError: true},
	{Name: "leading zero", Input: "01", WantError: true},
	{Name: "long leading zero", Input: "0123", WantError: true},
	{Name: "integer", Input: "12345", Want: []tokenValue{{Name: "INT", Value: "12345"}, {Name: "EOF"}}},
	{Name: "identifiers", Input: "aZ09 function length", Want: []tokenValue{
		{Name: "IDENT", Value: "aZ09"}, {Name: "IDENT", Value: "function"},
		{Name: "IDENT", Value: "length"}, {Name: "EOF"},
	}},
	{Name: "leading underscore", Input: "_abc", WantError: true},
	{Name: "identifier underscore", Input: "a_2", Want: []tokenValue{{Name: "IDENT", Value: "a"}}, WantError: true},
	{Name: "keyword prefix", Input: "function2 ifx length1", Want: []tokenValue{
		{Name: "IDENT", Value: "function2"}, {Name: "IDENT", Value: "ifx"},
		{Name: "IDENT", Value: "length1"}, {Name: "EOF"},
	}},
	{Name: "delimiters", Input: "()[]{},;:|", Want: []tokenValue{
		{Name: "LPAREN", Value: "("}, {Name: "RPAREN", Value: ")"},
		{Name: "LBRACKET", Value: "["}, {Name: "RBRACKET", Value: "]"},
		{Name: "LBRACE", Value: "{"}, {Name: "RBRACE", Value: "}"},
		{Name: "COMMA", Value: ","}, {Name: "SEMICOLON", Value: ";"},
		{Name: "COLON", Value: ":"}, {Name: "PIPE", Value: "|"}, {Name: "EOF"},
	}},
	{Name: "operators", Input: "+ - * / == != <= >= < > = => ->", Want: []tokenValue{
		{Name: "PLUS", Value: "+"}, {Name: "MINUS", Value: "-"},
		{Name: "STAR", Value: "*"}, {Name: "SLASH", Value: "/"},
		{Name: "EQ", Value: "=="}, {Name: "NE", Value: "!="},
		{Name: "LE", Value: "<="}, {Name: "GE", Value: ">="},
		{Name: "LT", Value: "<"}, {Name: "GT", Value: ">"},
		{Name: "ASSIGN", Value: "="}, {Name: "FAT_ARROW", Value: "=>"},
		{Name: "IMPLIES", Value: "->"}, {Name: "EOF"},
	}},
	{Name: "adjacent operators", Input: "=>=->>===", Want: []tokenValue{
		{Name: "FAT_ARROW", Value: "=>"}, {Name: "ASSIGN", Value: "="},
		{Name: "IMPLIES", Value: "->"}, {Name: "GE", Value: ">="},
		{Name: "EQ", Value: "=="}, {Name: "EOF"},
	}},
	{Name: "comment eof", Input: "x// text", Want: []tokenValue{{Name: "IDENT", Value: "x"}, {Name: "EOF"}}},
	{Name: "comment newline", Input: "// text\r\ny", Want: []tokenValue{{Name: "IDENT", Value: "y"}, {Name: "EOF"}}},
	{Name: "comment carriage return", Input: "// text\ry", Want: []tokenValue{{Name: "IDENT", Value: "y"}, {Name: "EOF"}}},
	{Name: "comment operators", Input: "/ // => -> 00 @\nx", Want: []tokenValue{
		{Name: "SLASH", Value: "/"}, {Name: "IDENT", Value: "x"}, {Name: "EOF"},
	}},
	{Name: "non ascii", Input: "é", WantError: true},
	{Name: "non ascii after token", Input: "xé", Want: []tokenValue{{Name: "IDENT", Value: "x"}}, WantError: true},
	{Name: "unknown ascii", Input: "@", WantError: true},
	{Name: "incomplete operator", Input: "!", WantError: true},
}
