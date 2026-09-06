package cli

import (
	"github.com/spf13/cobra"

	"github.com/ucpr/atama/internal/store"
)

func newInitCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialize a task store in the current directory",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			wd, err := workDir()
			if err != nil {
				return err
			}
			s, err := store.Init(wd)
			if err != nil {
				return err
			}
			if isJSON() {
				return printJSON(map[string]string{"store": s.Root()})
			}
			printText("Initialized empty atama store at %s", s.Root())
			return nil
		},
	}
}
