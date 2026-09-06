package github

import (
	"context"
	"fmt"
	"time"

	"github.com/ucpr/atama/internal/model"
	"github.com/ucpr/atama/internal/store"
)

// ErrAlreadyExported is returned by Export when the task already has linked
// GitHub metadata.
var ErrAlreadyExported = fmt.Errorf("github: task already exported")

// Export creates a new Issue in repo from a local task and links the task
// to it (requirements §5.4). Fields with no natural Issue representation
// (depends_on, goal_ids) are embedded in the Issue body as an invisible
// HTML comment block so a later Import round-trips them.
func Export(ctx context.Context, st *store.Store, api API, repo, taskID string) (*model.Task, error) {
	task, err := st.GetTask(taskID)
	if err != nil {
		return nil, fmt.Errorf("github: %w", err)
	}
	if task.GitHub != nil {
		return nil, fmt.Errorf("%w: %s (issue #%d)", ErrAlreadyExported, task.ID, task.GitHub.IssueNumber)
	}

	issue, err := api.CreateIssue(ctx, buildIssueInput(task))
	if err != nil {
		return nil, fmt.Errorf("github: creating issue for task %s: %w", task.ID, err)
	}

	now := time.Now().UTC()
	task.GitHub = &model.GitHubMeta{
		Repo:            repo,
		IssueNumber:     issue.Number,
		URL:             issue.URL,
		SyncedAt:        now,
		RemoteUpdatedAt: issue.UpdatedAt,
	}
	task.UpdatedAt = now
	if err := st.UpdateTask(task); err != nil {
		return nil, fmt.Errorf("github: saving task %s: %w", task.ID, err)
	}
	return task, nil
}

// buildIssueInput renders task as the body/labels/state GitHub expects,
// appending the atama metadata block.
func buildIssueInput(t *model.Task) IssueInput {
	body := appendMeta(t.Body, metaBlock{
		AtamaID:   t.ID,
		DependsOn: t.DependsOn,
		GoalIDs:   t.GoalIDs,
	})
	return IssueInput{
		Title:     t.Title,
		Body:      body,
		State:     stateForStatus(t.Status),
		Labels:    t.Labels,
		Assignees: t.Assignees,
	}
}
