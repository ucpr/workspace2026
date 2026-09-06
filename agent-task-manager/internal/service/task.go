package service

import (
	"fmt"
	"sort"
	"time"

	"github.com/ucpr/atama/internal/depgraph"
	"github.com/ucpr/atama/internal/id"
	"github.com/ucpr/atama/internal/model"
	"github.com/ucpr/atama/internal/store"
)

// AddTaskInput describes a new task to create.
type AddTaskInput struct {
	Title     string
	Body      string
	Priority  model.Priority // defaults to PriorityMedium if empty
	Status    model.Status   // defaults to StatusBacklog if empty
	Labels    []string
	Assignees []string
	DependsOn []string // task IDs, already resolved to full IDs
	GoalIDs   []string // goal IDs, already resolved to full IDs
}

// AddTask creates and persists a new task.
func (s *Service) AddTask(in AddTaskInput) (*model.Task, error) {
	if in.Title == "" {
		return nil, validationErrorf("title is required")
	}
	priority := in.Priority
	if priority == "" {
		priority = model.PriorityMedium
	}
	if !priority.Valid() {
		return nil, validationErrorf("invalid priority %q", priority)
	}
	status := in.Status
	if status == "" {
		status = model.StatusBacklog
	}
	if !status.Valid() {
		return nil, validationErrorf("invalid status %q", status)
	}

	existing, err := s.store.ListTasks()
	if err != nil {
		return nil, fmt.Errorf("service: listing tasks: %w", err)
	}
	tasks := tasksByID(existing)
	for _, dep := range in.DependsOn {
		if _, ok := tasks[dep]; !ok {
			return nil, validationErrorf("depends_on references unknown task %q", dep)
		}
	}

	now := time.Now().UTC()
	task := &model.Task{
		ID:        id.NewTask(),
		Title:     in.Title,
		Body:      in.Body,
		Status:    status,
		Priority:  priority,
		Labels:    in.Labels,
		Assignees: in.Assignees,
		DependsOn: in.DependsOn,
		GoalIDs:   in.GoalIDs,
		CreatedAt: now,
		UpdatedAt: now,
	}

	tasks[task.ID] = task
	if cyc := depgraph.FindCycle(tasks); cyc != nil {
		return nil, validationErrorf("%s", cyc.Error())
	}

	if err := s.store.CreateTask(task); err != nil {
		return nil, fmt.Errorf("service: creating task: %w", err)
	}
	if err := s.addTaskToGoals(task.ID, in.GoalIDs); err != nil {
		return nil, err
	}
	return task, nil
}

// GetTask returns a single task by full ID.
func (s *Service) GetTask(taskID string) (*model.Task, error) {
	t, err := s.store.GetTask(taskID)
	if err != nil {
		return nil, fmt.Errorf("service: %w", err)
	}
	return t, nil
}

// TaskFilter narrows ListTasks results. Zero-value fields are unfiltered.
type TaskFilter struct {
	Status model.Status
	Label  string
	GoalID string
}

// ListTasks returns tasks matching filter, sorted by ID (which sorts by
// creation time; see package id).
func (s *Service) ListTasks(filter TaskFilter) ([]*model.Task, error) {
	all, err := s.store.ListTasks()
	if err != nil {
		return nil, fmt.Errorf("service: listing tasks: %w", err)
	}
	sortTasksByID(all)

	if filter.Status == "" && filter.Label == "" && filter.GoalID == "" {
		return all, nil
	}

	var out []*model.Task
	for _, t := range all {
		if filter.Status != "" && t.Status != filter.Status {
			continue
		}
		if filter.Label != "" && !containsString(t.Labels, filter.Label) {
			continue
		}
		if filter.GoalID != "" && !containsString(t.GoalIDs, filter.GoalID) {
			continue
		}
		out = append(out, t)
	}
	return out, nil
}

// EditTaskInput carries optional field updates; nil fields are left
// unchanged.
type EditTaskInput struct {
	Title     *string
	Body      *string
	Priority  *model.Priority
	Labels    *[]string
	Assignees *[]string
}

// EditTask applies a partial update to an existing task.
func (s *Service) EditTask(taskID string, in EditTaskInput) (*model.Task, error) {
	t, err := s.store.GetTask(taskID)
	if err != nil {
		return nil, fmt.Errorf("service: %w", err)
	}
	if in.Title != nil {
		if *in.Title == "" {
			return nil, validationErrorf("title cannot be empty")
		}
		t.Title = *in.Title
	}
	if in.Body != nil {
		t.Body = *in.Body
	}
	if in.Priority != nil {
		if !in.Priority.Valid() {
			return nil, validationErrorf("invalid priority %q", *in.Priority)
		}
		t.Priority = *in.Priority
	}
	if in.Labels != nil {
		t.Labels = *in.Labels
	}
	if in.Assignees != nil {
		t.Assignees = *in.Assignees
	}
	t.UpdatedAt = time.Now().UTC()

	if err := s.store.UpdateTask(t); err != nil {
		return nil, fmt.Errorf("service: updating task: %w", err)
	}
	return t, nil
}

