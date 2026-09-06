package service

import (
	"fmt"
	"strings"

	"github.com/ucpr/atama/internal/model"
	"github.com/ucpr/atama/internal/store"
)

// ResolveTaskID resolves a full ID or unique prefix (as displayed by `task
// list`) to a full task ID, so agents and humans don't have to type a whole
// UUID on the command line.
func (s *Service) ResolveTaskID(ref string) (string, error) {
	tasks, err := s.store.ListTasks()
	if err != nil {
		return "", fmt.Errorf("service: listing tasks: %w", err)
	}
	ids := make([]string, len(tasks))
	for i, t := range tasks {
		ids[i] = t.ID
	}
	return resolvePrefix(ref, ids, "task")
}

// ResolveGoalID resolves a full ID or unique prefix to a full goal ID.
func (s *Service) ResolveGoalID(ref string) (string, error) {
	goals, err := s.store.ListGoals()
	if err != nil {
		return "", fmt.Errorf("service: listing goals: %w", err)
	}
	ids := make([]string, len(goals))
	for i, g := range goals {
		ids[i] = g.ID
	}
	return resolvePrefix(ref, ids, "goal")
}

func resolvePrefix(ref string, ids []string, kind string) (string, error) {
	for _, id := range ids {
		if id == ref {
			return id, nil
		}
	}
	var matches []string
	for _, id := range ids {
		if strings.HasPrefix(id, ref) {
			matches = append(matches, id)
		}
	}
	switch len(matches) {
	case 0:
		return "", fmt.Errorf("service: no %s matches %q: %w", kind, ref, store.ErrNotFound)
	case 1:
		return matches[0], nil
	default:
		return "", validationErrorf("%q matches multiple %ss: %s", ref, kind, strings.Join(matches, ", "))
	}
}

// tasksByID is a convenience for building the map depgraph functions expect.
func tasksByID(tasks []*model.Task) map[string]*model.Task {
	m := make(map[string]*model.Task, len(tasks))
	for _, t := range tasks {
		m[t.ID] = t
	}
	return m
}
