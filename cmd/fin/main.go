package main

import (
	"errors"
	"fmt"
	"os"

	"github.com/kdubb1337/fin-cli/internal/cmd"
	finerr "github.com/kdubb1337/fin-cli/internal/errors"
)

func main() {
	if err := cmd.Execute(); err != nil {
		// cmd.Execute already wrote the structured error to stderr;
		// just exit with the right code.
		var ee *finerr.ExitError
		if errors.As(err, &ee) {
			fmt.Fprintln(os.Stderr, "error:", ee)
			os.Exit(ee.Code())
		}
		var ec cmd.ExitCoder
		if errors.As(err, &ec) {
			os.Exit(ec.ExitCode())
		}
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(finerr.CodeGeneric)
	}
}
