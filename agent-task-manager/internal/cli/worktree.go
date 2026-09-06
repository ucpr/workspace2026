package cli

import (
	"github.com/spf13/cobra"

	"github.com/ucpr/atama/internal/service"
)

func newWorktreeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "worktree",
		Short: "Give a task its own isolated git worktree for delegated execution",
	}
	cmd.AddCommand(newWorktreeAddCmd(), newWorktreeRmCmd(), newWorktreeListCmd())
	return cmd
}

func newWorktreeAddCmd() *cobra.Command {
	var (
		branch     string
		path       string
		startPoint string
	)
	cmd := &cobra.Command{
		Use:   "add <id>",
		Short: "Create a worktree and branch for a task",
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
			task, err := svc.CreateWorktree(taskID, service.CreateWorktreeInput{
				Branch: branch, Dir: path, StartPoint: startPoint,
			})
			if err != nil {
				return err
			}
			if isJSON() {
				return printJSON(task.Execution.Worktree)
			}
			printText("%s", task.Execution.Worktree.Path)
			printText("branch: %s", task.Execution.Worktree.Branch)
			return nil
		},
	}
	cmd.Flags().StringVar(&branch, "branch", "", `branch name (default "atama/<task-id>")`)
	cmd.Flags().StringVar(&path, "path", "", "worktree path (default a sibling <repo>-worktrees directory)")
	cmd.Flags().StringVar(&startPoint, "start-point", "", `git ref to branch from (default "HEAD")`)
	return cmd
}

func newWorktreeRmCmd() *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "rm <id>",
		Short: "Remove a task's worktree",
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
			task, err := svc.RemoveWorktree(taskID, force)
			if err != nil {
				return err
			}
			if isJSON() {
				return printJSON(map[string]string{"removed_for_task": task.ID})
			}
			printText("Removed worktree for %s", task.ID)
			return nil
		},
	}
	cmd.Flags().BoolVar(&force, "force", false, "discard uncommitted changes in the worktree")
	return cmd
}

func newWorktreeListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List tasks that have an active worktree",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := loadService()
			if err != nil {
				return err
			}
			tasks, err := svc.ListTasks(service.TaskFilter{})
			if err != nil {
				return err
			}
			type row struct {
				TaskID string `json:"task_id"`
				Title  string `json:"title"`
				Path   string `json:"path"`
				Branch string `json:"branch"`
			}
			var rows []row
			for _, t := range tasks {
				if t.Execution.Worktree != nil {
					rows = append(rows, row{TaskID: t.ID, Title: t.Title, Path: t.Execution.Worktree.Path, Branch: t.Execution.Worktree.Branch})
				}
			}
			if isJSON() {
				return printJSON(rows)
			}
			if len(rows) == 0 {
				printText("No active worktrees.")
				return nil
			}
			for _, r := range rows {
				printText("%s  %s  %s  %s", r.TaskID, r.Branch, r.Path, r.Title)
			}
			return nil
		},
	}
}
