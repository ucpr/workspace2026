package cli

import (
	"github.com/spf13/cobra"
)

func newDepCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "dep",
		Short: "Manage task dependencies",
	}
	cmd.AddCommand(newDepAddCmd(), newDepRmCmd())
	return cmd
}

func newDepAddCmd() *cobra.Command {
	var on string
	cmd := &cobra.Command{
		Use:   "add <id>",
		Short: "Add a dependency: <id> depends on --on <id>",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := loadService()
			if err != nil {
				return err
			}
			fromID, err := svc.ResolveTaskID(args[0])
			if err != nil {
				return err
			}
			onID, err := svc.ResolveTaskID(on)
			if err != nil {
				return err
			}
			task, err := svc.AddDependency(fromID, onID)
			if err != nil {
				return err
			}
			return outputTask(svc, task)
		},
	}
	cmd.Flags().StringVar(&on, "on", "", "task ID/prefix that <id> depends on (required)")
	_ = cmd.MarkFlagRequired("on")
	return cmd
}

func newDepRmCmd() *cobra.Command {
	var on string
	cmd := &cobra.Command{
		Use:   "rm <id>",
		Short: "Remove a dependency: <id> no longer depends on --on <id>",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := loadService()
			if err != nil {
				return err
			}
			fromID, err := svc.ResolveTaskID(args[0])
			if err != nil {
				return err
			}
			onID, err := svc.ResolveTaskID(on)
			if err != nil {
				return err
			}
			task, err := svc.RemoveDependency(fromID, onID)
			if err != nil {
				return err
			}
			return outputTask(svc, task)
		},
	}
	cmd.Flags().StringVar(&on, "on", "", "task ID/prefix to stop depending on (required)")
	_ = cmd.MarkFlagRequired("on")
	return cmd
}
