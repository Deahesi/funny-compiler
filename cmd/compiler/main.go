package main

import (
	"errors"
	"fmt"
	"os"

	"compiler/internal/cli"
)

func main() {
	if err := cli.NewRootCommand().Execute(); err != nil {
		if !errors.Is(err, cli.ErrDiagnostics) {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(1)
	}
}
