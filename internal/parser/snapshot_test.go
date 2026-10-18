package parser_test

import (
	"encoding/json"
	"reflect"
	"testing"

	"compiler/internal/parser"
	"compiler/internal/source"
)

func TestProgramSnapshot(t *testing.T) {
	program := parseOK(t, string(fixture(t, "minimal.funny")))
	data, err := json.Marshal(program)
	if err != nil {
		t.Fatal(err)
	}
	var got, want any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(fixture(t, "minimal.json"), &want); err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(got, want) {
		actual, err := json.MarshalIndent(got, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		t.Errorf("AST differs from reviewed testdata/minimal.json\nactual:\n%s", actual)
	}
}

type diagnosticFixture struct {
	File string      `json:"file"`
	Span source.Span `json:"span"`
}

func TestDiagnosticPositions(t *testing.T) {
	var cases []diagnosticFixture
	if err := json.Unmarshal(fixture(t, "diagnostics.json"), &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) < 3 {
		t.Fatal("expected at least three diagnostic position fixtures")
	}
	for _, tt := range cases {
		t.Run(tt.File, func(t *testing.T) {
			input := string(fixture(t, tt.File))
			_, diagnostics := parser.Parse(scan(t, input))
			requireDiagnostics(t, input, diagnostics)
			// Cascading diagnostics and their wording are implementation details;
			// the primary error must still point at the exact offending token/EOF.
			requireSpan(t, diagnostics[0].Span, tt.Span)
		})
	}
}
