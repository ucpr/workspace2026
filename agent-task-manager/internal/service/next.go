package service

import (
	"fmt"

	"github.com/ucpr/atama/internal/depgraph"
	"github.com/ucpr/atama/internal/model"
)

// Next returns the Ready tasks (requirements §5.2, §5.3), in dependency
// order. If goalID is empty, every task in the store is considered;
// otherwise only tasks belonging to that goal are.
func (s *Service) Next(goalID string) ([]*model.Task, error) {
	all, err := s.store.ListTasks()
	if err != nil {
		return nil, fmt.Errorf("service: listing tasks: %w", err)
	}
	tasks := tasksByID(all)

	var ids []string
	if goalID == "" {
		for id := range tasks {
			ids = append(ids, id)
		}
	} else {
		g, err := s.store.GetGoal(goalID)
		if err != nil {
			return nil, fmt.Errorf("service: %w", err)
		}
		ids = g.TaskIDs
	}

	ready, err := depgraph.ReadyTasks(tasks, ids)
	if err != nil {
		return nil, fmt.Errorf("service: %w", err)
	}
	return ready, nil
}
