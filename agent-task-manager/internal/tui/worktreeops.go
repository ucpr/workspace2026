package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ucpr/atama/internal/service"
)

// worktreeCreatedMsg wraps reloadedMsg for the post-create refresh (key W).
type worktreeCreatedMsg struct {
	reloadedMsg
	summary string
}

func (m *Model) createWorktree(taskID string) tea.Cmd {
	return func() tea.Msg {
		task, err := m.svc.CreateWorktree(taskID, service.CreateWorktreeInput{})
		if err != nil {
			return errMsg{err}
		}
		tasks, err := m.svc.ListTasks(service.TaskFilter{})
		if err != nil {
			return errMsg{err}
		}
		goals, err := m.svc.ListGoals()
		if err != nil {
			return errMsg{err}
		}
		return worktreeCreatedMsg{
			reloadedMsg: reloadedMsg{tasks: tasks, goals: goals},
			summary:     fmt.Sprintf("Worktree ready: %s", task.Execution.Worktree.Path),
		}
	}
}
