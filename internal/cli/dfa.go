package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"compiler/internal/lexer"

	"github.com/spf13/cobra"
)

func newDFACommand() *cobra.Command {
	var outputPath string

	cmd := &cobra.Command{
		Use:   "dfa",
		Short: "Build and export the lexer's minimized DFA",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			full, minimal, err := lexer.Build()
			if err != nil {
				return fmt.Errorf("build DFA: %w", err)
			}

			data, err := json.MarshalIndent(minimal, "", "  ")
			if err != nil {
				return fmt.Errorf("encode DFA: %w", err)
			}
			if err := os.WriteFile(outputPath, append(data, '\n'), 0o644); err != nil {
				return fmt.Errorf("write DFA: %w", err)
			}

			_, err = fmt.Fprintf(cmd.OutOrStdout(),
				"DFA: %d before minimize, %d after; trap: %d\ntransitions table: %s\n",
				len(full.States), len(minimal.States), minimal.Trap, outputPath)
			return err
		},
	}
	cmd.Flags().StringVarP(&outputPath, "out", "o", "dfa.json", "Output file for the DFA transition table")
	return cmd
}
