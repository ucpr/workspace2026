package depgraph

import "github.com/ucpr/atama/internal/model"

// IsReady reports whether task can be started now: it isn't already
// terminal or in flight, and every dependency present in tasks is done. A
// dependency that is cancelled, or missing from tasks, is treated as
// unsatisfied — the default policy in requirements §5.2 blocks dependents of
// a cancelled task rather than letting them proceed.
func IsReady(task *model.Task, tasks map[string]*model.Task) bool {
	switch task.Status {
	case model.StatusDone, model.StatusCancelled, model.StatusInProgress, model.StatusInReview:
		return false
	}
	for _, dep := range task.DependsOn {
		d, ok := tasks[dep]
		if !ok || d.Status != model.StatusDone {
			return false
		}
	}
	return true
}

// DerivedStatus returns task's effective status for display: its own
// Status, unless unresolved dependencies mean it should be reported as
// blocked (requirements §5.1's distinction between manual and derived
// `blocked`). Terminal statuses are always reported as-is.
func DerivedStatus(task *model.Task, tasks map[string]*model.Task) model.Status {
	if task.Status.Terminal() {
		return task.Status
	}
	for _, dep := range task.DependsOn {
		d, ok := tasks[dep]
		if !ok || d.Status != model.StatusDone {
			return model.StatusBlocked
		}
	}
	return task.Status
}

// ReadyTasks filters ids to those that are ready per IsReady, ordered
// topologically (requirements §5.2, §5.3).
func ReadyTasks(tasks map[string]*model.Task, ids []string) ([]*model.Task, error) {
	order, err := TopoSort(tasks, ids)
	if err != nil {
		return nil, err
	}
	var ready []*model.Task
	for _, id := range order {
		t, ok := tasks[id]
		if !ok {
			continue
		}
		if IsReady(t, tasks) {
			ready = append(ready, t)
		}
	}
	return ready, nil
}
