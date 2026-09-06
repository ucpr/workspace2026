// Command atama is a task-management CLI/TUI for delegating work to coding
// agents, per specs/requirements.md.
package main

import (
	"os"

	"github.com/ucpr/atama/internal/cli"
)

func main() {
	root := cli.NewRootCmd()
	if err := root.Execute(); err != nil {
		cli.ReportError(err)
		os.Exit(cli.ExitCodeFor(err))
	}
}
