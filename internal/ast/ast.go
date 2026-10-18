package ast

import "compiler/internal/source"

type Node struct {
	Kind      string      `json:"kind"`
	Span      source.Span `json:"span"`
	Synthetic bool        `json:"synthetic,omitempty"`
}

func (node Node) GetSpan() source.Span { return node.Span }

type Declaration interface {
	GetSpan() source.Span
	declaration()
}

type Statement interface {
	GetSpan() source.Span
	statement()
}

type Expression interface {
	GetSpan() source.Span
	expression()
}

type Program struct {
	Node
	Declarations []Declaration `json:"declarations"`
}

type Variable struct {
	Name string      `json:"name"`
	Type string      `json:"type,omitempty"`
	Span source.Span `json:"span"`
}

type Function struct {
	Node
	Name       string     `json:"name"`
	Parameters []Variable `json:"parameters"`
	Returns    []Variable `json:"returns"`
	Requires   Expression `json:"requires"`
	Ensures    Expression `json:"ensures"`
	Locals     []Variable `json:"locals"`
	Body       Statement  `json:"body"`
}

type Formula struct {
	Node
	Name       string     `json:"name"`
	Parameters []Variable `json:"parameters"`
	Body       Expression `json:"body"`
}

type Block struct {
	Node
	Statements []Statement `json:"statements"`
}

type Assignment struct {
	Node
	Targets []*Identifier `json:"targets"`
	Value   Expression    `json:"value"`
}

type If struct {
	Node
	Condition Expression `json:"condition"`
	Then      Statement  `json:"then"`
	Else      Statement  `json:"else,omitempty"`
}

type While struct {
	Node
	Condition Expression `json:"condition"`
	Invariant Expression `json:"invariant"`
	Body      Statement  `json:"body"`
}

type Assert struct {
	Node
	Predicate Expression `json:"predicate"`
}

type Assume struct {
	Node
	Predicate Expression `json:"predicate"`
}

type Identifier struct {
	Node
	Name string `json:"name"`
}

type Integer struct {
	Node
	Value string `json:"value"`
}

type Boolean struct {
	Node
	Value bool `json:"value"`
}

type Unary struct {
	Node
	Operator string     `json:"operator"`
	Operand  Expression `json:"operand"`
}

type Binary struct {
	Node
	Operator string     `json:"operator"`
	Left     Expression `json:"left"`
	Right    Expression `json:"right"`
}

type Group struct {
	Node
	Expression Expression `json:"expression"`
}

type Call struct {
	Node
	Name      string       `json:"name"`
	Arguments []Expression `json:"arguments"`
}

type Index struct {
	Node
	Array Expression `json:"array"`
	Index Expression `json:"index"`
}

type Quantifier struct {
	Node
	Operator string     `json:"operator"`
	Variable Variable   `json:"variable"`
	Body     Expression `json:"body"`
}

// ArrayUpdate denotes a copy with one replaced element, not an in-place write.
type ArrayUpdate struct {
	Node
	Array Expression `json:"array"`
	Index Expression `json:"index"`
	Value Expression `json:"value"`
}

func (*Function) declaration() {}
func (*Formula) declaration()  {}

func (*Block) statement()      {}
func (*Assignment) statement() {}
func (*If) statement()         {}
func (*While) statement()      {}
func (*Assert) statement()     {}
func (*Assume) statement()     {}

func (*Identifier) expression()  {}
func (*Integer) expression()     {}
func (*Boolean) expression()     {}
func (*Unary) expression()       {}
func (*Binary) expression()      {}
func (*Group) expression()       {}
func (*Call) expression()        {}
func (*Index) expression()       {}
func (*Quantifier) expression()  {}
func (*ArrayUpdate) expression() {}