// SetStatus changes a task's status. Transitions are unrestricted
// (requirements §5.1). Reaching StatusDone or StatusCancelled stamps
// ClosedAt; leaving them clears it.
func (s *Service) SetStatus(taskID string, status model.Status) (*model.Task, error) {
	if !status.Valid() {
		return nil, validationErrorf("invalid status %q", status)
	}
	t, err := s.store.GetTask(taskID)
	if err != nil {
		return nil, fmt.Errorf("service: %w", err)
	}
	t.Status = status
	now := time.Now().UTC()
	if status.Terminal() {
		t.ClosedAt = &now
	} else {
		t.ClosedAt = nil
	}
	t.UpdatedAt = now

	if err := s.store.UpdateTask(t); err != nil {
		return nil, fmt.Errorf("service: updating task: %w", err)
	}
	return t, nil
}

// Complete marks a task done. It's the counterpart to Next in the
// delegated-execution loop (requirements §5.3, §8.2).
func (s *Service) Complete(taskID string) (*model.Task, error) {
	return s.SetStatus(taskID, model.StatusDone)
}

// RemoveTask deletes a task and clears references to it from goals and
// from other tasks' depends_on lists.
func (s *Service) RemoveTask(taskID string) error {
	t, err := s.store.GetTask(taskID)
	if err != nil {
		return fmt.Errorf("service: %w", err)
	}

	for _, goalID := range t.GoalIDs {
		g, err := s.store.GetGoal(goalID)
		if err != nil {
			continue
		}
		g.TaskIDs = removeString(g.TaskIDs, taskID)
		g.UpdatedAt = time.Now().UTC()
		if err := s.store.UpdateGoal(g); err != nil {
			return fmt.Errorf("service: updating goal %s: %w", goalID, err)
		}
	}

	all, err := s.store.ListTasks()
	if err != nil {
		return fmt.Errorf("service: listing tasks: %w", err)
	}
	for _, other := range all {
		if !containsString(other.DependsOn, taskID) {
			continue
		}
		other.DependsOn = removeString(other.DependsOn, taskID)
		other.UpdatedAt = time.Now().UTC()
		if err := s.store.UpdateTask(other); err != nil {
			return fmt.Errorf("service: updating task %s: %w", other.ID, err)
		}
	}

	if err := s.store.DeleteTask(taskID); err != nil {
		return fmt.Errorf("service: %w", err)
	}
	return nil
}

// AddDependency records that fromID depends on onID, rejecting the change
// if it would introduce a cycle (requirements §5.2).
func (s *Service) AddDependency(fromID, onID string) (*model.Task, error) {
	if fromID == onID {
		return nil, validationErrorf("a task cannot depend on itself")
	}
	all, err := s.store.ListTasks()
	if err != nil {
		return nil, fmt.Errorf("service: listing tasks: %w", err)
	}
	tasks := tasksByID(all)
	from, ok := tasks[fromID]
	if !ok {
		return nil, fmt.Errorf("service: task %s: %w", fromID, store.ErrNotFound)
	}
	if _, ok := tasks[onID]; !ok {
		return nil, fmt.Errorf("service: task %s: %w", onID, store.ErrNotFound)
	}
	if containsString(from.DependsOn, onID) {
		return from, nil
	}
	if depgraph.WouldCreateCycle(tasks, fromID, onID) {
		return nil, validationErrorf("adding dependency %s -> %s would create a cycle", fromID, onID)
	}

	from.DependsOn = append(from.DependsOn, onID)
	from.UpdatedAt = time.Now().UTC()
	if err := s.store.UpdateTask(from); err != nil {
		return nil, fmt.Errorf("service: updating task: %w", err)
	}
	return from, nil
}

// RemoveDependency removes the depends_on edge fromID -> onID, if present.
func (s *Service) RemoveDependency(fromID, onID string) (*model.Task, error) {
	from, err := s.store.GetTask(fromID)
	if err != nil {
		return nil, fmt.Errorf("service: %w", err)
	}
	from.DependsOn = removeString(from.DependsOn, onID)
	from.UpdatedAt = time.Now().UTC()
	if err := s.store.UpdateTask(from); err != nil {
		return nil, fmt.Errorf("service: updating task: %w", err)
	}
	return from, nil
}

func containsString(s []string, v string) bool {
	for _, x := range s {
		if x == v {
			return true
		}
	}
	return false
}

func removeString(s []string, v string) []string {
	out := s[:0:0]
	for _, x := range s {
		if x != v {
			out = append(out, x)
		}
	}
	return out
}

func sortTasksByID(tasks []*model.Task) {
	sort.Slice(tasks, func(i, j int) bool { return tasks[i].ID < tasks[j].ID })
}
