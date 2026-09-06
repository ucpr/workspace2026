package github

import (
	"context"
	"fmt"
	"time"

	"github.com/ucpr/atama/internal/id"
	"github.com/ucpr/atama/internal/model"
	"github.com/ucpr/atama/internal/store"
)

// TaskAction is the decision Sync made for one task's Issue-level fields
// (title, body, labels, assignees, open/closed state).
type TaskAction string

// Task-level sync actions.
const (
	ActionNone               TaskAction = "none"
	ActionPush               TaskAction = "push"                 // local wins trivially: push local -> remote
	ActionPull               TaskAction = "pull"                 // remote wins trivially: pull remote -> local
	ActionConflictLocalWins  TaskAction = "conflict_local_wins"  // both changed; local updated_at is newer
	ActionConflictRemoteWins TaskAction = "conflict_remote_wins" // both changed; remote updated_at is newer
)

// CommentAction is the decision Sync made for one comment.
type CommentAction string

// Comment-level sync actions (requirements §5.4).
const (
	CommentActionNone         CommentAction = "none"
	CommentActionCreateRemote CommentAction = "create_remote" // new local comment, never synced
	CommentActionCreateLocal  CommentAction = "create_local"  // new remote comment, not seen locally
	CommentActionPushEdit     CommentAction = "push_edit"
	CommentActionPullEdit     CommentAction = "pull_edit"
	CommentActionDeleteRemote CommentAction = "delete_remote" // local tombstone, remote still has it: delete-wins
	CommentActionDeleteLocal  CommentAction = "delete_local"  // remote deleted it: tombstone locally
	CommentActionPurgeLocal   CommentAction = "purge_local"   // tombstone already propagated (or never synced): drop
)

// CommentPlan is one comment's sync decision.
type CommentPlan struct {
	CommentID       string        `json:"comment_id,omitempty"`
	GitHubCommentID *int64        `json:"github_comment_id,omitempty"`
	Action          CommentAction `json:"action"`
}

// TaskPlan is one task's sync decision, including its comments.
type TaskPlan struct {
	TaskID      string        `json:"task_id"`
	IssueNumber int           `json:"issue_number"`
	Action      TaskAction    `json:"action"`
	Comments    []CommentPlan `json:"comments,omitempty"`
}

// SyncOptions controls Sync.
type SyncOptions struct {
	// DryRun computes and returns the plan without calling the remote API
	// or writing to the store (requirements §5.4: `--dry-run`).
	DryRun bool
}

// SyncResult is the full plan/outcome of a Sync call.
type SyncResult struct {
	DryRun bool       `json:"dry_run"`
	Tasks  []TaskPlan `json:"tasks,omitempty"`
}

// Sync reconciles every task linked to repo with its Issue: fields and
// open/closed state bidirectionally with last-write-wins on conflict, and
// comment threads bidirectionally with delete-wins (requirements §5.4).
func Sync(ctx context.Context, st *store.Store, api API, repo string, opts SyncOptions) (*SyncResult, error) {
	tasks, err := st.ListTasks()
	if err != nil {
		return nil, fmt.Errorf("github: listing tasks: %w", err)
	}

	now := time.Now().UTC()
	result := &SyncResult{DryRun: opts.DryRun}
	for _, t := range tasks {
		if t.GitHub == nil || t.GitHub.Repo != repo {
			continue
		}

		issue, err := api.GetIssue(ctx, t.GitHub.IssueNumber)
		if err != nil {
			return nil, fmt.Errorf("github: task %s (issue #%d): %w", t.ID, t.GitHub.IssueNumber, err)
		}

		action := decideTaskAction(t, issue)
		plan := TaskPlan{TaskID: t.ID, IssueNumber: t.GitHub.IssueNumber, Action: action}

		if !opts.DryRun {
			switch action {
			case ActionPull, ActionConflictRemoteWins:
				applyPull(t, issue, now)
			case ActionPush, ActionConflictLocalWins:
				if err := applyPush(ctx, api, t, now); err != nil {
					return nil, fmt.Errorf("github: task %s: %w", t.ID, err)
				}
			}
		}

		commentPlans, err := syncComments(ctx, api, t, opts.DryRun, now)
		if err != nil {
			return nil, fmt.Errorf("github: task %s comments: %w", t.ID, err)
		}
		plan.Comments = commentPlans

		if !opts.DryRun {
			if err := st.UpdateTask(t); err != nil {
				return nil, fmt.Errorf("github: saving task %s: %w", t.ID, err)
			}
		}
		result.Tasks = append(result.Tasks, plan)
	}
	return result, nil
}

// decideTaskAction is pure: it never mutates t or calls the API, so it's
// exercised directly in tests.
func decideTaskAction(t *model.Task, issue *Issue) TaskAction {
	localChanged := t.UpdatedAt.After(t.GitHub.SyncedAt)
	remoteChanged := issue.UpdatedAt.After(t.GitHub.RemoteUpdatedAt)
	switch {
	case !localChanged && !remoteChanged:
		return ActionNone
	case remoteChanged && !localChanged:
		return ActionPull
	case localChanged && !remoteChanged:
		return ActionPush
	default:
		if t.UpdatedAt.After(issue.UpdatedAt) {
			return ActionConflictLocalWins
		}
		return ActionConflictRemoteWins
	}
}

