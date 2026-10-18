package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp/syntax"
	"strings"
	"testing"

	"compiler/internal/lexer"
	"compiler/internal/parser"
	"compiler/internal/source"
)

const parseValidInput = "f(x: int) returns r: int { r = x + 1; }\npositive(x: int) => x > 0\n"
const parseSyntaxInput = "f(x: int) returns r: int {\n r = ;\n r = ;\n}\npositive(x: int) => x > 0\n"

func executeParseCommand(input io.Reader, args ...string) (string, string, error) {
	var stdout, stderr bytes.Buffer
	cmd := NewRootCommand()
	cmd.SetIn(input)
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(append([]string{"parse"}, args...))
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func expectedParseJSON(t *testing.T, input string) ([]byte, []source.Diagnostic) {
	t.Helper()
	_, minimal, err := lexer.Build()
	if err != nil {
		t.Fatal(err)
	}
	tokens, err := lexer.Scan(minimal, input)
	if err != nil {
		t.Fatal(err)
	}
	program, diagnostics := parser.Parse(tokens)
	if program == nil || len(program.Declarations) == 0 {
		t.Fatal("test input must produce a nonempty AST")
	}
	data, err := json.MarshalIndent(program, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	return append(data, '\n'), diagnostics
}

func TestParseCommand(t *testing.T) {
	want, diagnostics := expectedParseJSON(t, parseValidInput)
	if len(diagnostics) != 0 {
		t.Fatalf("valid input produced diagnostics: %v", diagnostics)
	}

	for _, fromFile := range []bool{false, true} {
		for _, flag := range []string{"", "--out", "-o"} {
			t.Run(fmt.Sprintf("file=%t/out=%s", fromFile, flag), func(t *testing.T) {
				filename := "-"
				var input io.Reader = strings.NewReader(parseValidInput)
				if fromFile {
					filename = filepath.Join(t.TempDir(), "source.txt")
					if err := os.WriteFile(filename, []byte(parseValidInput), 0o644); err != nil {
						t.Fatal(err)
					}
					input = strings.NewReader("@")
				}
				args := []string{filename}
				outputPath := filepath.Join(t.TempDir(), "ast.json")
				if flag != "" {
					args = append(args, flag, outputPath)
				}
				stdout, stderr, err := executeParseCommand(input, args...)
				if err != nil {
					t.Fatal(err)
				}
				if stderr != "" {
					t.Errorf("unexpected diagnostics: %q", stderr)
				}
				got := []byte(stdout)
				if flag != "" {
					if stdout != "" {
						t.Errorf("unexpected stdout with --out: %q", stdout)
					}
					got, err = os.ReadFile(outputPath)
					if err != nil {
						t.Fatal(err)
					}
					info, err := os.Stat(outputPath)
					if err != nil {
						t.Fatal(err)
					}
					if info.Mode().Perm() != 0o644 {
						t.Errorf("output permissions = %o, want 644", info.Mode().Perm())
					}
				}
				if !json.Valid(got) || !bytes.Equal(got, want) {
					t.Errorf("JSON output = %s, want %s", got, want)
				}
			})
		}
	}
}

func TestParseFunctionAndLengthIdentifiers(t *testing.T) {
	input := "function(length: int) returns r: int { r = length; }"
	want, diagnostics := expectedParseJSON(t, input)
	if len(diagnostics) != 0 {
		t.Fatalf("identifiers produced diagnostics: %v", diagnostics)
	}
	stdout, stderr, err := executeParseCommand(strings.NewReader(input), "-")
	if err != nil || stderr != "" || stdout != string(want) {
		t.Fatalf("stdout = %q, stderr = %q, error = %v", stdout, stderr, err)
	}
}

func TestParseSyntaxDiagnostics(t *testing.T) {
	want, diagnostics := expectedParseJSON(t, parseSyntaxInput)
	if len(diagnostics) < 2 {
		t.Fatalf("test input must produce multiple syntax diagnostics, got %v", diagnostics)
	}
	for _, fromFile := range []bool{false, true} {
		for _, toFile := range []bool{false, true} {
			t.Run(fmt.Sprintf("file=%t/out=%t", fromFile, toFile), func(t *testing.T) {
				filename, label := "-", "<stdin>"
				if fromFile {
					filename = filepath.Join(t.TempDir(), "broken.txt")
					label = filename
					if err := os.WriteFile(filename, []byte(parseSyntaxInput), 0o644); err != nil {
						t.Fatal(err)
					}
				}
				args := []string{filename}
				outputPath := filepath.Join(t.TempDir(), "partial.json")
				if toFile {
					args = append(args, "--out", outputPath)
				}
				stdout, stderr, err := executeParseCommand(strings.NewReader(parseSyntaxInput), args...)
				if !errors.Is(err, ErrDiagnostics) {
					t.Fatalf("expected ErrDiagnostics, got %v", err)
				}
				var expectedDiagnostics strings.Builder
				for i := range diagnostics {
					fmt.Fprintf(&expectedDiagnostics, "%s:%s\n", label, &diagnostics[i])
				}
				if stderr != expectedDiagnostics.String() {
					t.Errorf("stderr = %q, want %q", stderr, expectedDiagnostics.String())
				}
				got := []byte(stdout)
				if toFile {
					if stdout != "" {
						t.Errorf("unexpected stdout with --out: %q", stdout)
					}
					got, err = os.ReadFile(outputPath)
					if err != nil {
						t.Fatal(err)
					}
				}
				if !bytes.Equal(got, want) {
					t.Errorf("partial AST = %s, want %s", got, want)
				}
			})
		}
	}
}

func TestParseLexicalDiagnostics(t *testing.T) {
	for _, tc := range []struct {
		name, input, diagnostic string
	}{
		{"symbol", parseValidInput + "@", "3:1: undefined symbol '@'\n"},
		{"integer", parseValidInput + "01", "3:1: invalid int \"01\"\n"},
		{"syntax prefix", parseSyntaxInput + "@", "6:1: undefined symbol '@'\n"},
	} {
		for _, fromFile := range []bool{false, true} {
			for _, toFile := range []bool{false, true} {
				t.Run(fmt.Sprintf("%s/file=%t/out=%t", tc.name, fromFile, toFile), func(t *testing.T) {
					filename, label := "-", "<stdin>"
					if fromFile {
						filename = filepath.Join(t.TempDir(), "lexical.txt")
						label = filename
						if err := os.WriteFile(filename, []byte(tc.input), 0o644); err != nil {
							t.Fatal(err)
						}
					}
					args := []string{filename}
					outputPath := filepath.Join(t.TempDir(), "untouched.json")
					if toFile {
						if err := os.WriteFile(outputPath, []byte("keep"), 0o644); err != nil {
							t.Fatal(err)
						}
						args = append(args, "-o", outputPath)
					}
					stdout, stderr, err := executeParseCommand(strings.NewReader(tc.input), args...)
					if !errors.Is(err, ErrDiagnostics) {
						t.Fatalf("expected ErrDiagnostics, got %v", err)
					}
					if stdout != "" || stderr != label+":"+tc.diagnostic {
						t.Errorf("stdout = %q, stderr = %q", stdout, stderr)
					}
					if toFile {
						got, err := os.ReadFile(outputPath)
						if err != nil {
							t.Fatal(err)
						}
						if string(got) != "keep" {
							t.Errorf("lexical failure changed output file: %q", got)
						}
					}
				})
			}
		}
	}
}

func TestParseInvalidArguments(t *testing.T) {
	for _, args := range [][]string{
		{"parse"},
		{"parse", "-", "extra"},
		{"parse", "-", "--unknown"},
		{"parse", "-", "--out"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			output, err := executeCommand(args...)
			if err == nil || output != "" {
				t.Fatalf("output = %q, error = %v; want an argument error without output", output, err)
			}
		})
	}
}

func TestParseHelp(t *testing.T) {
	output, err := executeCommand("parse", "--help")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(output, "parse <file|->") || !strings.Contains(output, "--out") {
		t.Errorf("unexpected help: %q", output)
	}
}

func TestParseFileErrors(t *testing.T) {
	missingInput := filepath.Join(t.TempDir(), "missing.txt")
	missingOutput := filepath.Join(t.TempDir(), "missing", "ast.json")
	for _, tc := range []struct {
		name, context string
		args          []string
	}{
		{"input", "read", []string{missingInput}},
		{"output", "write AST", []string{"-", "--out", missingOutput}},
		{"empty output", "write AST", []string{"-", "--out", ""}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			stdout, stderr, err := executeParseCommand(strings.NewReader(parseValidInput), tc.args...)
			if !errors.Is(err, os.ErrNotExist) || !strings.Contains(err.Error(), tc.context) {
				t.Fatalf("expected a wrapped file error with %q context, got %v", tc.context, err)
			}
			if stdout != "" || stderr != "" {
				t.Errorf("unexpected output: stdout = %q, stderr = %q", stdout, stderr)
			}
		})
	}
}

