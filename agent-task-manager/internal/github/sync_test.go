package github

import (
	"context"
	"testing"
	"time"

	"github.com/ucpr/atama/internal/model"
)

// decideTaskAction is pure, so exercise it directly with hand-built inputs
// instead of going through the full Sync/store plumbing.
func TestDecideTaskAction(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := base.Add(time.Hour)

	mk := func(localUpdated, syncedAt time.Time) *model.Task {
		return &model.Task{
			UpdatedAt: localUpdated,
			GitHub:    &model.GitHubMeta{SyncedAt: syncedAt},
		}
	}

	cases := []struct {
		name           string
		localUpdatedAt time.Time
		syncedAt       time.Time
		issueUpdatedAt time.Time
		remoteKnownAt  time.Time
		want           TaskAction
	}{
		{"no changes", base, base, base, base, ActionNone},
		{"remote only changed", base, base, newer, base, ActionPull},
		{"local only changed", newer, base, base, base, ActionPush},
		{"both changed, local newer", newer.Add(time.Hour), base, newer, base, ActionConflictLocalWins},
		{"both changed, remote newer", newer, base, newer.Add(time.Hour), base, ActionConflictRemoteWins},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			task := mk(c.localUpdatedAt, c.syncedAt)
			task.GitHub.RemoteUpdatedAt = c.remoteKnownAt
			issue := &Issue{UpdatedAt: c.issueUpdatedAt}
			if got := decideTaskAction(task, issue); got != c.want {
				t.Errorf("decideTaskAction() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestSync_PushesLocalChanges(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	api := newFakeAPI()

	task := &model.Task{ID: "atm-1", Title: "orig", Status: model.StatusBacklog, Priority: model.PriorityMedium}
	st.CreateTask(task)
	exported, err := Export(ctx, st, api, testRepo, task.ID)
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	exported.Title = "renamed locally"
	exported.UpdatedAt = time.Now().UTC().Add(time.Hour)
	if err := st.UpdateTask(exported); err != nil {
		t.Fatalf("UpdateTask() error = %v", err)
	}

	result, err := Sync(ctx, st, api, testRepo, SyncOptions{})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if len(result.Tasks) != 1 || result.Tasks[0].Action != ActionPush {
		t.Fatalf("Sync() plan = %+v, want push", result.Tasks)
	}
	iss, _ := api.GetIssue(ctx, exported.GitHub.IssueNumber)
	if iss.Title != "renamed locally" {
		t.Errorf("remote title = %q, want pushed local title", iss.Title)
	}
}

func TestSync_PullsRemoteChanges(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	api := newFakeAPI()

	task := &model.Task{ID: "atm-1", Title: "orig", Status: model.StatusBacklog, Priority: model.PriorityMedium}
	st.CreateTask(task)
	exported, err := Export(ctx, st, api, testRepo, task.ID)
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}

	iss := api.issues[exported.GitHub.IssueNumber]
	iss.Title = "renamed remotely"
	iss.UpdatedAt = time.Now().UTC().Add(time.Hour)

	result, err := Sync(ctx, st, api, testRepo, SyncOptions{})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if len(result.Tasks) != 1 || result.Tasks[0].Action != ActionPull {
		t.Fatalf("Sync() plan = %+v, want pull", result.Tasks)
	}
	got, err := st.GetTask(exported.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if got.Title != "renamed remotely" {
		t.Errorf("local title = %q, want pulled remote title", got.Title)
	}
}

func TestSync_DryRunDoesNotMutate(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	api := newFakeAPI()

	task := &model.Task{ID: "atm-1", Title: "orig", Status: model.StatusBacklog, Priority: model.PriorityMedium}
	st.CreateTask(task)
	exported, err := Export(ctx, st, api, testRepo, task.ID)
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	iss := api.issues[exported.GitHub.IssueNumber]
	iss.Title = "renamed remotely"
	iss.UpdatedAt = time.Now().UTC().Add(time.Hour)

	result, err := Sync(ctx, st, api, testRepo, SyncOptions{DryRun: true})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	if len(result.Tasks) != 1 || result.Tasks[0].Action != ActionPull {
		t.Fatalf("Sync() plan = %+v, want pull (planned, not applied)", result.Tasks)
	}
	got, _ := st.GetTask(exported.ID)
	if got.Title != "orig" {
		t.Errorf("local title = %q, want unchanged under --dry-run", got.Title)
	}
}

func TestDecideCommentAction(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	newer := base.Add(time.Hour)
	ghID := int64(42)

	cases := []struct {
		name         string
		local        model.Comment
		remote       *Comment
		remoteExists bool
		want         CommentAction
	}{
		{
			name:  "new local comment, never synced",
			local: model.Comment{ID: "c1", Body: "hi"},
			want:  CommentActionCreateRemote,
		},
		{
			name:         "remote deleted it",
			local:        model.Comment{ID: "c1", GitHubCommentID: &ghID, SyncedAt: &base, UpdatedAt: base},
			remoteExists: false,
			want:         CommentActionDeleteLocal,
		},
		{
			name:         "local tombstone, remote still has it: delete-wins",
			local:        model.Comment{ID: "c1", GitHubCommentID: &ghID, DeletedAt: &newer},
			remote:       &Comment{ID: ghID, Body: "edited remotely after local delete", UpdatedAt: newer},
			remoteExists: true,
			want:         CommentActionDeleteRemote,
		},
		{
			name:  "local tombstone, remote already gone: just purge",
			local: model.Comment{ID: "c1", GitHubCommentID: &ghID, DeletedAt: &newer},
			want:  CommentActionPurgeLocal,
		},
		{
			name:  "local tombstone, never synced: just purge",
			local: model.Comment{ID: "c1", DeletedAt: &newer},
			want:  CommentActionPurgeLocal,
		},
		{
			name:         "no changes either side",
			local:        model.Comment{ID: "c1", GitHubCommentID: &ghID, SyncedAt: &base, UpdatedAt: base},
			remote:       &Comment{ID: ghID, UpdatedAt: base},
			remoteExists: true,
			want:         CommentActionNone,
		},
		{
			name:         "remote edited only",
			local:        model.Comment{ID: "c1", GitHubCommentID: &ghID, SyncedAt: &base, UpdatedAt: base},
			remote:       &Comment{ID: ghID, UpdatedAt: newer},
			remoteExists: true,
			want:         CommentActionPullEdit,
		},
		{
			name:         "local edited only",
			local:        model.Comment{ID: "c1", GitHubCommentID: &ghID, SyncedAt: &base, UpdatedAt: newer},
			remote:       &Comment{ID: ghID, UpdatedAt: base},
			remoteExists: true,
			want:         CommentActionPushEdit,
		},
		{
			name:         "both edited, local newer",
			local:        model.Comment{ID: "c1", GitHubCommentID: &ghID, SyncedAt: &base, UpdatedAt: newer.Add(time.Hour)},
			remote:       &Comment{ID: ghID, UpdatedAt: newer},
			remoteExists: true,
			want:         CommentActionPushEdit,
		},
		{
			name:         "both edited, remote newer",
			local:        model.Comment{ID: "c1", GitHubCommentID: &ghID, SyncedAt: &base, UpdatedAt: newer},
			remote:       &Comment{ID: ghID, UpdatedAt: newer.Add(time.Hour)},
			remoteExists: true,
			want:         CommentActionPullEdit,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := decideCommentAction(&c.local, c.remote, c.remoteExists)
			if got != c.want {
				t.Errorf("decideCommentAction() = %q, want %q", got, c.want)
			}
		})
	}
}

func TestSync_Comments_FullLifecycle(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	api := newFakeAPI()

	task := &model.Task{ID: "atm-1", Title: "t", Status: model.StatusBacklog, Priority: model.PriorityMedium}
	st.CreateTask(task)
	exported, err := Export(ctx, st, api, testRepo, task.ID)
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	number := exported.GitHub.IssueNumber

	// A new local-only comment should be pushed to GitHub.
	exported.Comments = append(exported.Comments, model.Comment{ID: "atm-c-1", Body: "local note", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()})
	if err := st.UpdateTask(exported); err != nil {
		t.Fatalf("UpdateTask() error = %v", err)
	}

	// A remote-only comment should be pulled in.
	if _, err := api.CreateComment(ctx, number, "remote note"); err != nil {
		t.Fatalf("CreateComment() error = %v", err)
	}

	result, err := Sync(ctx, st, api, testRepo, SyncOptions{})
	if err != nil {
		t.Fatalf("Sync() error = %v", err)
	}
	got, err := st.GetTask(exported.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if len(got.Comments) != 2 {
		t.Fatalf("Comments = %+v, want 2 (one pushed, one pulled)", got.Comments)
	}
	bodies := map[string]bool{}
	for _, c := range got.Comments {
		bodies[c.Body] = true
		if c.GitHubCommentID == nil {
			t.Errorf("comment %+v missing GitHubCommentID after sync", c)
		}
		if c.SyncedAt == nil {
			t.Errorf("comment %+v missing SyncedAt after sync", c)
		}
	}
	if !bodies["local note"] || !bodies["remote note"] {
		t.Errorf("comment bodies = %v, want local note and remote note both present", bodies)
	}
	remoteComments, _ := api.ListComments(ctx, number)
	if len(remoteComments) != 2 {
		t.Fatalf("remote comments = %v, want 2 (local note pushed alongside remote note)", remoteComments)
	}

	_ = result

	// Now delete the "local note" comment locally (tombstone) and re-sync:
	// delete-wins should propagate the delete to GitHub and purge it locally.
	var localNoteID string
	for _, c := range got.Comments {
		if c.Body == "local note" {
			localNoteID = c.ID
		}
	}
	now := time.Now().UTC()
	for i := range got.Comments {
		if got.Comments[i].ID == localNoteID {
			got.Comments[i].DeletedAt = &now
		}
	}
	if err := st.UpdateTask(got); err != nil {
		t.Fatalf("UpdateTask() error = %v", err)
	}

	if _, err := Sync(ctx, st, api, testRepo, SyncOptions{}); err != nil {
		t.Fatalf("Sync() (delete propagation) error = %v", err)
	}
	final, _ := st.GetTask(exported.ID)
	if len(final.Comments) != 1 || final.Comments[0].Body != "remote note" {
		t.Fatalf("Comments after delete-wins sync = %+v, want only [remote note]", final.Comments)
	}
	remoteAfterDelete, _ := api.ListComments(ctx, number)
	if len(remoteAfterDelete) != 1 {
		t.Fatalf("remote comments after delete-wins sync = %v, want 1 (local note removed remotely)", remoteAfterDelete)
	}
}

func TestSync_Comments_RemoteDeletionTombstonesLocally(t *testing.T) {
	ctx := context.Background()
	st := newTestStore(t)
	api := newFakeAPI()

	task := &model.Task{ID: "atm-1", Title: "t", Status: model.StatusBacklog, Priority: model.PriorityMedium}
	st.CreateTask(task)
	exported, err := Export(ctx, st, api, testRepo, task.ID)
	if err != nil {
		t.Fatalf("Export() error = %v", err)
	}
	number := exported.GitHub.IssueNumber
	created, err := api.CreateComment(ctx, number, "will be deleted remotely")
	if err != nil {
		t.Fatalf("CreateComment() error = %v", err)
	}

	if _, err := Sync(ctx, st, api, testRepo, SyncOptions{}); err != nil {
		t.Fatalf("Sync() first pass error = %v", err)
	}

	if err := api.DeleteComment(ctx, created.ID); err != nil {
		t.Fatalf("DeleteComment() error = %v", err)
	}

	if _, err := Sync(ctx, st, api, testRepo, SyncOptions{}); err != nil {
		t.Fatalf("Sync() second pass error = %v", err)
	}
	got, _ := st.GetTask(exported.ID)
	if len(got.Comments) != 1 || got.Comments[0].DeletedAt == nil {
		t.Fatalf("Comments = %+v, want the single comment tombstoned (deleted_at set)", got.Comments)
	}
}
