package service

import (
	"fmt"
	"path/filepath"
	"time"

	"github.com/ucpr/atama/internal/gitutil"
	"github.com/ucpr/atama/internal/model"
)

// CreateWorktreeInput overrides CreateWorktree's defaults.
type CreateWorktreeInput struct {
	// Branch defaults to "atama/<task-id>".
	Branch string
	// Dir is the worktree's full path. Defaults to a directory named after
	// the task ID under a "<repo-name>-worktrees" sibling of the repo root.
	Dir string
	// StartPoint is the git ref the new branch starts from. Defaults to
	// "HEAD".
	StartPoint string
}

// CreateWorktree gives a task its own isolated git worktree and branch, so
// a delegated agent can work on it without sharing a checkout with every
// other in-flight task. The project directory (the store's parent) must be
// inside a git repository.
func (s *Service) CreateWorktree(taskID string, in CreateWorktreeInput) (*model.Task, error) {
	t, err := s.store.GetTask(taskID)
	if err != nil {
		return nil, fmt.Errorf("service: %w", err)
	}
	if t.Execution.Worktree != nil {
		return nil, validationErrorf("task %s already has a worktree at %s", t.ID, t.Execution.Worktree.Path)
	}

	repoRoot, err := gitutil.RepoRoot(s.projectDir())
	if err != nil {
		return nil, fmt.Errorf("service: %w", err)
	}

	branch := in.Branch
	if branch == "" {
		branch = "atama/" + t.ID
	}
	dir := in.Dir
	if dir == "" {
		dir = filepath.Join(defaultWorktreeBase(repoRoot), t.ID)
	}
	startPoint := in.StartPoint
	if startPoint == "" {
		startPoint = "HEAD"
	}

	if err := gitutil.AddWorktree(repoRoot, dir, branch, startPoint); err != nil {
		return nil, fmt.Errorf("service: %w", err)
	}

	t.Execution.Worktree = &model.Worktree{Path: dir, Branch: branch, CreatedAt: time.Now().UTC()}
	t.UpdatedAt = time.Now().UTC()
	if err := s.store.UpdateTask(t); err != nil {
		return nil, fmt.Errorf("service: saving task: %w", err)
	}
	return t, nil
}

// RemoveWorktree removes a task's worktree (force discards uncommitted
// changes in it) and clears the task's worktree metadata.
func (s *Service) RemoveWorktree(taskID string, force bool) (*model.Task, error) {
	t, err := s.store.GetTask(taskID)
	if err != nil {
		return nil, fmt.Errorf("service: %w", err)
	}
	if t.Execution.Worktree == nil {
		return nil, validationErrorf("task %s has no worktree", t.ID)
	}

	repoRoot, err := gitutil.RepoRoot(s.projectDir())
	if err != nil {
		return nil, fmt.Errorf("service: %w", err)
	}
	if err := gitutil.RemoveWorktree(repoRoot, t.Execution.Worktree.Path, force); err != nil {
		return nil, fmt.Errorf("service: %w", err)
	}

	t.Execution.Worktree = nil
	t.UpdatedAt = time.Now().UTC()
	if err := s.store.UpdateTask(t); err != nil {
		return nil, fmt.Errorf("service: saving task: %w", err)
	}
	return t, nil
}

// projectDir is the directory atama was initialized in (the store's
// parent), which is what needs to be inside a git repo for worktree
// operations — the store's own directory is just ".atama" within it.
func (s *Service) projectDir() string {
	return filepath.Dir(s.store.Root())
}

func defaultWorktreeBase(repoRoot string) string {
	return filepath.Join(filepath.Dir(repoRoot), filepath.Base(repoRoot)+"-worktrees")
}
