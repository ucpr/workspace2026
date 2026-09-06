package tui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/glamour"

	"github.com/ucpr/atama/internal/depgraph"
	"github.com/ucpr/atama/internal/model"
)

var detailTabs = []string{"body", "comments", "dependencies"}

func (m *Model) updateDetail(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch {
	case key.Matches(keyMsg, keys.Esc) || keyMsg.String() == "q":
		m.mode = modeBoard
	case key.Matches(keyMsg, keys.Tab):
		m.detailTab = (m.detailTab + 1) % len(detailTabs)
	case key.Matches(keyMsg, keys.Edit):
		if t := m.tasksByID[m.detailTaskID]; t != nil {
			m.startEditTaskForm(t)
		}
	case key.Matches(keyMsg, keys.DepEditor):
		m.startDepEditor(m.detailTaskID)
	case key.Matches(keyMsg, keys.Comment):
		m.commentInput.SetValue("")
		m.commentInput.Focus()
		m.mode = modeCommentInput
	case key.Matches(keyMsg, keys.Delete):
		m.confirmTaskID = m.detailTaskID
		m.mode = modeConfirmDelete
	}
	return m, nil
}

func (m *Model) viewDetail() string {
	t := m.tasksByID[m.detailTaskID]
	if t == nil {
		return "task not found\n"
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf(" %s ", t.Title)))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render(fmt.Sprintf("%s  status=%s  priority=%s", t.ID, depgraph.DerivedStatus(t, m.tasksByID), t.Priority)))
	b.WriteString("\n\n")

	var tabLine strings.Builder
	for i, name := range detailTabs {
		if i == m.detailTab {
			tabLine.WriteString(titleStyle.Render("[" + name + "]"))
		} else {
			tabLine.WriteString(dimStyle.Render(" " + name + " "))
		}
		tabLine.WriteString("  ")
	}
	b.WriteString(tabLine.String())
	b.WriteString("\n\n")

	switch m.detailTab {
	case 0:
		b.WriteString(renderMarkdown(t.Body))
	case 1:
		b.WriteString(renderComments(t))
	case 2:
		b.WriteString(m.renderDependencies(t))
	}

	b.WriteString("\n")
	b.WriteString(helpHintStyle.Render("tab: switch  e: edit  D: dependencies  c: comment  d: delete  esc: back"))
	return b.String()
}

func renderMarkdown(body string) string {
	if body == "" {
		return dimStyle.Render("(no body)")
	}
	out, err := glamour.Render(body, "dark")
	if err != nil {
		return body
	}
	return strings.TrimRight(out, "\n")
}

func renderComments(t *model.Task) string {
	if len(t.Comments) == 0 {
		return dimStyle.Render("(no comments)")
	}
	var b strings.Builder
	for _, c := range t.Comments {
		if c.DeletedAt != nil {
			continue
		}
		b.WriteString(titleStyle.Render(c.Author))
		b.WriteString(dimStyle.Render("  " + c.UpdatedAt.Format("2006-01-02 15:04")))
		b.WriteString("\n")
		b.WriteString(c.Body)
		b.WriteString("\n\n")
	}
	return strings.TrimRight(b.String(), "\n")
}

func (m *Model) renderDependencies(t *model.Task) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render("Depends on:"))
	b.WriteString("\n")
	if len(t.DependsOn) == 0 {
		b.WriteString(dimStyle.Render("  (none)\n"))
	}
	for _, id := range t.DependsOn {
		b.WriteString(m.describeRef(id))
	}

	b.WriteString("\n")
	b.WriteString(titleStyle.Render("Blocks:"))
	b.WriteString("\n")
	found := false
	for _, other := range m.allTasks {
		if containsString(other.DependsOn, t.ID) {
			b.WriteString(m.describeRef(other.ID))
			found = true
		}
	}
	if !found {
		b.WriteString(dimStyle.Render("  (none)\n"))
	}
	return b.String()
}

func (m *Model) describeRef(id string) string {
	other := m.tasksByID[id]
	if other == nil {
		return dimStyle.Render("  " + id + " (missing)\n")
	}
	status := depgraph.DerivedStatus(other, m.tasksByID)
	return fmt.Sprintf("  %s [%s] %s\n", id, status, other.Title)
}
