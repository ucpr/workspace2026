package service

import (
	"errors"
	"testing"

	"github.com/ucpr/atama/internal/model"
	"github.com/ucpr/atama/internal/store"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	st, err := store.Init(t.TempDir())
	if err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}
	return New(st)
}

func TestAddTask_Defaults(t *testing.T) {
	svc := newTestService(t)
	task, err := svc.AddTask(AddTaskInput{Title: "write docs"})
	if err != nil {
		t.Fatalf("AddTask() error = %v", err)
	}
	if task.Status != model.StatusBacklog {
		t.Errorf("Status = %q, want backlog", task.Status)
	}
	if task.Priority != model.PriorityMedium {
		t.Errorf("Priority = %q, want medium", task.Priority)
	}
	if task.CreatedAt.IsZero() || task.UpdatedAt.IsZero() {
		t.Error("timestamps not set")
	}
}

func TestAddTask_RequiresTitle(t *testing.T) {
	svc := newTestService(t)
	if _, err := svc.AddTask(AddTaskInput{}); err == nil {
		t.Fatal("AddTask() with empty title = nil error, want error")
	}
}

func TestAddTask_UnknownDependency(t *testing.T) {
	svc := newTestService(t)
	_, err := svc.AddTask(AddTaskInput{Title: "x", DependsOn: []string{"atm-ghost"}})
	if err == nil {
		t.Fatal("AddTask() with unknown dependency = nil error, want error")
	}
}

func TestAddDependency_RejectsCycle(t *testing.T) {
	svc := newTestService(t)
	a, err := svc.AddTask(AddTaskInput{Title: "a"})
	if err != nil {
		t.Fatalf("AddTask() error = %v", err)
	}
	b, err := svc.AddTask(AddTaskInput{Title: "b"})
	if err != nil {
		t.Fatalf("AddTask() error = %v", err)
	}
	if _, err := svc.AddDependency(b.ID, a.ID); err != nil {
		t.Fatalf("AddDependency(b, a) error = %v", err)
	}
	if _, err := svc.AddDependency(a.ID, b.ID); err == nil {
		t.Fatal("AddDependency(a, b) = nil error, want cycle rejection")
	}
	if _, err := svc.AddDependency(a.ID, a.ID); err == nil {
		t.Fatal("AddDependency(a, a) = nil error, want self-dependency rejection")
	}
}

func TestRemoveDependency(t *testing.T) {
	svc := newTestService(t)
	a, _ := svc.AddTask(AddTaskInput{Title: "a"})
	b, _ := svc.AddTask(AddTaskInput{Title: "b"})
	if _, err := svc.AddDependency(b.ID, a.ID); err != nil {
		t.Fatalf("AddDependency() error = %v", err)
	}
	updated, err := svc.RemoveDependency(b.ID, a.ID)
	if err != nil {
		t.Fatalf("RemoveDependency() error = %v", err)
	}
	if len(updated.DependsOn) != 0 {
		t.Errorf("DependsOn = %v, want empty", updated.DependsOn)
	}
}

func TestSetStatus_StampsClosedAt(t *testing.T) {
	svc := newTestService(t)
	task, _ := svc.AddTask(AddTaskInput{Title: "a"})

	done, err := svc.SetStatus(task.ID, model.StatusDone)
	if err != nil {
		t.Fatalf("SetStatus(done) error = %v", err)
	}
	if done.ClosedAt == nil {
		t.Error("ClosedAt not set after reaching done")
	}

	reopened, err := svc.SetStatus(task.ID, model.StatusInProgress)
	if err != nil {
		t.Fatalf("SetStatus(in_progress) error = %v", err)
	}
	if reopened.ClosedAt != nil {
		t.Error("ClosedAt should be cleared after leaving a terminal status")
	}
}

func TestSetStatus_InvalidStatus(t *testing.T) {
	svc := newTestService(t)
	task, _ := svc.AddTask(AddTaskInput{Title: "a"})
	if _, err := svc.SetStatus(task.ID, "bogus"); err == nil {
		t.Fatal("SetStatus(bogus) = nil error, want error")
	}
}

