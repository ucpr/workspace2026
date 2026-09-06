package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/ucpr/atama/internal/model"
	"github.com/ucpr/atama/internal/service"
)

func newTaskCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "task",
		Short: "Manage tasks",
	}
	cmd.AddCommand(
		newTaskAddCmd(),
		newTaskListCmd(),
		newTaskShowCmd(),
		newTaskEditCmd(),
		newTaskRmCmd(),
		newTaskStatusCmd(),
	)
	return cmd
}

func newTaskAddCmd() *cobra.Command {
	var (
		body      string
		priority  string
		status    string
		labels    []string
		assignees []string
		dependsOn []string
		goals     []string
	)
	cmd := &cobra.Command{
		Use:   "add <title>",
		Short: "Create a task",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := loadService()
			if err != nil {
				return err
			}
			depIDs, err := resolveAll(svc.ResolveTaskID, dependsOn)
			if err != nil {
				return err
			}
			goalIDs, err := resolveAll(svc.ResolveGoalID, goals)
			if err != nil {
				return err
			}
			task, err := svc.AddTask(service.AddTaskInput{
				Title:     args[0],
				Body:      body,
				Priority:  model.Priority(priority),
				Status:    model.Status(status),
				Labels:    labels,
				Assignees: assignees,
				DependsOn: depIDs,
				GoalIDs:   goalIDs,
			})
			if err != nil {
				return err
			}
			return outputTask(svc, task)
		},
	}
	cmd.Flags().StringVar(&body, "body", "", "task body (Markdown)")
	cmd.Flags().StringVar(&priority, "priority", "", "priority: low, medium, high, urgent (default medium)")
	cmd.Flags().StringVar(&status, "status", "", "initial status (default backlog)")
	cmd.Flags().StringSliceVar(&labels, "label", nil, "label (repeatable)")
	cmd.Flags().StringSliceVar(&assignees, "assignee", nil, "assignee (repeatable)")
	cmd.Flags().StringSliceVar(&dependsOn, "depends-on", nil, "task ID/prefix this task depends on (repeatable)")
	cmd.Flags().StringSliceVar(&goals, "goal", nil, "goal ID/prefix this task belongs to (repeatable)")
	return cmd
}

func newTaskListCmd() *cobra.Command {
	var (
		status string
		label  string
		goal   string
	)
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List tasks",
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
			tasks, err := svc.ListTasks(service.TaskFilter{
				Status: model.Status(status),
				Label:  label,
				GoalID: goalID,
			})
			if err != nil {
				return err
			}
			all, err := svc.TasksMap()
			if err != nil {
				return err
			}
			views := newTaskViews(tasks, all)
			if isJSON() {
				return printJSON(views)
			}
			printTaskTable(views)
			return nil
		},
	}
	cmd.Flags().StringVar(&status, "status", "", "filter by status")
	cmd.Flags().StringVar(&label, "label", "", "filter by label")
	cmd.Flags().StringVar(&goal, "goal", "", "filter by goal ID/prefix")
	return cmd
}

func newTaskShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "show <id>",
		Short: "Show task details",
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
			task, err := svc.GetTask(taskID)
			if err != nil {
				return err
			}
			return outputTask(svc, task)
		},
	}
}

func newTaskEditCmd() *cobra.Command {
	var (
		title     string
		body      string
		priority  string
		labels    []string
		assignees []string
	)
	cmd := &cobra.Command{
		Use:   "edit <id>",
		Short: "Edit a task",
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
			in := service.EditTaskInput{}
			if cmd.Flags().Changed("title") {
				in.Title = &title
			}
			if cmd.Flags().Changed("body") {
				in.Body = &body
			}
			if cmd.Flags().Changed("priority") {
				p := model.Priority(priority)
				in.Priority = &p
			}
			if cmd.Flags().Changed("label") {
				in.Labels = &labels
			}
			if cmd.Flags().Changed("assignee") {
				in.Assignees = &assignees
			}
			task, err := svc.EditTask(taskID, in)
			if err != nil {
				return err
			}
			return outputTask(svc, task)
		},
	}
	cmd.Flags().StringVar(&title, "title", "", "new title")
	cmd.Flags().StringVar(&body, "body", "", "new body (Markdown)")
	cmd.Flags().StringVar(&priority, "priority", "", "new priority")
	cmd.Flags().StringSliceVar(&labels, "label", nil, "replace labels (repeatable)")
	cmd.Flags().StringSliceVar(&assignees, "assignee", nil, "replace assignees (repeatable)")
	return cmd
}

func newTaskRmCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rm <id>",
		Short: "Delete a task",
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
			if err := svc.RemoveTask(taskID); err != nil {
				return err
			}
			if isJSON() {
				return printJSON(map[string]string{"removed": taskID})
			}
			printText("Removed task %s", taskID)
			return nil
		},
	}
	return cmd
}

func newTaskStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status <id> <status>",
		Short: "Change a task's status",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := loadService()
			if err != nil {
				return err
			}
			taskID, err := svc.ResolveTaskID(args[0])
			if err != nil {
				return err
			}
			task, err := svc.SetStatus(taskID, model.Status(args[1]))
			if err != nil {
				return err
			}
			return outputTask(svc, task)
		},
	}
}

func outputTask(svc *service.Service, task *model.Task) error {
	all, err := svc.TasksMap()
	if err != nil {
		return err
	}
	view := newTaskView(task, all)
	if isJSON() {
		return printJSON(view)
	}
	printText("%s  [%s/%s]  %s", view.ID, view.EffectiveStatus, view.Priority, view.Title)
	return nil
}

func printTaskTable(views []taskView) {
	if len(views) == 0 {
		printText("No tasks.")
		return
	}
	for _, v := range views {
		ready := ""
		if v.Ready {
			ready = "*"
		}
		printText("%-1s %s  %-11s  %-6s  %s", ready, v.ID, v.EffectiveStatus, v.Priority, v.Title)
	}
}

// resolveAll resolves each ref via resolver, e.g. svc.ResolveTaskID for a
// list of --depends-on/--goal flag values.
func resolveAll(resolver func(string) (string, error), refs []string) ([]string, error) {
	if len(refs) == 0 {
		return nil, nil
	}
	out := make([]string, 0, len(refs))
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		id, err := resolver(ref)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", ref, err)
		}
		out = append(out, id)
	}
	return out, nil
}