type parseErrorReader struct{ err error }

func (reader parseErrorReader) Read(data []byte) (int, error) {
	return copy(data, parseValidInput), reader.err
}

func TestParseReadError(t *testing.T) {
	failure := errors.New("reader failed")
	stdout, stderr, err := executeParseCommand(parseErrorReader{failure}, "-")
	if !errors.Is(err, failure) || !strings.Contains(err.Error(), "read \"<stdin>\"") {
		t.Fatalf("expected a wrapped stdin read error, got %v", err)
	}
	if stdout != "" || stderr != "" {
		t.Errorf("unexpected output on read failure: stdout = %q, stderr = %q", stdout, stderr)
	}
}

type parseErrorWriter struct{ err error }

func (writer parseErrorWriter) Write(data []byte) (int, error) {
	return 0, writer.err
}

func TestParseWriteErrors(t *testing.T) {
	failure := errors.New("writer failed")
	for _, tc := range []struct {
		name, input, context string
		stderr               bool
	}{
		{"AST", parseValidInput, "write AST", false},
		{"partial AST", parseSyntaxInput, "write AST", false},
		{"lexical diagnostic", "@", "write diagnostic", true},
		{"syntax diagnostic", parseSyntaxInput, "write diagnostic", true},
	} {
		for _, short := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/short=%t", tc.name, short), func(t *testing.T) {
				writer := parseErrorWriter{failure}
				want := failure
				if short {
					writer.err = nil
					want = io.ErrShortWrite
				}
				var stdout, stderr bytes.Buffer
				cmd := NewRootCommand()
				cmd.SetIn(strings.NewReader(tc.input))
				cmd.SetOut(&stdout)
				cmd.SetErr(&stderr)
				if tc.stderr {
					cmd.SetErr(writer)
				} else {
					cmd.SetOut(writer)
				}
				cmd.SetArgs([]string{"parse", "-"})
				err := cmd.Execute()
				if !errors.Is(err, want) || !strings.Contains(err.Error(), tc.context) {
					t.Fatalf("expected a wrapped %v with %q context, got %v", want, tc.context, err)
				}
				if errors.Is(err, ErrDiagnostics) {
					t.Error("write failure must not be suppressed as source diagnostics")
				}
				if tc.stderr && stdout.Len() != 0 {
					t.Errorf("unexpected AST after diagnostic write failure: %q", stdout.String())
				}
			})
		}
	}
}

func TestParseBuildError(t *testing.T) {
	originalRules := lexer.Rules
	lexer.Rules = append(lexer.Rules[:0:0], lexer.Rules...)
	lexer.Rules[0].Regex = "("
	t.Cleanup(func() { lexer.Rules = originalRules })

	stdout, stderr, err := executeParseCommand(strings.NewReader(parseValidInput), "-")
	var regexError *syntax.Error
	if !errors.As(err, &regexError) || !strings.Contains(err.Error(), "build lexer") {
		t.Fatalf("expected a wrapped lexer build error, got %v", err)
	}
	if stdout != "" || stderr != "" {
		t.Errorf("unexpected output on build failure: stdout = %q, stderr = %q", stdout, stderr)
	}
}
