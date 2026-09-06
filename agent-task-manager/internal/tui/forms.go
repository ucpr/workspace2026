package tui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/ucpr/atama/internal/model"
	"github.com/ucpr/atama/internal/service"
)

const (
	formFieldTitle = iota
	formFieldBody
	formFieldPriority
	formFieldLabels
	formFieldAssignees
	formFieldCount
)

func (m *Model) startNewTaskForm() {
	m.formEditingID = ""
	m.formTitle.SetValue("")
	m.formBody.SetValue("")
	m.formLabels.SetValue("")
	m.formAssignees.SetValue("")
	m.formPriority = model.PriorityMedium
	m.formFocusIdx = formFieldTitle
	m.formTitle.Focus()
	m.mode = modeForm
}

func (m *Model) startEditTaskForm(t *model.Task) {
	m.formEditingID = t.ID
	m.formTitle.SetValue(t.Title)
	m.formBody.SetValue(t.Body)
	m.formLabels.SetValue(strings.Join(t.Labels, ","))
	m.formAssignees.SetValue(strings.Join(t.Assignees, ","))
	m.formPriority = t.Priority
	m.formFocusIdx = formFieldTitle
	m.formTitle.Focus()
	m.mode = modeForm
}

func splitCSV(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(s, ",") {
		part = strings.TrimSpace(part)
		if part != "" {
			out = append(out, part)
		}
	}
	return out
}

func (m *Model) updateForm(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}

	switch keyMsg.String() {
	case "esc":
		m.mode = modeBoard
		m.blurForm()
		return m, nil
	case "tab", "shift+tab":
		m.blurForm()
		if keyMsg.String() == "tab" {
			m.formFocusIdx = (m.formFocusIdx + 1) % formFieldCount
		} else {
			m.formFocusIdx = (m.formFocusIdx - 1 + formFieldCount) % formFieldCount
		}
		m.focusForm()
		return m, nil
	case "ctrl+p":
		if m.formFocusIdx == formFieldPriority {
			m.formPriority = cyclePriority(m.formPriority)
		}
		return m, nil
	case "enter":
		if m.formFocusIdx == formFieldBody {
			break // let textarea insert a newline
		}
		return m, m.submitForm()
	}

	var cmd tea.Cmd
	switch m.formFocusIdx {
	case formFieldTitle:
		m.formTitle, cmd = m.formTitle.Update(msg)
	case formFieldBody:
		m.formBody, cmd = m.formBody.Update(msg)
	case formFieldPriority:
		if keyMsg.String() == "left" || keyMsg.String() == "h" {
			m.formPriority = cyclePriorityBack(m.formPriority)
		} else if keyMsg.String() == "right" || keyMsg.String() == "l" {
			m.formPriority = cyclePriority(m.formPriority)
		}
	case formFieldLabels:
		m.formLabels, cmd = m.formLabels.Update(msg)
	case formFieldAssignees:
		m.formAssignees, cmd = m.formAssignees.Update(msg)
	}
	return m, cmd
}

func cyclePriority(p model.Priority) model.Priority {
	order := []model.Priority{model.PriorityLow, model.PriorityMedium, model.PriorityHigh, model.PriorityUrgent}
	for i, x := range order {
		if x == p {
			return order[(i+1)%len(order)]
		}
	}
	return model.PriorityMedium
}

func cyclePriorityBack(p model.Priority) model.Priority {
	order := []model.Priority{model.PriorityLow, model.PriorityMedium, model.PriorityHigh, model.PriorityUrgent}
	for i, x := range order {
		if x == p {
			return order[(i-1+len(order))%len(order)]
		}
	}
	return model.PriorityMedium
}

func (m *Model) blurForm() {
	m.formTitle.Blur()
	m.formBody.Blur()
	m.formLabels.Blur()
	m.formAssignees.Blur()
}

func (m *Model) focusForm() {
	switch m.formFocusIdx {
	case formFieldTitle:
		m.formTitle.Focus()
	case formFieldBody:
		m.formBody.Focus()
	case formFieldLabels:
		m.formLabels.Focus()
	case formFieldAssignees:
		m.formAssignees.Focus()
	}
}

func (m *Model) submitForm() tea.Cmd {
	title := strings.TrimSpace(m.formTitle.Value())
	body := m.formBody.Value()
	priority := m.formPriority
	labels := splitCSV(m.formLabels.Value())
	assignees := splitCSV(m.formAssignees.Value())
	editingID := m.formEditingID

	return func() tea.Msg {
		if editingID == "" {
			if _, err := m.svc.AddTask(service.AddTaskInput{
				Title: title, Body: body, Priority: priority, Labels: labels, Assignees: assignees,
			}); err != nil {
				return errMsg{err}
			}
		} else {
			if _, err := m.svc.EditTask(editingID, service.EditTaskInput{
				Title: &title, Body: &body, Priority: &priority, Labels: &labels, Assignees: &assignees,
			}); err != nil {
				return errMsg{err}
			}
		}
		tasks, err := m.svc.ListTasks(service.TaskFilter{})
		if err != nil {
			return errMsg{err}
		}
		goals, err := m.svc.ListGoals()
		if err != nil {
			return errMsg{err}
		}
		return formSubmittedMsg{reloadedMsg{tasks: tasks, goals: goals}}
	}
}

// formSubmittedMsg wraps reloadedMsg so Update can also return the form to
// board mode once the reload lands.
type formSubmittedMsg struct{ reloadedMsg }

func (m *Model) viewForm() string {
	var b strings.Builder
	if m.formEditingID == "" {
		b.WriteString(titleStyle.Render(" New Task "))
	} else {
		b.WriteString(titleStyle.Render(" Edit Task "))
	}
	b.WriteString("\n\n")
	b.WriteString(formLabel("Title", m.formFocusIdx == formFieldTitle))
	b.WriteString(m.formTitle.View())
	b.WriteString("\n\n")
	b.WriteString(formLabel("Body", m.formFocusIdx == formFieldBody))
	b.WriteString(m.formBody.View())
	b.WriteString("\n\n")
	b.WriteString(formLabel("Priority", m.formFocusIdx == formFieldPriority))
	b.WriteString(priorityStyle(m.formPriority).Render(string(m.formPriority)))
	b.WriteString(dimStyle.Render("  (h/l to change)"))
	b.WriteString("\n\n")
	b.WriteString(formLabel("Labels", m.formFocusIdx == formFieldLabels))
	b.WriteString(m.formLabels.View())
	b.WriteString("\n\n")
	b.WriteString(formLabel("Assignees", m.formFocusIdx == formFieldAssignees))
	b.WriteString(m.formAssignees.View())
	b.WriteString("\n\n")
	b.WriteString(helpHintStyle.Render("tab/shift+tab: next/prev field  enter: submit  esc: cancel"))
	return b.String()
}

func formLabel(name string, focused bool) string {
	if focused {
		return titleStyle.Render(name+":") + "\n"
	}
	return dimStyle.Render(name+":") + "\n"
}
