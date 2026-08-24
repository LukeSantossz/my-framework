// Command mf activates and enforces this repository's development standards.
package main

import (
	"os"

	"github.com/LukeSantossz/my-framework/internal/cli"
)

func main() {
	os.Exit(cli.Run(cli.Env{
		Args:   os.Args[1:],
		Stdout: os.Stdout,
		Stderr: os.Stderr,
	}))
}
