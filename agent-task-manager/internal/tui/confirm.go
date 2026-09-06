package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ucpr/atama/internal/service"
)

// deletedMsg wraps reloadedMsg for the post-delete refresh.
type deletedMsg struct{ reloadedMsg }

func (m *Model) updateConfirmDelete(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "y", "Y":
		return m, m.deleteTask()
	case "n", "N", "esc", "q":
		m.mode = modeBoard
	}
	return m, nil
}

func (m *Model) deleteTask() tea.Cmd {
	taskID := m.confirmTaskID
	return func() tea.Msg {
		if err := m.svc.RemoveTask(taskID); err != nil {
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
		return deletedMsg{reloadedMsg{tasks: tasks, goals: goals}}
	}
}

func (m *Model) viewConfirmDelete() string {
	t := m.tasksByID[m.confirmTaskID]
	title := m.confirmTaskID
	if t != nil {
		title = t.Title
	}
	return titleStyle.Render(" Delete task? ") + "\n\n" +
		fmt.Sprintf("%s\n\n", title) +
		helpHintStyle.Render("y: delete   n/esc: cancel")
}
