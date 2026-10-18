package lexer

import (
	"fmt"
	"unicode/utf8"

	"compiler/internal/automats"
	"compiler/internal/source"
)

type Token struct {
	Name  string      `json:"name"`
	Value string      `json:"value"`
	Span  source.Span `json:"span"`
}

func Build() (*automats.DFA, *automats.DFA, error) {
	nfa, err := automats.BuildNFA(Rules)

	if err != nil {
		return nil, nil, err
	}

	full := automats.Determinize(nfa)
	return full, automats.Minimize(full), nil
}

func Scan(dfa *automats.DFA, input string) ([]Token, error) {
	var tokens []Token
	position := source.Position{Offset: 0, Line: 1, Column: 1}
	for position.Offset < len(input) {
		start := position.Offset
		state := dfa.Start
		lastEnd := -1
		var lastAccept *automats.AcceptInfo

		for pos := start; pos < len(input); pos++ {
			state = dfa.Next(state, rune(input[pos]))
			if state == dfa.Trap {
				break
			}
			if accept := dfa.States[state].Accept; accept != nil {
				lastEnd = pos + 1
				lastAccept = accept
			}
		}

		if lastEnd == -1 {
			symbol, width := utf8.DecodeRuneInString(input[start:])
			return tokens, &source.Diagnostic{
				Span: source.Span{
					Start: position,
					End:   advancePosition(position, input, start+width),
				},
				Message: fmt.Sprintf("undefined symbol %q", symbol),
			}
		}
		end := advancePosition(position, input, lastEnd)
		span := source.Span{Start: position, End: end}
		if lastAccept.Error {
			return tokens, &source.Diagnostic{
				Span:    span,
				Message: fmt.Sprintf("invalid int %q", input[start:lastEnd]),
			}
		}
		if !lastAccept.Skip {
			tokens = append(tokens, Token{
				Name:  lastAccept.Token,
				Value: input[start:lastEnd],
				Span:  span,
			})
		}
		position = end
	}
	tokens = append(tokens, Token{
		Name:  "EOF",
		Value: "",
		Span:  source.Span{Start: position, End: position},
	})
	return tokens, nil
}

func advancePosition(position source.Position, input string, end int) source.Position {
	for position.Offset < end {
		symbol, width := utf8.DecodeRuneInString(input[position.Offset:end])
		switch symbol {
		case '\r':
			position.Line++
			position.Column = 1
		case '\n':
			if position.Offset == 0 || input[position.Offset-1] != '\r' {
				position.Line++
			}
			position.Column = 1
		default:
			position.Column++
		}
		position.Offset += width
	}
	return position
}
