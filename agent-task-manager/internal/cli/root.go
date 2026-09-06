// Package cli wires cobra commands to package service, and handles the
// human/JSON dual output format required by requirements §5.6.
package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/ucpr/atama/internal/service"
	"github.com/ucpr/atama/internal/store"
)

// outputFormat holds the --output flag value, read by every command's RunE
// via isJSON.
var outputFormat string

// dir holds the --dir flag value: the directory to treat as the working
// directory when locating or creating a store. Defaults to the real cwd.
var dir string

// NewRootCmd builds the atama command tree.
func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "atama",
		Short:         "atama manages tasks and their dependencies for delegated agent execution",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().StringVar(&outputFormat, "output", "text", `output format: "text" or "json"`)
	root.PersistentFlags().StringVar(&dir, "dir", "", "directory to operate in (default: current directory)")

	root.AddCommand(
		newInitCmd(),
		newTaskCmd(),
		newDepCmd(),
		newGoalCmd(),
		newNextCmd(),
		newCompleteCmd(),
		newGitHubCmd(),
	)
	return root
}

func isJSON() bool { return outputFormat == "json" }

func workDir() (string, error) {
	if dir != "" {
		return dir, nil
	}
	return os.Getwd()
}

// loadService locates the store for the current directory and returns a
// ready-to-use service.
func loadService() (*service.Service, error) {
	st, err := loadStore()
	if err != nil {
		return nil, err
	}
	return service.New(st), nil
}

// loadStore locates the store for the current directory. Most commands go
// through loadService instead; the github command group uses the store
// directly since it needs field-level control service.Service doesn't
// expose (requirements §8.1: github sync is a separate, independently
// testable layer built on the same store).
func loadStore() (*store.Store, error) {
	wd, err := workDir()
	if err != nil {
		return nil, err
	}
	return store.Find(wd)
}
