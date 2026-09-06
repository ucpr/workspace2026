package cli

import (
	"github.com/ucpr/atama/internal/depgraph"
	"github.com/ucpr/atama/internal/model"
)

// taskView adds derived, non-persisted fields to a task for CLI output
// (requirements §5.1's manual/derived blocked distinction, §8.2's
// requirement that `next` surface everything an agent needs to act).
type taskView struct {
	*model.Task
	EffectiveStatus model.Status `json:"effective_status"`
	Ready           bool         `json:"ready"`
}

func newTaskView(t *model.Task, all map[string]*model.Task) taskView {
	return taskView{
		Task:            t,
		EffectiveStatus: depgraph.DerivedStatus(t, all),
		Ready:           depgraph.IsReady(t, all),
	}
}

func newTaskViews(tasks []*model.Task, all map[string]*model.Task) []taskView {
	views := make([]taskView, len(tasks))
	for i, t := range tasks {
		views[i] = newTaskView(t, all)
	}
	return views
}
