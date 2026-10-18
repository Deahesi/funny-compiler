package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"compiler/internal/lexer"
)

func executeCommand(args ...string) (string, error) {
	var output bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return output.String(), err
}

func TestDFACommand(t *testing.T) {
	full, minimal, err := lexer.Build()
	if err != nil {
		t.Fatal(err)
	}
	want, err := json.MarshalIndent(minimal, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	want = append(want, '\n')

	for _, flag := range []string{"--out", "-o"} {
		t.Run(flag, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "dfa.json")
			output, err := executeCommand("dfa", flag, path)
			if err != nil {
				t.Fatal(err)
			}
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Error("exported DFA does not match the lexer's minimized DFA")
			}
			if !strings.Contains(output, "DFA:") || !strings.Contains(output, path) {
				t.Errorf("missing DFA summary or output path: %q", output)
			}
		})
	}

	if len(minimal.States) > len(full.States) {
		t.Error("minimization increased the number of states")
	}
}

func TestDFADefaultOutput(t *testing.T) {
	t.Chdir(t.TempDir())
	if _, err := executeCommand("dfa"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat("dfa.json"); err != nil {
		t.Fatalf("default output file was not created: %v", err)
	}
}

func TestDFAWriteError(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing", "dfa.json")
	output, err := executeCommand("dfa", "--out", path)
	if err == nil || !strings.Contains(err.Error(), "write DFA:") || !strings.Contains(err.Error(), path) {
		t.Fatalf("expected a write error with the output path, got %v", err)
	}
	if output != "" {
		t.Errorf("unexpected output on failure: %q", output)
	}
}

func TestCommandHelp(t *testing.T) {
	for _, args := range [][]string{nil, {"--help"}, {"dfa", "--help"}} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			output, err := executeCommand(args...)
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(output, "Usage:") || !strings.Contains(output, "dfa") {
				t.Errorf("unexpected help output: %q", output)
			}
			if strings.Contains(output, "DFA:") {
				t.Errorf("help unexpectedly ran the lexer: %q", output)
			}
		})
	}
}

func TestInvalidArguments(t *testing.T) {
	for _, args := range [][]string{
		{"unknown"},
		{"dfa", "unexpected"},
		{"dfa", "--unknown"},
		{"dfa", "--out"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			output, err := executeCommand(args...)
			if err == nil {
				t.Fatal("expected an argument error")
			}
			if output != "" {
				t.Errorf("command should return errors without printing them: %q", output)
			}
		})
	}
}
