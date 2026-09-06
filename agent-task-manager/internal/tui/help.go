package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func (m *Model) updateHelp(msg tea.Msg) (tea.Model, tea.Cmd) {
	if _, ok := msg.(tea.KeyMsg); ok {
		m.mode = modeBoard
	}
	return m, nil
}

func (m *Model) viewHelp() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(" Help "))
	b.WriteString("\n\n")
	for _, line := range helpLines() {
		b.WriteString(line)
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(helpHintStyle.Render("press any key to close"))
	return b.String()
}
