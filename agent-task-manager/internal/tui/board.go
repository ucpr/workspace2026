package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/ucpr/atama/internal/depgraph"
	"github.com/ucpr/atama/internal/model"
	"github.com/ucpr/atama/internal/service"
)

func (m *Model) updateBoard(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch {
	case key.Matches(keyMsg, keys.Quit):
		return m, tea.Quit
	case key.Matches(keyMsg, keys.Left):
		if m.focusCol > 0 {
			m.focusCol--
			m.clampFocusRow()
		}
	case key.Matches(keyMsg, keys.Right):
		if m.focusCol < len(m.columns)-1 {
			m.focusCol++
			m.clampFocusRow()
		}
	case key.Matches(keyMsg, keys.Up):
		if m.focusRow > 0 {
			m.focusRow--
		}
	case key.Matches(keyMsg, keys.Down):
		if len(m.columns) > 0 && m.focusRow < len(m.columns[m.focusCol].Tasks)-1 {
			m.focusRow++
		}
	case key.Matches(keyMsg, keys.Enter):
		if t := m.focusedTask(); t != nil {
			m.detailTaskID = t.ID
			m.detailTab = 0
			m.mode = modeDetail
		}
	case key.Matches(keyMsg, keys.Space):
		if t := m.focusedTask(); t != nil {
			return m, m.setStatusCmd(t.ID, nextCycleStatus(t.Status))
		}
	case key.Matches(keyMsg, keys.JumpRight):
		if t := m.focusedTask(); t != nil {
			return m, m.setStatusCmd(t.ID, nextCycleStatus(t.Status))
		}
	case key.Matches(keyMsg, keys.JumpLeft):
		if t := m.focusedTask(); t != nil {
			return m, m.setStatusCmd(t.ID, prevCycleStatus(t.Status))
		}
	case key.Matches(keyMsg, keys.New):
		m.startNewTaskForm()
	case key.Matches(keyMsg, keys.Edit):
		if t := m.focusedTask(); t != nil {
			m.startEditTaskForm(t)
		}
	case key.Matches(keyMsg, keys.Delete):
		if t := m.focusedTask(); t != nil {
			m.confirmTaskID = t.ID
			m.mode = modeConfirmDelete
		}
	case key.Matches(keyMsg, keys.DepEditor):
		if t := m.focusedTask(); t != nil {
			m.startDepEditor(t.ID)
		}
	case key.Matches(keyMsg, keys.Graph):
		if t := m.focusedTask(); t != nil {
			m.detailTaskID = t.ID
			m.mode = modeGraph
		}
	case key.Matches(keyMsg, keys.Worktree):
		if t := m.focusedTask(); t != nil {
			return m, m.createWorktree(t.ID)
		}
	case key.Matches(keyMsg, keys.GoalFilter):
		m.goalPickerCursor = 0
		m.mode = modeGoalPicker
	case key.Matches(keyMsg, keys.FilterMenu):
		m.filterLabelInput.SetValue(m.filters.Label)
		m.filterLabelInput.Focus()
		m.filterMenuField = 0
		m.mode = modeFilterMenu
	case key.Matches(keyMsg, keys.Sort):
		m.sortMode = m.sortMode.next()
		m.rebuildColumns()
	case key.Matches(keyMsg, keys.Search):
		m.searchInput.SetValue(m.filters.Query)
		m.searchInput.Focus()
		m.mode = modeSearch
	case key.Matches(keyMsg, keys.Sync):
		return m, m.runSync()
	case key.Matches(keyMsg, keys.Import):
		return m, m.runImport()
	case key.Matches(keyMsg, keys.Reload):
		return m, m.reload()
	case key.Matches(keyMsg, keys.Help):
		m.mode = modeHelp
	case key.Matches(keyMsg, keys.Esc):
		if m.filters.active() {
			m.filters = filters{}
			m.rebuildColumns()
		}
	}
	return m, nil
}

// setStatusCmd changes a task's status via the service and reloads the
// board so every column reflects the new derived state.
func (m *Model) setStatusCmd(taskID string, status model.Status) tea.Cmd {
	return func() tea.Msg {
		if _, err := m.svc.SetStatus(taskID, status); err != nil {
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
		return reloadedMsg{tasks: tasks, goals: goals}
	}
}

func (m *Model) viewBoard() string {
	if len(m.columns) == 0 {
		return "loading...\n"
	}
	rendered := make([]string, len(m.columns))
	for i, col := range m.columns {
		rendered[i] = m.renderColumn(col, i == m.focusCol)
	}
	board := lipgloss.JoinHorizontal(lipgloss.Top, rendered...)

	var b strings.Builder
	b.WriteString(titleStyle.Render(" atama board "))
	b.WriteString("\n")
	b.WriteString(board)
	b.WriteString("\n")
	b.WriteString(m.statusLine())
	return b.String()
}

func (m *Model) renderColumn(col column, focused bool) string {
	header := fmt.Sprintf("%s (%d)", col.Status, len(col.Tasks))
	var body strings.Builder
	body.WriteString(columnHeaderStyle.Render(header))
	body.WriteString("\n")
	for i, t := range col.Tasks {
		isFocused := focused && i == m.focusRow
		body.WriteString(m.renderCard(t, isFocused))
		body.WriteString("\n")
	}
	content := columnStyle.Render(body.String())
	if focused {
		return focusedColumnBorder.Render(content)
	}
	return plainColumnBorder.Render(content)
}

func (m *Model) renderCard(t *model.Task, focused bool) string {
	title := t.Title
	maxLen := colWidth - 6
	if maxLen > 0 && len(title) > maxLen {
		title = title[:maxLen-1] + "…"
	}

	var badge string
	if depgraph.IsReady(t, m.tasksByID) {
		badge = readyBadge.Render("●")
	} else if len(t.DependsOn) > 0 {
		badge = blockedBadge.Render("●")
	} else {
		badge = " "
	}

	line := fmt.Sprintf("%s %s", badge, priorityStyle(t.Priority).Render(title))
	if focused {
		return focusedCardStyle.Render("> " + line)
	}
	return cardStyle.Render("  " + line)
}

func (m *Model) statusLine() string {
	var parts []string
	if m.filters.active() {
		parts = append(parts, "filter: "+m.filters.summary())
	}
	parts = append(parts, "sort: "+m.sortMode.String())
	hint := strings.Join(parts, "  ") + "  press ? for help"
	if m.err != nil {
		return errStyle.Render("error: "+m.err.Error()) + "  " + helpHintStyle.Render(hint)
	}
	if m.statusBar != "" {
		return statusBarStyle.Render(m.statusBar) + "  " + helpHintStyle.Render(hint)
	}
	return helpHintStyle.Render(hint)
}

func (f filters) summary() string {
	var parts []string
	if f.GoalID != "" {
		parts = append(parts, "goal="+f.GoalID)
	}
	if f.Label != "" {
		parts = append(parts, "label="+f.Label)
	}
	if f.Priority != "" {
		parts = append(parts, "priority="+string(f.Priority))
	}
	if f.Query != "" {
		parts = append(parts, "q=\""+f.Query+"\"")
	}
	return strings.Join(parts, ",")
}
