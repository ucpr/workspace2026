package cli

import (
	"github.com/spf13/cobra"
)

func newNextCmd() *cobra.Command {
	var goal string
	cmd := &cobra.Command{
		Use:   "next",
		Short: "Show the next Ready task(s) to work on",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := loadService()
			if err != nil {
				return err
			}
			goalID := ""
			if goal != "" {
				goalID, err = svc.ResolveGoalID(goal)
				if err != nil {
					return err
				}
			}
			ready, err := svc.Next(goalID)
			if err != nil {
				return err
			}
			all, err := svc.TasksMap()
			if err != nil {
				return err
			}
			views := newTaskViews(ready, all)
			if isJSON() {
				return printJSON(views)
			}
			if len(views) == 0 {
				printText("No ready tasks.")
				return nil
			}
			printText("Next task: %s  %s", views[0].ID, views[0].Title)
			if len(views) > 1 {
				printText("(%d more ready)", len(views)-1)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&goal, "goal", "", "restrict to a goal's tasks (ID/prefix)")
	return cmd
}
