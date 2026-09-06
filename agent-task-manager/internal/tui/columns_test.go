package tui

import (
	"testing"

	"github.com/ucpr/atama/internal/model"
)

func TestNextPrevCycleStatus(t *testing.T) {
	cases := []struct {
		status   model.Status
		wantNext model.Status
		wantPrev model.Status
	}{
		{model.StatusBacklog, model.StatusReady, model.StatusBacklog},
		{model.StatusReady, model.StatusInProgress, model.StatusBacklog},
		{model.StatusCancelled, model.StatusCancelled, model.StatusDone},
	}
	for _, c := range cases {
		if got := nextCycleStatus(c.status); got != c.wantNext {
			t.Errorf("nextCycleStatus(%s) = %s, want %s", c.status, got, c.wantNext)
		}
		if got := prevCycleStatus(c.status); got != c.wantPrev {
			t.Errorf("prevCycleStatus(%s) = %s, want %s", c.status, got, c.wantPrev)
		}
	}
}

func TestBuildColumns_GroupsByDerivedStatus(t *testing.T) {
	a := &model.Task{ID: "a", Status: model.StatusBacklog}
	b := &model.Task{ID: "b", Status: model.StatusBacklog, DependsOn: []string{"a"}} // blocked: a not done
	all := map[string]*model.Task{"a": a, "b": b}

	cols := buildColumns([]*model.Task{a, b}, all, sortByPriority)

	var backlog, blocked []*model.Task
	for _, col := range cols {
		switch col.Status {
		case model.StatusBacklog:
			backlog = col.Tasks
		case model.StatusBlocked:
			blocked = col.Tasks
		}
	}
	if len(backlog) != 1 || backlog[0].ID != "a" {
		t.Errorf("backlog column = %v, want [a]", backlog)
	}
	if len(blocked) != 1 || blocked[0].ID != "b" {
		t.Errorf("blocked column = %v, want [b] (unresolved dependency)", blocked)
	}
}

func TestSortTasks(t *testing.T) {
	low := &model.Task{ID: "low", Priority: model.PriorityLow}
	urgent := &model.Task{ID: "urgent", Priority: model.PriorityUrgent}
	tasks := []*model.Task{low, urgent}

	sortTasks(tasks, sortByPriority)
	if tasks[0].ID != "urgent" {
		t.Errorf("sortByPriority: tasks[0] = %s, want urgent first", tasks[0].ID)
	}
}

func TestSortMode_Next(t *testing.T) {
	m := sortByPriority
	seen := map[sortMode]bool{}
	for i := 0; i < 3; i++ {
		seen[m] = true
		m = m.next()
	}
	if len(seen) != 3 {
		t.Errorf("sortMode.next() cycle covers %d distinct modes, want 3", len(seen))
	}
	if m != sortByPriority {
		t.Errorf("sortMode.next() after full cycle = %v, want back to sortByPriority", m)
	}
}
