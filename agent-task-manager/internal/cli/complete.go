package cli

import (
	"github.com/spf13/cobra"
)

func newCompleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "complete <id>",
		Short: "Report a task as done",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := loadService()
			if err != nil {
				return err
			}
			taskID, err := svc.ResolveTaskID(args[0])
			if err != nil {
				return err
			}
			task, err := svc.Complete(taskID)
			if err != nil {
				return err
			}
			return outputTask(svc, task)
		},
	}
}
