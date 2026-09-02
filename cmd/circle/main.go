// Command circle encodes a project's execution, quality and knowledge contracts
// into the repository, then enforces them.
package main

import (
	"os"

	"github.com/edwardmontoya/circle/internal/cli"
)

func main() { os.Exit(cli.Main()) }
