package parser_test

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"compiler/internal/source"
)

// These checks can also run before parser.Parse exists, from internal/parser:
// go test helper_test.go fixtures_test.go
func TestFixtureSourcesAreLexicallyValid(t *testing.T) {
	paths, err := filepath.Glob("testdata/*.funny")
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) < 6 {
		t.Fatalf("expected at least six source fixtures, got %d", len(paths))
	}
	for _, path := range paths {
		name := filepath.Base(path)
		t.Run(name, func(t *testing.T) {
			scan(t, string(fixture(t, name)))
		})
	}
}

func TestReferenceJSONSpansMatchSource(t *testing.T) {
	// Validate the hand-authored reference independently of any future parser
	// output, including line/column coordinates for all nested snapshot spans.
	var snapshot any
	if err := json.Unmarshal(fixture(t, "minimal.json"), &snapshot); err != nil {
		t.Fatal(err)
	}
	checkReferenceSpans(t, string(fixture(t, "minimal.funny")), snapshot)

	var diagnostics []struct {
		File string      `json:"file"`
		Span source.Span `json:"span"`
	}
	if err := json.Unmarshal(fixture(t, "diagnostics.json"), &diagnostics); err != nil {
		t.Fatal(err)
	}
	for _, diagnostic := range diagnostics {
		t.Run(diagnostic.File, func(t *testing.T) {
			input := string(fixture(t, diagnostic.File))
			checkReferenceSpan(t, input, diagnostic.Span)
		})
	}
}

func checkReferenceSpan(t testing.TB, input string, span source.Span) {
	t.Helper()
	start, end := span.Start.Offset, span.End.Offset
	if start < 0 || end < start || end > len(input) {
		t.Fatalf("reference span outside source: %+v", span)
	}
	requireSpan(t, span, spanAt(input, start, end))
}

func checkReferenceSpans(t testing.TB, input string, value any) {
	t.Helper()
	switch value := value.(type) {
	case map[string]any:
		if raw, ok := value["span"]; ok {
			data, err := json.Marshal(raw)
			if err != nil {
				t.Fatal(err)
			}
			var span source.Span
			if err := json.Unmarshal(data, &span); err != nil {
				t.Fatal(err)
			}
			checkReferenceSpan(t, input, span)
		}
		for _, child := range value {
			checkReferenceSpans(t, input, child)
		}
	case []any:
		for _, child := range value {
			checkReferenceSpans(t, input, child)
		}
	}
}
