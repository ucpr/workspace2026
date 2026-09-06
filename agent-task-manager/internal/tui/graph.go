package tui

import (
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
)

// updateGraph and viewGraph implement key `g`: a textual dependency graph
// for the focused task (what it depends on and what depends on it), since
// a full graph-drawing renderer is out of scope for v0.1's terminal UI.
func (m *Model) updateGraph(msg tea.Msg) (tea.Model, tea.Cmd) {
	if keyMsg, ok := msg.(tea.KeyMsg); ok {
		switch keyMsg.String() {
		case "esc", "q":
			m.mode = modeBoard
		}
	}
	return m, nil
}

func (m *Model) viewGraph() string {
	t := m.tasksByID[m.detailTaskID]
	if t == nil {
		return "task not found\n"
	}
	title := fmt.Sprintf(" Dependency graph: %s ", t.Title)
	return titleStyle.Render(title) + "\n\n" + m.renderDependencies(t) + "\n" +
		helpHintStyle.Render("esc: back")
}
