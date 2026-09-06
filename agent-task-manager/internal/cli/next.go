package cli

import (
	"github.com/spf13/cobra"

	"github.com/ucpr/atama/internal/service"
)

func newNextCmd() *cobra.Command {
	var (
		goal     string
		worktree bool
	)
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

			// --worktree hands the top task an isolated git worktree in the
			// same call, so an agent's next->execute->complete loop (§8.2)
			// can go straight from `next` to `cd`-ing into its own checkout.
			if worktree && len(ready) > 0 && ready[0].Execution.Worktree == nil {
				updated, err := svc.CreateWorktree(ready[0].ID, service.CreateWorktreeInput{})
				if err != nil {
					return err
				}
				ready[0] = updated
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
			if wt := views[0].Execution.Worktree; wt != nil {
				printText("worktree: %s  (branch %s)", wt.Path, wt.Branch)
			}
			if len(views) > 1 {
				printText("(%d more ready)", len(views)-1)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&goal, "goal", "", "restrict to a goal's tasks (ID/prefix)")
	cmd.Flags().BoolVar(&worktree, "worktree", false, "also create a git worktree for the top task, if it doesn't have one yet")
	return cmd
}
