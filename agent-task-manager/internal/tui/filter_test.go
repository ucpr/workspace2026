package tui

import (
	"testing"

	"github.com/ucpr/atama/internal/model"
)

func TestApplyFilters(t *testing.T) {
	a := &model.Task{ID: "a", Title: "fix login bug", Priority: model.PriorityHigh, Labels: []string{"bug"}, GoalIDs: []string{"g1"}}
	b := &model.Task{ID: "b", Title: "write docs", Priority: model.PriorityLow, Labels: []string{"docs"}}
	tasks := []*model.Task{a, b}

	cases := []struct {
		name string
		f    filters
		want []string
	}{
		{"no filter", filters{}, []string{"a", "b"}},
		{"by goal", filters{GoalID: "g1"}, []string{"a"}},
		{"by label", filters{Label: "docs"}, []string{"b"}},
		{"by priority", filters{Priority: model.PriorityHigh}, []string{"a"}},
		{"by query matches title", filters{Query: "login"}, []string{"a"}},
		{"by query case-insensitive", filters{Query: "DOCS"}, []string{"b"}},
		{"no match", filters{Label: "nope"}, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := applyFilters(tasks, c.f)
			if len(got) != len(c.want) {
				t.Fatalf("applyFilters() = %v, want %v", ids(got), c.want)
			}
			for i, task := range got {
				if task.ID != c.want[i] {
					t.Errorf("applyFilters()[%d] = %s, want %s", i, task.ID, c.want[i])
				}
			}
		})
	}
}

func ids(tasks []*model.Task) []string {
	out := make([]string, len(tasks))
	for i, t := range tasks {
		out[i] = t.ID
	}
	return out
}
