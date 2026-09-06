package github

import (
	"context"
	"errors"
	"testing"

	"github.com/ucpr/atama/internal/model"
)

func TestExport_CreatesIssueAndLinksTask(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	api := newFakeAPI()

	task := &model.Task{ID: "atm-1", Title: "ship it", Body: "the plan", Status: model.StatusBacklog, Priority: model.PriorityMedium}
	if err := st.CreateTask(task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}

	exported, err := Export(ctx, st, api, testRepo, task.ID)
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	if exported.GitHub == nil || exported.GitHub.Repo != testRepo {
		t.Fatalf("GitHub meta = %+v, want linked to %s", exported.GitHub, testRepo)
	}

	iss, err := api.GetIssue(ctx, exported.GitHub.IssueNumber)
	if err != nil {
		t.Fatalf("GetIssue() error = %v", err)
	}
	if iss.Title != "ship it" {
		t.Errorf("issue title = %q, want %q", iss.Title, "ship it")
	}
	clean, _ := stripMeta(iss.Body)
	if clean != "the plan" {
		t.Errorf("issue body (meta stripped) = %q, want %q", clean, "the plan")
	}
}

func TestExport_RejectsAlreadyExported(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	api := newFakeAPI()
	task := &model.Task{ID: "atm-1", Title: "x", Status: model.StatusBacklog, Priority: model.PriorityMedium}
	st.CreateTask(task)

	if _, err := Export(ctx, st, api, testRepo, task.ID); err != nil {
		t.Fatalf("first Export() error = %v", err)
	}
	if _, err := Export(ctx, st, api, testRepo, task.ID); !errors.Is(err, ErrAlreadyExported) {
		t.Errorf("second Export() error = %v, want ErrAlreadyExported", err)
	}
}

func TestExport_ClosedStatusCreatesClosedIssue(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	api := newFakeAPI()
	task := &model.Task{ID: "atm-1", Title: "x", Status: model.StatusDone, Priority: model.PriorityMedium}
	st.CreateTask(task)

	exported, err := Export(ctx, st, api, testRepo, task.ID)
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	iss, _ := api.GetIssue(ctx, exported.GitHub.IssueNumber)
	if iss.State != "closed" {
		t.Errorf("issue state = %q, want closed for a done task", iss.State)
	}
}
