package tui

import (
	"sort"

	"github.com/ucpr/atama/internal/depgraph"
	"github.com/ucpr/atama/internal/model"
)

// columnOrder lists the kanban columns left to right (requirements §5.5).
// StatusBlocked is included as a column, but it is never a manual
// destination — a task lands there only when depgraph.DerivedStatus
// computes it, keeping the manual/derived distinction from requirements
// §5.1. cycleOrder is the subset Space/Shift+H/L cycle through.
var columnOrder = []model.Status{
	model.StatusBacklog,
	model.StatusReady,
	model.StatusBlocked,
	model.StatusInProgress,
	model.StatusInReview,
	model.StatusDone,
	model.StatusCancelled,
}

var cycleOrder = []model.Status{
	model.StatusBacklog,
	model.StatusReady,
	model.StatusInProgress,
	model.StatusInReview,
	model.StatusDone,
	model.StatusCancelled,
}

func cycleIndex(status model.Status) int {
	for i, s := range cycleOrder {
		if s == status {
			return i
		}
	}
	return 0
}

// nextCycleStatus returns the manual status one step to the right of
// status, clamped at the end.
func nextCycleStatus(status model.Status) model.Status {
	i := cycleIndex(status)
	if i+1 < len(cycleOrder) {
		return cycleOrder[i+1]
	}
	return cycleOrder[i]
}

// prevCycleStatus returns the manual status one step to the left of
// status, clamped at the start.
func prevCycleStatus(status model.Status) model.Status {
	i := cycleIndex(status)
	if i > 0 {
		return cycleOrder[i-1]
	}
	return cycleOrder[i]
}

// column is one kanban column's tasks, already sorted.
type column struct {
	Status model.Status
	Tasks  []*model.Task
}

// buildColumns groups tasks into columnOrder by their derived status.
func buildColumns(tasks []*model.Task, all map[string]*model.Task, sortMode sortMode) []column {
	byStatus := make(map[model.Status][]*model.Task, len(columnOrder))
	for _, t := range tasks {
		derived := depgraph.DerivedStatus(t, all)
		byStatus[derived] = append(byStatus[derived], t)
	}

	cols := make([]column, len(columnOrder))
	for i, status := range columnOrder {
		ts := byStatus[status]
		sortTasks(ts, sortMode)
		cols[i] = column{Status: status, Tasks: ts}
	}
	return cols
}

type sortMode int

const (
	sortByPriority sortMode = iota
	sortByUpdated
	sortByCreated
)

func (m sortMode) String() string {
	switch m {
	case sortByPriority:
		return "priority"
	case sortByUpdated:
		return "updated_at"
	case sortByCreated:
		return "created_at"
	default:
		return "?"
	}
}

func (m sortMode) next() sortMode {
	return (m + 1) % 3
}

var priorityRank = map[model.Priority]int{
	model.PriorityUrgent: 0,
	model.PriorityHigh:   1,
	model.PriorityMedium: 2,
	model.PriorityLow:    3,
}

func sortTasks(tasks []*model.Task, mode sortMode) {
	sort.SliceStable(tasks, func(i, j int) bool {
		switch mode {
		case sortByUpdated:
			return tasks[i].UpdatedAt.After(tasks[j].UpdatedAt)
		case sortByCreated:
			return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
		default:
			pi, pj := priorityRank[tasks[i].Priority], priorityRank[tasks[j].Priority]
			if pi != pj {
				return pi < pj
			}
			return tasks[i].ID < tasks[j].ID
		}
	})
}
