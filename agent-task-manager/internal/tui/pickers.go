package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

// --- Goal filter picker (key G) ---

func (m *Model) updateGoalPicker(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	n := len(m.goals) + 1 // +1 for "All goals"
	switch keyMsg.String() {
	case "esc", "q":
		m.mode = modeBoard
	case "j", "down":
		if m.goalPickerCursor < n-1 {
			m.goalPickerCursor++
		}
	case "k", "up":
		if m.goalPickerCursor > 0 {
			m.goalPickerCursor--
		}
	case "enter":
		if m.goalPickerCursor == 0 {
			m.filters.GoalID = ""
		} else {
			m.filters.GoalID = m.goals[m.goalPickerCursor-1].ID
		}
		m.rebuildColumns()
		m.mode = modeBoard
	}
	return m, nil
}

func (m *Model) viewGoalPicker() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(" Filter by Goal "))
	b.WriteString("\n\n")

	renderRow := func(idx int, line string) {
		if idx == m.goalPickerCursor {
			b.WriteString(focusedCardStyle.Render("> " + line))
		} else {
			b.WriteString(cardStyle.Render("  " + line))
		}
		b.WriteString("\n")
	}
	renderRow(0, "(all goals)")
	for i, g := range m.goals {
		renderRow(i+1, fmt.Sprintf("%s  %s (%d tasks)", g.ID, g.Name, len(g.TaskIDs)))
	}
	b.WriteString("\n")
	b.WriteString(helpHintStyle.Render("j/k: move  enter: apply  esc: cancel"))
	return b.String()
}

// --- Label/priority filter menu (key f) ---

func (m *Model) updateFilterMenu(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "esc":
		m.filterLabelInput.Blur()
		m.mode = modeBoard
		return m, nil
	case "tab":
		m.filterMenuField = (m.filterMenuField + 1) % 2
		if m.filterMenuField == 0 {
			m.filterLabelInput.Focus()
		} else {
			m.filterLabelInput.Blur()
		}
		return m, nil
	case "enter":
		m.filters.Label = strings.TrimSpace(m.filterLabelInput.Value())
		m.filters.Priority = priorityChoices[m.filterPriorityIdx]
		m.rebuildColumns()
		m.filterLabelInput.Blur()
		m.mode = modeBoard
		return m, nil
	}

	if m.filterMenuField == 1 {
		switch keyMsg.String() {
		case "h", "left":
			m.filterPriorityIdx = (m.filterPriorityIdx - 1 + len(priorityChoices)) % len(priorityChoices)
		case "l", "right":
			m.filterPriorityIdx = (m.filterPriorityIdx + 1) % len(priorityChoices)
		}
		return m, nil
	}

	var cmd tea.Cmd
	m.filterLabelInput, cmd = m.filterLabelInput.Update(msg)
	return m, cmd
}

func (m *Model) viewFilterMenu() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(" Filter "))
	b.WriteString("\n\n")
	b.WriteString(formLabel("Label", m.filterMenuField == 0))
	b.WriteString(m.filterLabelInput.View())
	b.WriteString("\n\n")
	b.WriteString(formLabel("Priority", m.filterMenuField == 1))
	p := priorityChoices[m.filterPriorityIdx]
	if p == "" {
		b.WriteString(dimStyle.Render("(any)"))
	} else {
		b.WriteString(priorityStyle(p).Render(string(p)))
	}
	b.WriteString(dimStyle.Render("  (h/l to change)"))
	b.WriteString("\n\n")
	b.WriteString(helpHintStyle.Render("tab: switch field  enter: apply  esc: cancel"))
	return b.String()
}

// --- Search (key /) ---

func (m *Model) updateSearch(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "esc":
		m.searchInput.Blur()
		m.mode = modeBoard
		return m, nil
	case "enter":
		m.filters.Query = strings.TrimSpace(m.searchInput.Value())
		m.rebuildColumns()
		m.searchInput.Blur()
		m.mode = modeBoard
		return m, nil
	}
	var cmd tea.Cmd
	m.searchInput, cmd = m.searchInput.Update(msg)
	return m, cmd
}

func (m *Model) viewSearch() string {
	return titleStyle.Render(" Search ") + "\n\n" + m.searchInput.View() + "\n\n" +
		helpHintStyle.Render("enter: apply  esc: cancel")
}
