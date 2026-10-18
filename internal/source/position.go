package source

import "fmt"

type Position struct {
	Offset int `json:"offset"`
	Line   int `json:"line"`
	Column int `json:"column"`
}

type Span struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type Diagnostic struct {
	Span    Span   `json:"span"`
	Message string `json:"message"`
}

func (diagnostic *Diagnostic) Error() string {
	return fmt.Sprintf("%d:%d: %s", diagnostic.Span.Start.Line, diagnostic.Span.Start.Column, diagnostic.Message)
}
