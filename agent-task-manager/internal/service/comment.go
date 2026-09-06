package service

import (
	"fmt"
	"time"

	"github.com/ucpr/atama/internal/id"
	"github.com/ucpr/atama/internal/model"
)

// AddComment appends a local-only comment to a task (github_comment_id is
// left nil until a later `github sync` pushes it, per requirements §5.4).
func (s *Service) AddComment(taskID, author, body string) (*model.Task, error) {
	if body == "" {
		return nil, validationErrorf("comment body cannot be empty")
	}
	t, err := s.store.GetTask(taskID)
	if err != nil {
		return nil, fmt.Errorf("service: %w", err)
	}
	now := time.Now().UTC()
	t.Comments = append(t.Comments, model.Comment{
		ID:        id.NewComment(),
		Author:    author,
		Body:      body,
		CreatedAt: now,
		UpdatedAt: now,
	})
	t.UpdatedAt = now
	if err := s.store.UpdateTask(t); err != nil {
		return nil, fmt.Errorf("service: updating task: %w", err)
	}
	return t, nil
}