func applyPull(t *model.Task, issue *Issue, now time.Time) {
	body, meta := stripMeta(issue.Body)
	t.Title = issue.Title
	t.Body = body
	t.Labels = issue.Labels
	t.Assignees = issue.Assignees
	t.Status = mapIssueStateToStatus(issue.State, t.Status)
	if meta != nil {
		t.DependsOn = meta.DependsOn
		t.GoalIDs = meta.GoalIDs
	}
	t.GitHub.SyncedAt = now
	t.GitHub.RemoteUpdatedAt = issue.UpdatedAt
	t.UpdatedAt = now
}

func applyPush(ctx context.Context, api API, t *model.Task, now time.Time) error {
	updated, err := api.UpdateIssue(ctx, t.GitHub.IssueNumber, buildIssueInput(t))
	if err != nil {
		return err
	}
	t.GitHub.SyncedAt = now
	t.GitHub.RemoteUpdatedAt = updated.UpdatedAt
	return nil
}

// decideCommentAction is pure, mirroring decideTaskAction.
func decideCommentAction(lc *model.Comment, remote *Comment, remoteExists bool) CommentAction {
	if lc.DeletedAt != nil {
		if lc.GitHubCommentID != nil && remoteExists {
			return CommentActionDeleteRemote
		}
		return CommentActionPurgeLocal
	}
	if lc.GitHubCommentID == nil {
		return CommentActionCreateRemote
	}
	if !remoteExists {
		return CommentActionDeleteLocal
	}

	localChanged := lc.SyncedAt == nil || lc.UpdatedAt.After(*lc.SyncedAt)
	remoteChanged := lc.SyncedAt == nil || remote.UpdatedAt.After(*lc.SyncedAt)
	switch {
	case !localChanged && !remoteChanged:
		return CommentActionNone
	case remoteChanged && !localChanged:
		return CommentActionPullEdit
	case localChanged && !remoteChanged:
		return CommentActionPushEdit
	default:
		if lc.UpdatedAt.After(remote.UpdatedAt) {
			return CommentActionPushEdit
		}
		return CommentActionPullEdit
	}
}

// syncComments reconciles t's comment slice against its Issue's comment
// thread. In dry-run mode it only computes plans; otherwise it also calls
// the API and rewrites t.Comments in place.
func syncComments(ctx context.Context, api API, t *model.Task, dryRun bool, now time.Time) ([]CommentPlan, error) {
	remote, err := api.ListComments(ctx, t.GitHub.IssueNumber)
	if err != nil {
		return nil, err
	}
	remoteByID := make(map[int64]*Comment, len(remote))
	for _, c := range remote {
		remoteByID[c.ID] = c
	}

	var plans []CommentPlan
	var result []model.Comment
	handled := make(map[int64]bool, len(remote))

	for _, lc := range t.Comments {
		var remoteComment *Comment
		var remoteExists bool
		if lc.GitHubCommentID != nil {
			remoteComment, remoteExists = remoteByID[*lc.GitHubCommentID]
			if remoteExists {
				handled[*lc.GitHubCommentID] = true
			}
		}

		action := decideCommentAction(&lc, remoteComment, remoteExists)
		plans = append(plans, CommentPlan{CommentID: lc.ID, GitHubCommentID: lc.GitHubCommentID, Action: action})
		if dryRun {
			continue
		}

		switch action {
		case CommentActionNone:
			result = append(result, lc)
		case CommentActionCreateRemote:
			created, err := api.CreateComment(ctx, t.GitHub.IssueNumber, lc.Body)
			if err != nil {
				return nil, err
			}
			ghID := created.ID
			lc.GitHubCommentID = &ghID
			lc.SyncedAt = &now
			result = append(result, lc)
		case CommentActionPushEdit:
			updated, err := api.UpdateComment(ctx, *lc.GitHubCommentID, lc.Body)
			if err != nil {
				return nil, err
			}
			lc.UpdatedAt = updated.UpdatedAt
			lc.SyncedAt = &now
			result = append(result, lc)
		case CommentActionPullEdit:
			lc.Body = remoteComment.Body
			lc.UpdatedAt = remoteComment.UpdatedAt
			lc.SyncedAt = &now
			result = append(result, lc)
		case CommentActionDeleteLocal:
			lc.DeletedAt = &now
			result = append(result, lc)
		case CommentActionDeleteRemote:
			if err := api.DeleteComment(ctx, *lc.GitHubCommentID); err != nil {
				return nil, err
			}
			// tombstone fully propagated: drop from result.
		case CommentActionPurgeLocal:
			// drop from result.
		}
	}

	for _, rc := range remote {
		if handled[rc.ID] {
			continue
		}
		ghID := rc.ID
		plans = append(plans, CommentPlan{GitHubCommentID: &ghID, Action: CommentActionCreateLocal})
		if dryRun {
			continue
		}
		synced := now
		result = append(result, model.Comment{
			ID:              id.NewComment(),
			GitHubCommentID: &ghID,
			Author:          rc.Author,
			Body:            rc.Body,
			CreatedAt:       rc.CreatedAt,
			UpdatedAt:       rc.UpdatedAt,
			SyncedAt:        &synced,
		})
	}

	if !dryRun {
		t.Comments = result
	}
	return plans, nil
}
