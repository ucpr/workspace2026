package github

import (
	"fmt"

	"github.com/ucpr/atama/internal/model"
	"github.com/ucpr/atama/internal/store"
)

// stateForStatus maps a task's richer local status onto GitHub's binary
// open/closed (requirements §5.1: atama statuses are finer-grained than
// Issue state).
func stateForStatus(status model.Status) string {
	if status.Terminal() {
		return "closed"
	}
	return "open"
}

// mapIssueStateToStatus maps a remote Issue state onto a local status. It
// only crosses the open/closed <-> non-terminal/terminal boundary, leaving
// an already-consistent granular status (e.g. in_progress) untouched so
// sync doesn't clobber workflow detail GitHub has no concept of.
func mapIssueStateToStatus(state string, current model.Status) model.Status {
	switch state {
	case "closed":
		if !current.Terminal() {
			return model.StatusDone
		}
	case "open":
		if current.Terminal() {
			return model.StatusBacklog
		}
	}
	return current
}

// findTaskByIssueNumber returns the task linked to the given repo/issue
// number, or nil if none is linked yet.
func findTaskByIssueNumber(st *store.Store, repo string, number int) (*model.Task, error) {
	tasks, err := st.ListTasks()
	if err != nil {
		return nil, fmt.Errorf("github: listing tasks: %w", err)
	}
	for _, t := range tasks {
		if t.GitHub != nil && t.GitHub.Repo == repo && t.GitHub.IssueNumber == number {
			return t, nil
		}
	}
	return nil, nil
}
