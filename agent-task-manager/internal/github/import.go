package github

import (
	"context"
	"fmt"
	"time"

	"github.com/ucpr/atama/internal/id"
	"github.com/ucpr/atama/internal/model"
	"github.com/ucpr/atama/internal/store"
)

// ImportOptions controls how already-imported issues are handled
// (requirements §5.4: "既存タスクとの重複...は上書きかスキップを選択できる").
type ImportOptions struct {
	// Overwrite, if true, refreshes local fields from GitHub for issues
	// already linked to a task. If false (default), such issues are
	// skipped.
	Overwrite bool
}

// ImportResult summarizes what Import did.
type ImportResult struct {
	Created []*model.Task `json:"created,omitempty"`
	Updated []*model.Task `json:"updated,omitempty"`
	Skipped []int         `json:"skipped_issue_numbers,omitempty"`
}

// Import fetches every Issue in repo and creates or updates local tasks
// keyed by Issue number (requirements §5.4).
func Import(ctx context.Context, st *store.Store, api API, repo string, opts ImportOptions) (*ImportResult, error) {
	issues, err := api.ListIssues(ctx)
	if err != nil {
		return nil, err
	}

	result := &ImportResult{}
	now := time.Now().UTC()
	for _, issue := range issues {
		existing, err := findTaskByIssueNumber(st, repo, issue.Number)
		if err != nil {
			return nil, err
		}

		if existing == nil {
			body, meta := stripMeta(issue.Body)
			task := &model.Task{
				ID:        id.NewTask(),
				Title:     issue.Title,
				Body:      body,
				Status:    mapIssueStateToStatus(issue.State, model.StatusBacklog),
				Priority:  model.PriorityMedium,
				Labels:    issue.Labels,
				Assignees: issue.Assignees,
				CreatedAt: issue.CreatedAt,
				UpdatedAt: now,
				GitHub: &model.GitHubMeta{
					Repo:            repo,
					IssueNumber:     issue.Number,
					URL:             issue.URL,
					SyncedAt:        now,
					RemoteUpdatedAt: issue.UpdatedAt,
				},
			}
			if meta != nil {
				task.DependsOn = meta.DependsOn
				task.GoalIDs = meta.GoalIDs
			}
			if err := st.CreateTask(task); err != nil {
				return nil, fmt.Errorf("github: creating task for issue #%d: %w", issue.Number, err)
			}
			result.Created = append(result.Created, task)
			continue
		}

		if !opts.Overwrite {
			result.Skipped = append(result.Skipped, issue.Number)
			continue
		}

		body, meta := stripMeta(issue.Body)
		existing.Title = issue.Title
		existing.Body = body
		existing.Labels = issue.Labels
		existing.Assignees = issue.Assignees
		existing.Status = mapIssueStateToStatus(issue.State, existing.Status)
		if meta != nil {
			existing.DependsOn = meta.DependsOn
			existing.GoalIDs = meta.GoalIDs
		}
		existing.GitHub.URL = issue.URL
		existing.GitHub.SyncedAt = now
		existing.GitHub.RemoteUpdatedAt = issue.UpdatedAt
		existing.UpdatedAt = now
		if err := st.UpdateTask(existing); err != nil {
			return nil, fmt.Errorf("github: updating task for issue #%d: %w", issue.Number, err)
		}
		result.Updated = append(result.Updated, existing)
	}
	return result, nil
}
