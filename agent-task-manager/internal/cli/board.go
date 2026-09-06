package cli

import (
	"github.com/spf13/cobra"

	"github.com/ucpr/atama/internal/service"
	"github.com/ucpr/atama/internal/tui"
)

func newBoardCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "board",
		Short: "Launch the interactive TUI board",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			st, err := loadStore()
			if err != nil {
				return err
			}
			return tui.Run(service.New(st), st)
		},
	}
}
