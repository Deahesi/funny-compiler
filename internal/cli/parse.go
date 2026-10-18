package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"

	"compiler/internal/lexer"
	"compiler/internal/parser"
	"compiler/internal/source"

	"github.com/spf13/cobra"
)

var ErrDiagnostics = errors.New("source diagnostics")

func newParseCommand() *cobra.Command {
	var outputPath string

	cmd := &cobra.Command{
		Use:   "parse <file|->",
		Short: "Parse source and export its AST as JSON",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			filename := args[0]
			var input []byte
			var err error
			if filename == "-" {
				filename = "<stdin>"
				input, err = io.ReadAll(cmd.InOrStdin())
			} else {
				input, err = os.ReadFile(filename)
			}
			if err != nil {
				return fmt.Errorf("read %q: %w", filename, err)
			}

			_, minimal, err := lexer.Build()
			if err != nil {
				return fmt.Errorf("build lexer: %w", err)
			}
			tokens, err := lexer.Scan(minimal, string(input))
			if err != nil {
				var diagnostic *source.Diagnostic
				if !errors.As(err, &diagnostic) {
					return fmt.Errorf("scan %q: %w", filename, err)
				}
				if err := writeParseDiagnostic(cmd.ErrOrStderr(), filename, diagnostic); err != nil {
					return err
				}
				return ErrDiagnostics
			}

			program, diagnostics := parser.Parse(tokens)
			for i := range diagnostics {
				if err := writeParseDiagnostic(cmd.ErrOrStderr(), filename, &diagnostics[i]); err != nil {
					return err
				}
			}

			data, err := json.MarshalIndent(program, "", "  ")
			if err != nil {
				return fmt.Errorf("encode AST: %w", err)
			}
			data = append(data, '\n')
			if cmd.Flags().Changed("out") {
				if err := os.WriteFile(outputPath, data, 0o644); err != nil {
					return fmt.Errorf("write AST to %q: %w", outputPath, err)
				}
			} else if err := writeParseOutput(cmd.OutOrStdout(), data); err != nil {
				return fmt.Errorf("write AST: %w", err)
			}

			if len(diagnostics) > 0 {
				return ErrDiagnostics
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&outputPath, "out", "o", "", "Output file for the JSON AST (stdout by default)")
	return cmd
}

func writeParseDiagnostic(writer io.Writer, filename string, diagnostic *source.Diagnostic) error {
	data := []byte(fmt.Sprintf("%s:%s\n", filename, diagnostic))
	if err := writeParseOutput(writer, data); err != nil {
		return fmt.Errorf("write diagnostic: %w", err)
	}
	return nil
}

func writeParseOutput(writer io.Writer, data []byte) error {
	n, err := writer.Write(data)
	if err == nil && n != len(data) {
		return io.ErrShortWrite
	}
	return err
}
