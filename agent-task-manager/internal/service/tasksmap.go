package service

import (
	"fmt"

	"github.com/ucpr/atama/internal/model"
)

// TasksMap returns every task in the store, keyed by ID. Callers (CLI, TUI)
// use it together with package depgraph to compute derived status and
// readiness without duplicating store access.
func (s *Service) TasksMap() (map[string]*model.Task, error) {
	all, err := s.store.ListTasks()
	if err != nil {
		return nil, fmt.Errorf("service: listing tasks: %w", err)
	}
	return tasksByID(all), nil
}
