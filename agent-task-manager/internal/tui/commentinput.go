package tui

import (
	tea "github.com/charmbracelet/bubbletea"

	"github.com/ucpr/atama/internal/service"
)

// commentSubmittedMsg wraps reloadedMsg for the post-comment refresh.
type commentSubmittedMsg struct{ reloadedMsg }

func (m *Model) updateCommentInput(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc":
			m.commentInput.Blur()
			m.mode = modeDetail
			return m, nil
		case "ctrl+s":
			return m, m.submitComment()
		}
	}
	var cmd tea.Cmd
	m.commentInput, cmd = m.commentInput.Update(msg)
	return m, cmd
}

func (m *Model) submitComment() tea.Cmd {
	taskID := m.detailTaskID
	body := m.commentInput.Value()
	return func() tea.Msg {
		if _, err := m.svc.AddComment(taskID, "you", body); err != nil {
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
		return commentSubmittedMsg{reloadedMsg{tasks: tasks, goals: goals}}
	}
}

func (m *Model) viewCommentInput() string {
	return titleStyle.Render(" Add Comment ") + "\n\n" + m.commentInput.View() + "\n\n" +
		helpHintStyle.Render("ctrl+s: submit  esc: cancel")
}
