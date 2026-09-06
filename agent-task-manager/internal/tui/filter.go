package tui

import (
	"strings"

	"github.com/ucpr/atama/internal/model"
)

// filters narrows the board's visible tasks (requirements §5.5: goal/label/
// priority filtering, title/body search).
type filters struct {
	GoalID   string
	Label    string
	Priority model.Priority
	Query    string
}

func (f filters) active() bool {
	return f.GoalID != "" || f.Label != "" || f.Priority != "" || f.Query != ""
}

func applyFilters(tasks []*model.Task, f filters) []*model.Task {
	if !f.active() {
		return tasks
	}
	out := make([]*model.Task, 0, len(tasks))
	for _, t := range tasks {
		if f.GoalID != "" && !containsString(t.GoalIDs, f.GoalID) {
			continue
		}
		if f.Label != "" && !containsString(t.Labels, f.Label) {
			continue
		}
		if f.Priority != "" && t.Priority != f.Priority {
			continue
		}
		if f.Query != "" {
			q := strings.ToLower(f.Query)
			if !strings.Contains(strings.ToLower(t.Title), q) && !strings.Contains(strings.ToLower(t.Body), q) {
				continue
			}
		}
		out = append(out, t)
	}
	return out
}

func containsString(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}
