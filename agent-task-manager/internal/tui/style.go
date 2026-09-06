package tui

import (
	"github.com/charmbracelet/lipgloss"

	"github.com/ucpr/atama/internal/model"
)

var (
	colWidth = 28

	columnHeaderStyle   = lipgloss.NewStyle().Bold(true).Padding(0, 1)
	columnStyle         = lipgloss.NewStyle().Padding(0, 1).Width(colWidth)
	focusedColumnBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("62"))
	plainColumnBorder = lipgloss.NewStyle().
				Border(lipgloss.RoundedBorder()).
				BorderForeground(lipgloss.Color("240"))

	cardStyle        = lipgloss.NewStyle().Padding(0, 1)
	focusedCardStyle = cardStyle.Copy().Background(lipgloss.Color("236")).Bold(true)

	statusBarStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Padding(0, 1)
	errStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	helpHintStyle  = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	titleStyle     = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	dimStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("243"))
	blockedBadge   = lipgloss.NewStyle().Foreground(lipgloss.Color("208")).Bold(true)
	readyBadge     = lipgloss.NewStyle().Foreground(lipgloss.Color("42")).Bold(true)
)

func priorityStyle(p model.Priority) lipgloss.Style {
	switch p {
	case model.PriorityUrgent:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("196")).Bold(true)
	case model.PriorityHigh:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("208"))
	case model.PriorityLow:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("245"))
	default:
		return lipgloss.NewStyle().Foreground(lipgloss.Color("39"))
	}
}
