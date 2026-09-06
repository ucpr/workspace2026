package depgraph

import (
	"testing"

	"github.com/ucpr/atama/internal/model"
)

func task(id string, deps ...string) *model.Task {
	return &model.Task{ID: id, Status: model.StatusBacklog, DependsOn: deps}
}

func TestFindCycle_NoCycle(t *testing.T) {
	tasks := map[string]*model.Task{
		"a": task("a"),
		"b": task("b", "a"),
		"c": task("c", "a", "b"),
	}
	if cyc := FindCycle(tasks); cyc != nil {
		t.Fatalf("FindCycle() = %v, want nil", cyc)
	}
}

func TestFindCycle_DirectCycle(t *testing.T) {
	tasks := map[string]*model.Task{
		"a": task("a", "b"),
		"b": task("b", "a"),
	}
	cyc := FindCycle(tasks)
	if cyc == nil {
		t.Fatal("FindCycle() = nil, want cycle error")
	}
}

func TestFindCycle_SelfDependency(t *testing.T) {
	tasks := map[string]*model.Task{
		"a": task("a", "a"),
	}
	if cyc := FindCycle(tasks); cyc == nil {
		t.Fatal("FindCycle() = nil, want cycle error for self-dependency")
	}
}

func TestFindCycle_IgnoresUnknownDeps(t *testing.T) {
	tasks := map[string]*model.Task{
		"a": task("a", "ghost"),
	}
	if cyc := FindCycle(tasks); cyc != nil {
		t.Fatalf("FindCycle() = %v, want nil (unknown dep should be ignored)", cyc)
	}
}

func TestWouldCreateCycle(t *testing.T) {
	tasks := map[string]*model.Task{
		"a": task("a"),
		"b": task("b", "a"),
	}
	if !WouldCreateCycle(tasks, "a", "b") {
		t.Error("WouldCreateCycle(a, b) = false, want true (a<-b<-a)")
	}
	if WouldCreateCycle(tasks, "b", "a") {
		t.Error("WouldCreateCycle(b, a) = true, want false (already exists, no new cycle)")
	}
	if !WouldCreateCycle(tasks, "c", "c") {
		t.Error("WouldCreateCycle(c, c) = false, want true (self dependency)")
	}
}

func TestTopoSort_OrdersByDependency(t *testing.T) {
	tasks := map[string]*model.Task{
		"a": task("a"),
		"b": task("b", "a"),
		"c": task("c", "b"),
	}
	got, err := TopoSort(tasks, []string{"c", "b", "a"})
	if err != nil {
		t.Fatalf("TopoSort() error = %v", err)
	}
	want := []string{"a", "b", "c"}
	if !equalSlices(got, want) {
		t.Errorf("TopoSort() = %v, want %v", got, want)
	}
}

func TestTopoSort_IgnoresDepsOutsideSubset(t *testing.T) {
	tasks := map[string]*model.Task{
		"a": task("a"),
		"b": task("b", "a"),
		"c": task("c", "b"),
	}
	// Subset excludes "a"; "b" should sort fine without it.
	got, err := TopoSort(tasks, []string{"c", "b"})
	if err != nil {
		t.Fatalf("TopoSort() error = %v", err)
	}
	want := []string{"b", "c"}
	if !equalSlices(got, want) {
		t.Errorf("TopoSort() = %v, want %v", got, want)
	}
}

func TestTopoSort_CycleError(t *testing.T) {
	tasks := map[string]*model.Task{
		"a": task("a", "b"),
		"b": task("b", "a"),
	}
	_, err := TopoSort(tasks, []string{"a", "b"})
	if err == nil {
		t.Fatal("TopoSort() error = nil, want cycle error")
	}
}

func TestIsReady(t *testing.T) {
	done := task("dep", nil...)
	done.Status = model.StatusDone
	pending := task("pending", nil...)
	cancelled := task("cancelled-dep", nil...)
	cancelled.Status = model.StatusCancelled

	tasks := map[string]*model.Task{
		"dep":           done,
		"pending":       pending,
		"cancelled-dep": cancelled,
	}

	cases := []struct {
		name string
		t    *model.Task
		want bool
	}{
		{"no deps, backlog", task("x"), true},
		{"dep done", task("x", "dep"), true},
		{"dep pending", task("x", "pending"), false},
		{"dep cancelled blocks", task("x", "cancelled-dep"), false},
		{"unknown dep blocks", task("x", "ghost"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := IsReady(c.t, tasks); got != c.want {
				t.Errorf("IsReady(%s) = %v, want %v", c.name, got, c.want)
			}
		})
	}

	inProgress := task("in-progress-no-deps")
	inProgress.Status = model.StatusInProgress
	if IsReady(inProgress, tasks) {
		t.Error("IsReady(in_progress) = true, want false (already started)")
	}

	doneTask := task("done-no-deps")
	doneTask.Status = model.StatusDone
	if IsReady(doneTask, tasks) {
		t.Error("IsReady(done) = true, want false (terminal)")
	}
}

func TestDerivedStatus(t *testing.T) {
	done := task("dep")
	done.Status = model.StatusDone
	tasks := map[string]*model.Task{"dep": done}

	blocked := task("x", "dep")
	blocked.Status = model.StatusBacklog
	// dep not done yet in this second scenario
	pendingDep := task("pending-dep")
	tasksWithPending := map[string]*model.Task{"pending-dep": pendingDep}
	blockedByPending := task("x", "pending-dep")
	if got := DerivedStatus(blockedByPending, tasksWithPending); got != model.StatusBlocked {
		t.Errorf("DerivedStatus() = %v, want blocked", got)
	}

	ready := task("y", "dep")
	if got := DerivedStatus(ready, tasks); got != model.StatusBacklog {
		t.Errorf("DerivedStatus() = %v, want backlog (deps satisfied)", got)
	}

	cancelledTask := task("z")
	cancelledTask.Status = model.StatusCancelled
	if got := DerivedStatus(cancelledTask, tasks); got != model.StatusCancelled {
		t.Errorf("DerivedStatus() = %v, want cancelled (terminal statuses pass through)", got)
	}
}

func TestReadyTasks_Ordered(t *testing.T) {
	a := task("a")
	a.Status = model.StatusDone
	b := task("b", "a")
	c := task("c", "b")
	d := task("d") // independent, no deps

	tasks := map[string]*model.Task{"a": a, "b": b, "c": c, "d": d}
	got, err := ReadyTasks(tasks, []string{"a", "b", "c", "d"})
	if err != nil {
		t.Fatalf("ReadyTasks() error = %v", err)
	}
	// a is done (not ready/terminal excluded), c depends on b which isn't
	// done yet, so only b and d should be ready.
	if len(got) != 2 {
		t.Fatalf("ReadyTasks() = %v, want 2 ready tasks", ids(got))
	}
	seen := map[string]bool{}
	for _, tk := range got {
		seen[tk.ID] = true
	}
	if !seen["b"] || !seen["d"] {
		t.Errorf("ReadyTasks() = %v, want b and d", ids(got))
	}
}

func ids(tasks []*model.Task) []string {
	out := make([]string, len(tasks))
	for i, t := range tasks {
		out[i] = t.ID
	}
	return out
}

func equalSlices(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
