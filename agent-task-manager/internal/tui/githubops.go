package tui

import (
	"context"
	"errors"
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	ghlib "github.com/ucpr/atama/internal/github"
	"github.com/ucpr/atama/internal/service"
)

var errNoGitHubRepo = errors.New("no GitHub repo configured; run `atama init --github-repo owner/repo` or `atama github set-repo owner/repo`")

// syncPreviewMsg carries a dry-run plan (key S) awaiting user confirmation.
type syncPreviewMsg struct {
	result *ghlib.SyncResult
	repo   string
}

// syncAppliedMsg wraps reloadedMsg for the post-apply refresh.
type syncAppliedMsg struct {
	reloadedMsg
	summary string
}

func (m *Model) runSync() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		api, repo, err := m.ghAPI(ctx)
		if err != nil {
			return errMsg{err}
		}
		result, err := ghlib.Sync(ctx, m.st, api, repo, ghlib.SyncOptions{DryRun: true})
		if err != nil {
			return errMsg{err}
		}
		return syncPreviewMsg{result: result, repo: repo}
	}
}

func (m *Model) applySync() tea.Cmd {
	repo := m.pendingSyncRepo
	return func() tea.Msg {
		ctx := context.Background()
		api, _, err := m.ghAPI(ctx)
		if err != nil {
			return errMsg{err}
		}
		result, err := ghlib.Sync(ctx, m.st, api, repo, ghlib.SyncOptions{DryRun: false})
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
		return syncAppliedMsg{
			reloadedMsg: reloadedMsg{tasks: tasks, goals: goals},
			summary:     fmt.Sprintf("Synced %d task(s) with %s", len(result.Tasks), repo),
		}
	}
}

func (m *Model) runImport() tea.Cmd {
	return func() tea.Msg {
		ctx := context.Background()
		api, repo, err := m.ghAPI(ctx)
		if err != nil {
			return errMsg{err}
		}
		result, err := ghlib.Import(ctx, m.st, api, repo, ghlib.ImportOptions{})
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
		return syncAppliedMsg{
			reloadedMsg: reloadedMsg{tasks: tasks, goals: goals},
			summary:     fmt.Sprintf("Imported %d, updated %d, skipped %d", len(result.Created), len(result.Updated), len(result.Skipped)),
		}
	}
}

func (m *Model) updateConfirmSync(msg tea.Msg) (tea.Model, tea.Cmd) {
	keyMsg, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch keyMsg.String() {
	case "y", "Y":
		return m, m.applySync()
	case "n", "N", "esc", "q":
		m.mode = modeBoard
	}
	return m, nil
}

func (m *Model) viewConfirmSync() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf(" Sync with %s — preview ", m.pendingSyncRepo)))
	b.WriteString("\n\n")
	if m.pendingSyncResult == nil || len(m.pendingSyncResult.Tasks) == 0 {
		b.WriteString(dimStyle.Render("No linked tasks to sync.\n"))
	}
	for _, tp := range m.pendingSyncResult.Tasks {
		b.WriteString(fmt.Sprintf("%s  issue #%d  %s\n", tp.TaskID, tp.IssueNumber, tp.Action))
		for _, cp := range tp.Comments {
			if cp.Action == "none" {
				continue
			}
			b.WriteString(fmt.Sprintf("    comment %s  %s\n", cp.CommentID, cp.Action))
		}
	}
	b.WriteString("\n")
	b.WriteString(helpHintStyle.Render("y: apply  n/esc: cancel"))
	return b.String()
}
