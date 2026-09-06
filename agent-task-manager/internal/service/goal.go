package service

import (
	"fmt"
	"sort"
	"time"

	"github.com/ucpr/atama/internal/id"
	"github.com/ucpr/atama/internal/model"
)

// AddGoalInput describes a new goal to create.
type AddGoalInput struct {
	Name        string
	Description string
	TaskIDs     []string // already resolved to full task IDs
}

// AddGoal creates and persists a new goal, and back-fills goal_ids on each
// referenced task.
func (s *Service) AddGoal(in AddGoalInput) (*model.Goal, error) {
	if in.Name == "" {
		return nil, validationErrorf("name is required")
	}
	for _, taskID := range in.TaskIDs {
		if _, err := s.store.GetTask(taskID); err != nil {
			return nil, validationErrorf("task_ids references unknown task %q", taskID)
		}
	}

	now := time.Now().UTC()
	goal := &model.Goal{
		ID:          id.NewGoal(),
		Name:        in.Name,
		Description: in.Description,
		TaskIDs:     in.TaskIDs,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.store.CreateGoal(goal); err != nil {
		return nil, fmt.Errorf("service: creating goal: %w", err)
	}
	for _, taskID := range in.TaskIDs {
		t, err := s.store.GetTask(taskID)
		if err != nil {
			continue
		}
		if !containsString(t.GoalIDs, goal.ID) {
			t.GoalIDs = append(t.GoalIDs, goal.ID)
			t.UpdatedAt = now
			if err := s.store.UpdateTask(t); err != nil {
				return nil, fmt.Errorf("service: updating task %s: %w", taskID, err)
			}
		}
	}
	return goal, nil
}

// GetGoal returns a single goal by full ID.
func (s *Service) GetGoal(goalID string) (*model.Goal, error) {
	g, err := s.store.GetGoal(goalID)
	if err != nil {
		return nil, fmt.Errorf("service: %w", err)
	}
	return g, nil
}

// ListGoals returns every goal, sorted by ID (creation order).
func (s *Service) ListGoals() ([]*model.Goal, error) {
	goals, err := s.store.ListGoals()
	if err != nil {
		return nil, fmt.Errorf("service: listing goals: %w", err)
	}
	sort.Slice(goals, func(i, j int) bool { return goals[i].ID < goals[j].ID })
	return goals, nil
}

// addTaskToGoals appends taskID to each goal's task_ids, used when a task is
// created with goal_ids already set.
func (s *Service) addTaskToGoals(taskID string, goalIDs []string) error {
	now := time.Now().UTC()
	for _, goalID := range goalIDs {
		g, err := s.store.GetGoal(goalID)
		if err != nil {
			return validationErrorf("goal_ids references unknown goal %q", goalID)
		}
		if !containsString(g.TaskIDs, taskID) {
			g.TaskIDs = append(g.TaskIDs, taskID)
			g.UpdatedAt = now
			if err := s.store.UpdateGoal(g); err != nil {
				return fmt.Errorf("service: updating goal %s: %w", goalID, err)
			}
		}
	}
	return nil
}
