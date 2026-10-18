package cli

import "github.com/spf13/cobra"

// NewRootCommand creates the compiler CLI with independent command and flag state.
func NewRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:           "compiler",
		Short:         "Compiler tools",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	cmd.AddCommand(newDFACommand(), newParseCommand())
	return cmd
}
