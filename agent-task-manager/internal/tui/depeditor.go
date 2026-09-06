package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ucpr/atama/internal/service"
)

// depsToggledMsg wraps reloadedMsg for updates triggered while the
// dependency editor stays open (add/remove one edge at a time).
type depsToggledMsg struct{ reloadedMsg }

func (m *Model) startDepEditor(taskID string) {
	m.depEditorTaskID = taskID
	m.depEditorReturn = m.mode
	m.depEditorCursor = 0
	m.depEditorCandidates = m.depEditorCandidates[:0]
	for _, t := range m.allTasks {
		if t.ID != taskID {
			m.depEditorCandidates = append(m.depEditorCandidates, t)
		}
	}
	m.mode = modeDepEditor
}

func (m *Model) updateDepEditor(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "esc", "q":
		m.mode = m.depEditorReturn
	case "j", "down":
		if m.depEditorCursor < len(m.depEditorCandidates)-1 {
			m.depEditorCursor++
		}
	case "k", "up":
		if m.depEditorCursor > 0 {
			m.depEditorCursor--
		}
	case "enter", " ":
		return m, m.toggleDependency()
	}
	return m, nil
}

func (m *Model) toggleDependency() tea.Cmd {
	if m.depEditorCursor < 0 || m.depEditorCursor >= len(m.depEditorCandidates) {
		return nil
	}
	self := m.depEditorTaskID
	target := m.depEditorCandidates[m.depEditorCursor].ID
	selfTask := m.tasksByID[self]
	alreadyDep := selfTask != nil && containsString(selfTask.DependsOn, target)

	return func() tea.Msg {
		var err error
		if alreadyDep {
			_, err = m.svc.RemoveDependency(self, target)
		} else {
			_, err = m.svc.AddDependency(self, target)
		}
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
		return depsToggledMsg{reloadedMsg{tasks: tasks, goals: goals}}
	}
}

func (m *Model) viewDepEditor() string {
	self := m.tasksByID[m.depEditorTaskID]
	var b strings.Builder
	title := "Edit Dependencies"
	if self != nil {
		title = fmt.Sprintf("Dependencies for %s", self.Title)
	}
	b.WriteString(titleStyle.Render(" " + title + " "))
	b.WriteString("\n\n")

	for i, t := range m.depEditorCandidates {
		checked := " "
		if self != nil && containsString(self.DependsOn, t.ID) {
			checked = "x"
		}
		line := fmt.Sprintf("[%s] %s  %s", checked, t.ID, t.Title)
		if i == m.depEditorCursor {
			b.WriteString(focusedCardStyle.Render("> " + line))
		} else {
			b.WriteString(cardStyle.Render("  " + line))
		}
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(helpHintStyle.Render("j/k: move  enter/space: toggle depends_on  esc: done"))
	return b.String()
}
