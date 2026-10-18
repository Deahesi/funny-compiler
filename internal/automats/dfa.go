package automats

import (
	"sort"
	"strconv"
	"strings"
)

const ASCII = 128

type DFAState struct {
	Transitions [ASCII]int  `json:"transitions"`
	Accept      *AcceptInfo `json:"accept"`
}

type DFA struct {
	Start  int        `json:"start"`
	Trap   int        `json:"trap"`
	States []DFAState `json:"states"`
}

// Next отправляет любой символ вне ASCII в ловушку.
func (dfa *DFA) Next(state int, symbol rune) int {
	if symbol < 0 || symbol >= ASCII {
		return dfa.Trap
	}
	return dfa.States[state].Transitions[symbol]
}

func closure(nfa *NFA, states map[int]bool) []int {
	stack := make([]int, 0, len(states))
	for state := range states {
		stack = append(stack, state)
	}
	for len(stack) > 0 {
		state := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		for _, edge := range nfa.States[state].Transitions {
			if edge.Symbol == Epsilon && !states[edge.To] {
				states[edge.To] = true
				stack = append(stack, edge.To)
			}
		}
	}
	result := make([]int, 0, len(states))
	for state := range states {
		result = append(result, state)
	}
	sort.Ints(result)
	return result
}

func subsetKey(states []int) string {
	var text strings.Builder
	for _, state := range states {
		text.WriteString(strconv.Itoa(state))
		text.WriteByte(',')
	}
	return text.String()
}

func bestAccept(nfa *NFA, states []int) *AcceptInfo {
	var best *AcceptInfo
	for _, state := range states {
		accept := nfa.States[state].Accept
		if accept != nil && (best == nil ||
			accept.Priority > best.Priority ||
			accept.Priority == best.Priority && accept.Order < best.Order) {
			best = accept
		}
	}
	return best
}

func Determinize(nfa *NFA) *DFA {
	start := closure(nfa, map[int]bool{nfa.Start: true})
	subsets := [][]int{start}

	seen := map[string]int{subsetKey(start): 0}
	dfa := &DFA{Start: 0, States: []DFAState{{Accept: bestAccept(nfa, start)}}}

	for state := 0; state < len(subsets); state++ {
		for symbol := 0; symbol < ASCII; symbol++ {
			moved := make(map[int]bool)
			for _, from := range subsets[state] {
				for _, edge := range nfa.States[from].Transitions {
					if edge.Symbol == symbol {
						moved[edge.To] = true
					}
				}
			}
			if len(moved) == 0 {
				dfa.States[state].Transitions[symbol] = -1
				continue
			}
			next := closure(nfa, moved)
			key := subsetKey(next)
			id, found := seen[key]
			if !found {
				id = len(subsets)
				seen[key] = id
				subsets = append(subsets, next)
				dfa.States = append(dfa.States, DFAState{Accept: bestAccept(nfa, next)})
			}
			dfa.States[state].Transitions[symbol] = id
		}
	}

	dfa.Trap = len(dfa.States)
	dfa.States = append(dfa.States, DFAState{})
	for state := range dfa.States {
		for symbol := 0; symbol < ASCII; symbol++ {
			if state == dfa.Trap || dfa.States[state].Transitions[symbol] == -1 {
				dfa.States[state].Transitions[symbol] = dfa.Trap
			}
		}
	}
	return dfa
}
