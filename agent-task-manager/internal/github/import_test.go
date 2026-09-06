package github

import (
	"context"
	"testing"
	"time"

	"github.com/ucpr/atama/internal/model"
	"github.com/ucpr/atama/internal/store"
)

const testRepo = "acme/widgets"

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	s, err := store.Init(t.TempDir())
	if err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}
	return s
}

func TestImport_CreatesNewTasks(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	api := newFakeAPI()
	iss, _ := api.CreateIssue(ctx, IssueInput{Title: "fix bug", Body: "steps to repro"})
	api.issues[iss.Number].UpdatedAt = time.Now().UTC()

	result, err := Import(ctx, st, api, testRepo, ImportOptions{})
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if len(result.Created) != 1 {
		t.Fatalf("Created = %v, want 1 task", result.Created)
	}
	task := result.Created[0]
	if task.Title != "fix bug" || task.Body != "steps to repro" {
		t.Errorf("task = %+v, want title/body from issue", task)
	}
	if task.GitHub == nil || task.GitHub.IssueNumber != iss.Number || task.GitHub.Repo != testRepo {
		t.Errorf("task.GitHub = %+v, want linked to issue #%d", task.GitHub, iss.Number)
	}
	if task.Status != model.StatusBacklog {
		t.Errorf("Status = %q, want backlog for an open issue", task.Status)
	}
}

func TestImport_ClosedIssueMapsToDone(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	api := newFakeAPI()
	api.CreateIssue(ctx, IssueInput{Title: "done already", State: "closed"})

	result, err := Import(ctx, st, api, testRepo, ImportOptions{})
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	if result.Created[0].Status != model.StatusDone {
		t.Errorf("Status = %q, want done for a closed issue", result.Created[0].Status)
	}
}

func TestImport_SkipsExistingByDefault(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	api := newFakeAPI()
	api.CreateIssue(ctx, IssueInput{Title: "v1"})

	if _, err := Import(ctx, st, api, testRepo, ImportOptions{}); err != nil {
		t.Fatalf("Import() first pass error = %v", err)
	}
	api.issues[1].Title = "v2"

	result, err := Import(ctx, st, api, testRepo, ImportOptions{Overwrite: false})
	if err != nil {
		t.Fatalf("Import() second pass error = %v", err)
	}
	if len(result.Skipped) != 1 || len(result.Created) != 0 || len(result.Updated) != 0 {
		t.Fatalf("result = %+v, want issue #1 skipped", result)
	}

	tasks, _ := st.ListTasks()
	if tasks[0].Title != "v1" {
		t.Errorf("local title = %q, want unchanged %q", tasks[0].Title, "v1")
	}
}

func TestImport_OverwriteRefreshesExisting(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	api := newFakeAPI()
	api.CreateIssue(ctx, IssueInput{Title: "v1"})
	if _, err := Import(ctx, st, api, testRepo, ImportOptions{}); err != nil {
		t.Fatalf("Import() first pass error = %v", err)
	}
	api.issues[1].Title = "v2"
	api.issues[1].State = "closed"

	result, err := Import(ctx, st, api, testRepo, ImportOptions{Overwrite: true})
	if err != nil {
		t.Fatalf("Import() second pass error = %v", err)
	}
	if len(result.Updated) != 1 {
		t.Fatalf("result = %+v, want issue #1 updated", result)
	}
	if result.Updated[0].Title != "v2" || result.Updated[0].Status != model.StatusDone {
		t.Errorf("updated task = %+v, want title v2 and status done", result.Updated[0])
	}
}

func TestImport_RoundTripsDependsOnAndGoalIDs(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	api := newFakeAPI()
	body := appendMeta("desc", metaBlock{AtamaID: "atm-x", DependsOn: []string{"atm-a"}, GoalIDs: []string{"atm-g-1"}})
	api.CreateIssue(ctx, IssueInput{Title: "with meta", Body: body})

	result, err := Import(ctx, st, api, testRepo, ImportOptions{})
	if err != nil {
		t.Fatalf("Import() error = %v", err)
	}
	task := result.Created[0]
	if task.Body != "desc" {
		t.Errorf("Body = %q, want meta block stripped", task.Body)
	}
	if len(task.DependsOn) != 1 || task.DependsOn[0] != "atm-a" {
		t.Errorf("DependsOn = %v, want [atm-a]", task.DependsOn)
	}
	if len(task.GoalIDs) != 1 || task.GoalIDs[0] != "atm-g-1" {
		t.Errorf("GoalIDs = %v, want [atm-g-1]", task.GoalIDs)
	}
}
