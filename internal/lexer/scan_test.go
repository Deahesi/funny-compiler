package lexer

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"compiler/internal/automats"
	"compiler/internal/source"
)

func TestCases(t *testing.T) {
	full, minimal, err := Build()
	if err != nil {
		t.Fatal(err)
	}
	if len(minimal.States) > len(full.States) {
		t.Fatal("после минимизации состояний стало больше")
	}
	for _, example := range cases {
		t.Run(example.Name, func(t *testing.T) {
			for _, dfa := range []*automats.DFA{full, minimal} {
				got, scanErr := Scan(dfa, example.Input)
				var values []tokenValue
				for _, token := range got {
					values = append(values, tokenValue{Name: token.Name, Value: token.Value})
				}
				if !reflect.DeepEqual(values, example.Want) || (scanErr != nil) != example.WantError {
					t.Errorf("got %v, error %v; want %v, error %v",
						values, scanErr, example.Want, example.WantError)
				}
				if scanErr != nil {
					if _, ok := scanErr.(*source.Diagnostic); !ok {
						t.Errorf("expected *source.Diagnostic, got %T", scanErr)
					}
				}
			}
		})
	}
}

func TestEveryASCIICharacter(t *testing.T) {
	_, dfa, err := Build()
	if err != nil {
		t.Fatal(err)
	}
	for symbol := 0; symbol < automats.ASCII; symbol++ {
		input := string(rune(symbol))
		_, scanErr := Scan(dfa, input)
		valid := strings.ContainsRune(" \t\r\n()[]{},;:|+-*/<>=", rune(symbol)) ||
			'A' <= symbol && symbol <= 'Z' ||
			'a' <= symbol && symbol <= 'z' ||
			'0' <= symbol && symbol <= '9'
		if (scanErr == nil) != valid {
			t.Errorf("ASCII %d (%q): error %v, valid %v", symbol, input, scanErr, valid)
		}
		if target := dfa.Next(dfa.Start, rune(symbol)); target < 0 || target >= len(dfa.States) {
			t.Errorf("ASCII %d: неверный переход %d", symbol, target)
		}
	}
	if dfa.Next(dfa.Start, 'é') != dfa.Trap {
		t.Error("не-ASCII символ не ведёт в ловушку")
	}
	for symbol := 0; symbol < automats.ASCII; symbol++ {
		if dfa.Next(dfa.Trap, rune(symbol)) != dfa.Trap {
			t.Errorf("ловушка выходит из себя по символу %d", symbol)
		}
	}
}

func TestKeywordsAndPrefixes(t *testing.T) {
	full, minimal, err := Build()
	if err != nil {
		t.Fatal(err)
	}
	for _, keyword := range keywords {
		t.Run(keyword.Name, func(t *testing.T) {
			input := strings.Join([]string{
				keyword.Value, keyword.Value + "x", keyword.Value + "2",
				"x" + keyword.Value, strings.ToUpper(keyword.Value),
				keyword.Value[:len(keyword.Value)-1],
			}, " ")
			want := []tokenValue{
				keyword,
				{Name: "IDENT", Value: keyword.Value + "x"},
				{Name: "IDENT", Value: keyword.Value + "2"},
				{Name: "IDENT", Value: "x" + keyword.Value},
				{Name: "IDENT", Value: strings.ToUpper(keyword.Value)},
				{Name: "IDENT", Value: keyword.Value[:len(keyword.Value)-1]},
				{Name: "EOF"},
			}
			for _, dfa := range []*automats.DFA{full, minimal} {
				got, err := Scan(dfa, input)
				if err != nil {
					t.Fatal(err)
				}
				var values []tokenValue
				for _, token := range got {
					values = append(values, tokenValue{Name: token.Name, Value: token.Value})
				}
				if !reflect.DeepEqual(values, want) {
					t.Errorf("got %v; want %v", values, want)
				}
			}
		})
	}
}

func TestBuildAndScanDeterministic(t *testing.T) {
	var wantDFA [2][]byte
	var wantTokens [2][]Token
	const input = "function f(x: int) returns int\r\nrequires true ensures not false uses a; // comment\nforall i: int | i >= 0 => exists j: int | i -> j"
	for attempt := 0; attempt < 5; attempt++ {
		full, minimal, err := Build()
		if err != nil {
			t.Fatal(err)
		}
		for i, dfa := range []*automats.DFA{full, minimal} {
			data, err := json.Marshal(dfa)
			if err != nil {
				t.Fatal(err)
			}
			before := automats.DFA{
				Start: dfa.Start, Trap: dfa.Trap,
				States: append([]automats.DFAState(nil), dfa.States...),
			}
			for repeat := 0; repeat < 2; repeat++ {
				tokens, err := Scan(dfa, input)
				if err != nil {
					t.Fatal(err)
				}
				if attempt == 0 && repeat == 0 {
					wantTokens[i] = tokens
				}
				if !reflect.DeepEqual(tokens, wantTokens[i]) {
					t.Fatalf("scan is not deterministic: DFA %d, attempt %d", i, attempt)
				}
			}
			if !reflect.DeepEqual(*dfa, before) {
				t.Fatal("Scan mutated the DFA")
			}
			if attempt == 0 {
				wantDFA[i] = data
			} else if !bytes.Equal(data, wantDFA[i]) {
				t.Fatalf("Build is not deterministic: DFA %d, attempt %d", i, attempt)
			}
		}
	}
	if !reflect.DeepEqual(wantTokens[0], wantTokens[1]) {
		t.Fatal("full and minimized DFA produced different token spans")
	}
}

func TestMinimizedDFAEquivalent(t *testing.T) {
	full, minimal, err := Build()
	if err != nil {
		t.Fatal(err)
	}

	type pair struct{ full, minimal int }
	start := pair{full: full.Start, minimal: minimal.Start}
	queue := []pair{start}
	seen := map[pair]bool{start: true}
	for i := 0; i < len(queue); i++ {
		current := queue[i]
		left := full.States[current.full].Accept
		right := minimal.States[current.minimal].Accept
		if !reflect.DeepEqual(left, right) {
			t.Fatalf("разные принимающие состояния: %v и %v", left, right)
		}
		for symbol := 0; symbol < automats.ASCII; symbol++ {
			next := pair{
				full:    full.States[current.full].Transitions[symbol],
				minimal: minimal.States[current.minimal].Transitions[symbol],
			}
			if !seen[next] {
				seen[next] = true
				queue = append(queue, next)
			}
		}
	}
}
