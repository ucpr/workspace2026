package cli

import (
	"github.com/spf13/cobra"

	"github.com/ucpr/atama/internal/model"
	"github.com/ucpr/atama/internal/service"
)

func newGoalCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "goal",
		Short: "Manage goals",
	}
	cmd.AddCommand(newGoalAddCmd(), newGoalListCmd(), newGoalShowCmd())
	return cmd
}

func newGoalAddCmd() *cobra.Command {
	var (
		description string
		tasks       []string
	)
	cmd := &cobra.Command{
		Use:   "add <name>",
		Short: "Create a goal",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := loadService()
			if err != nil {
				return err
			}
			taskIDs, err := resolveAll(svc.ResolveTaskID, tasks)
			if err != nil {
				return err
			}
			goal, err := svc.AddGoal(service.AddGoalInput{
				Name:        args[0],
				Description: description,
				TaskIDs:     taskIDs,
			})
			if err != nil {
				return err
			}
			return outputGoal(goal)
		},
	}
	cmd.Flags().StringVar(&description, "description", "", "goal description (Markdown)")
	cmd.Flags().StringSliceVar(&tasks, "task", nil, "task ID/prefix belonging to this goal (repeatable)")
	return cmd
}

func newGoalListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List goals",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := loadService()
			if err != nil {
				return err
			}
			goals, err := svc.ListGoals()
			if err != nil {
				return err
			}
			if isJSON() {
				return printJSON(goals)
			}
			if len(goals) == 0 {
				printText("No goals.")
				return nil
			}
			for _, g := range goals {
				printText("%s  (%d tasks)  %s", g.ID, len(g.TaskIDs), g.Name)
			}
			return nil
		},
	}
}

func newGoalShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show goal details",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := loadService()
			if err != nil {
				return err
			}
			goalID, err := svc.ResolveGoalID(args[0])
			if err != nil {
				return err
			}
			goal, err := svc.GetGoal(goalID)
			if err != nil {
				return err
			}
			return outputGoal(goal)
		},
	}
}

func outputGoal(goal *model.Goal) error {
	if isJSON() {
		return printJSON(goal)
	}
	printText("%s  %s", goal.ID, goal.Name)
	if goal.Description != "" {
		printText("%s", goal.Description)
	}
	printText("tasks: %d", len(goal.TaskIDs))
	for _, id := range goal.TaskIDs {
		printText("  - %s", id)
	}
	return nil
}
