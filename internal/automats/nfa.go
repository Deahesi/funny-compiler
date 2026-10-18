package automats

import (
	"fmt"
	"regexp/syntax"
)

const Epsilon = -1

type Rule struct {
	Name     string
	Regex    string
	Priority int
	Skip     bool
	Error    bool
}

type AcceptInfo struct {
	Token    string `json:"token"`
	Priority int    `json:"priority"`
	Skip     bool   `json:"skip"`
	Error    bool   `json:"error"`
	Order    int    `json:"-"`
}

type NFATransition struct {
	Symbol int
	To     int
}

type NFAState struct {
	Transitions []NFATransition
	Accept      *AcceptInfo
}

type NFA struct {
	Start  int
	States []NFAState
}

type fragment struct {
	start int
	end   int
}

func (nfa *NFA) newState() int {
	id := len(nfa.States)
	nfa.States = append(nfa.States, NFAState{})
	return id
}

func (nfa *NFA) add(from, symbol, to int) {
	nfa.States[from].Transitions = append(
		nfa.States[from].Transitions,
		NFATransition{Symbol: symbol, To: to},
	)
}

func BuildNFA(rules []Rule) (*NFA, error) {
	nfa := &NFA{}
	nfa.Start = nfa.newState()

	for i, rule := range rules {
		tree, err := syntax.Parse(rule.Regex, syntax.Perl)
		if err != nil {
			return nil, fmt.Errorf("правило %s: %w", rule.Name, err)
		}

		part, err := nfa.build(tree.Simplify())
		if err != nil {
			return nil, fmt.Errorf("правило %s: %w", rule.Name, err)
		}

		nfa.add(nfa.Start, Epsilon, part.start)
		nfa.States[part.end].Accept = &AcceptInfo{
			Token: rule.Name, Priority: rule.Priority,
			Skip: rule.Skip, Error: rule.Error, Order: i,
		}
	}

	return nfa, nil
}

func (nfa *NFA) empty() fragment {
	start := nfa.newState()
	end := nfa.newState()
	nfa.add(start, Epsilon, end)
	return fragment{start, end}
}

func (nfa *NFA) build(tree *syntax.Regexp) (fragment, error) {
	switch tree.Op {
	case syntax.OpEmptyMatch:
		return nfa.empty(), nil

	case syntax.OpLiteral:
		if len(tree.Rune) == 0 {
			return nfa.empty(), nil
		}
		start := nfa.newState()
		end := start
		for _, symbol := range tree.Rune {
			if symbol >= ASCII {
				return fragment{}, fmt.Errorf("symbol %q not ASCII", symbol)
			}
			next := nfa.newState()
			nfa.add(end, int(symbol), next)
			end = next
		}
		return fragment{start, end}, nil

	case syntax.OpCharClass:
		start := nfa.newState()
		end := nfa.newState()
		for symbol := 0; symbol < ASCII; symbol++ {
			for i := 0; i < len(tree.Rune); i += 2 {
				if int(tree.Rune[i]) <= symbol && symbol <= int(tree.Rune[i+1]) {
					nfa.add(start, symbol, end)
					break
				}
			}
		}
		return fragment{start, end}, nil

	case syntax.OpConcat:
		first, err := nfa.build(tree.Sub[0])
		if err != nil {
			return fragment{}, err
		}
		for _, child := range tree.Sub[1:] {
			next, err := nfa.build(child)
			if err != nil {
				return fragment{}, err
			}
			nfa.add(first.end, Epsilon, next.start)
			first.end = next.end
		}
		return first, nil

	case syntax.OpAlternate:
		start := nfa.newState()
		end := nfa.newState()
		for _, child := range tree.Sub {
			part, err := nfa.build(child)
			if err != nil {
				return fragment{}, err
			}
			nfa.add(start, Epsilon, part.start)
			nfa.add(part.end, Epsilon, end)
		}
		return fragment{start, end}, nil

	case syntax.OpStar, syntax.OpPlus, syntax.OpQuest:
		start := nfa.newState()
		end := nfa.newState()
		part, err := nfa.build(tree.Sub[0])
		if err != nil {
			return fragment{}, err
		}
		nfa.add(start, Epsilon, part.start)
		nfa.add(part.end, Epsilon, end)
		if tree.Op != syntax.OpPlus {
			nfa.add(start, Epsilon, end)
		}
		if tree.Op != syntax.OpQuest {
			nfa.add(part.end, Epsilon, part.start)
		}
		return fragment{start, end}, nil

	case syntax.OpCapture:
		return nfa.build(tree.Sub[0])
	}

	return fragment{}, fmt.Errorf("undefined op: %s", tree.Op)
}
