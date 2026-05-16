package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/kdubb1337/fin-cli/internal/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		// cmd.Execute already wrote the structured error to stderr;
		// just exit with the right code.
		var ec cmd.ExitCoder
		if errors.As(err, &ec) {
			os.Exit(ec.ExitCode())
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