func TestRemoveTask_CleansUpReferences(t *testing.T) {
	svc := newTestService(t)
	a, _ := svc.AddTask(AddTaskInput{Title: "a"})
	b, err := svc.AddTask(AddTaskInput{Title: "b", DependsOn: []string{a.ID}})
	if err != nil {
		t.Fatalf("AddTask() error = %v", err)
	}
	goal, err := svc.AddGoal(AddGoalInput{Name: "g", TaskIDs: []string{a.ID, b.ID}})
	if err != nil {
		t.Fatalf("AddGoal() error = %v", err)
	}

	if err := svc.RemoveTask(a.ID); err != nil {
		t.Fatalf("RemoveTask() error = %v", err)
	}

	gotB, err := svc.GetTask(b.ID)
	if err != nil {
		t.Fatalf("GetTask(b) error = %v", err)
	}
	if len(gotB.DependsOn) != 0 {
		t.Errorf("b.DependsOn = %v, want empty after a removed", gotB.DependsOn)
	}

	gotGoal, err := svc.GetGoal(goal.ID)
	if err != nil {
		t.Fatalf("GetGoal() error = %v", err)
	}
	if len(gotGoal.TaskIDs) != 1 || gotGoal.TaskIDs[0] != b.ID {
		t.Errorf("goal.TaskIDs = %v, want [%s]", gotGoal.TaskIDs, b.ID)
	}

	if _, err := svc.GetTask(a.ID); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("GetTask(a) after removal error = %v, want ErrNotFound", err)
	}
}

func TestResolveTaskID_Prefix(t *testing.T) {
	svc := newTestService(t)
	task, _ := svc.AddTask(AddTaskInput{Title: "a"})

	got, err := svc.ResolveTaskID(task.ID[:8])
	if err != nil {
		t.Fatalf("ResolveTaskID() error = %v", err)
	}
	if got != task.ID {
		t.Errorf("ResolveTaskID() = %q, want %q", got, task.ID)
	}

	if _, err := svc.ResolveTaskID("atm-doesnotexist"); err == nil {
		t.Error("ResolveTaskID() unknown = nil error, want error")
	}
}

func TestNext_ReadyOrder(t *testing.T) {
	svc := newTestService(t)
	a, _ := svc.AddTask(AddTaskInput{Title: "a"})
	b, err := svc.AddTask(AddTaskInput{Title: "b", DependsOn: []string{a.ID}})
	if err != nil {
		t.Fatalf("AddTask() error = %v", err)
	}
	goal, err := svc.AddGoal(AddGoalInput{Name: "g", TaskIDs: []string{a.ID, b.ID}})
	if err != nil {
		t.Fatalf("AddGoal() error = %v", err)
	}

	ready, err := svc.Next(goal.ID)
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if len(ready) != 1 || ready[0].ID != a.ID {
		t.Fatalf("Next() = %v, want only [a] (b depends on incomplete a)", idsOf(ready))
	}

	if _, err := svc.Complete(a.ID); err != nil {
		t.Fatalf("Complete() error = %v", err)
	}
	ready, err = svc.Next(goal.ID)
	if err != nil {
		t.Fatalf("Next() error = %v", err)
	}
	if len(ready) != 1 || ready[0].ID != b.ID {
		t.Fatalf("Next() after completing a = %v, want only [b]", idsOf(ready))
	}
}

func TestAddComment(t *testing.T) {
	svc := newTestService(t)
	task, _ := svc.AddTask(AddTaskInput{Title: "a"})

	updated, err := svc.AddComment(task.ID, "alice", "looks good")
	if err != nil {
		t.Fatalf("AddComment() error = %v", err)
	}
	if len(updated.Comments) != 1 {
		t.Fatalf("Comments = %v, want 1", updated.Comments)
	}
	c := updated.Comments[0]
	if c.Author != "alice" || c.Body != "looks good" {
		t.Errorf("comment = %+v, want author=alice body=looks good", c)
	}
	if c.GitHubCommentID != nil {
		t.Errorf("GitHubCommentID = %v, want nil for a local-only comment", c.GitHubCommentID)
	}

	if _, err := svc.AddComment(task.ID, "alice", ""); err == nil {
		t.Error("AddComment() with empty body = nil error, want error")
	}
}

func idsOf(tasks []*model.Task) []string {
	out := make([]string, len(tasks))
	for i, t := range tasks {
		out[i] = t.ID
	}
	return out
}
