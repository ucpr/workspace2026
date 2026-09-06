package store

import (
	"errors"
	"os"
	"testing"
	"time"

	"github.com/ucpr/atama/internal/model"
)

func TestInit_CreatesLayout(t *testing.T) {
	dir := t.TempDir()
	s, err := Init(dir)
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	cfg, err := s.LoadConfig()
	if err != nil {
		t.Fatalf("LoadConfig() error = %v", err)
	}
	if cfg.SchemaVersion != SchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", cfg.SchemaVersion, SchemaVersion)
	}
}

func TestInit_AlreadyInitialized(t *testing.T) {
	dir := t.TempDir()
	if _, err := Init(dir); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	if _, err := Init(dir); !errors.Is(err, ErrAlreadyInitialized) {
		t.Errorf("Init() second time error = %v, want ErrAlreadyInitialized", err)
	}
}

func TestFind_WalksUpward(t *testing.T) {
	root := t.TempDir()
	if _, err := Init(root); err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	sub := root + "/a/b/c"
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("os.MkdirAll() error = %v", err)
	}
	s, err := Find(sub)
	if err != nil {
		t.Fatalf("Find() error = %v", err)
	}
	if s.Root() == "" {
		t.Error("Find() returned store with empty root")
	}
}

func TestFind_NotFound(t *testing.T) {
	dir := t.TempDir()
	if _, err := Find(dir); !errors.Is(err, ErrNotFound) {
		t.Errorf("Find() error = %v, want ErrNotFound", err)
	}
}

func TestTaskCRUD(t *testing.T) {
	s := newTestStore(t)

	task := &model.Task{
		ID:        "atm-test-1",
		Title:     "first task",
		Status:    model.StatusBacklog,
		Priority:  model.PriorityMedium,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.CreateTask(task); err != nil {
		t.Fatalf("CreateTask() error = %v", err)
	}
	if err := s.CreateTask(task); err == nil {
		t.Error("CreateTask() duplicate = nil error, want error")
	}

	got, err := s.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if got.Title != task.Title {
		t.Errorf("GetTask().Title = %q, want %q", got.Title, task.Title)
	}

	got.Title = "updated title"
	if err := s.UpdateTask(got); err != nil {
		t.Fatalf("UpdateTask() error = %v", err)
	}
	reloaded, err := s.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask() error = %v", err)
	}
	if reloaded.Title != "updated title" {
		t.Errorf("GetTask().Title after update = %q, want %q", reloaded.Title, "updated title")
	}

	list, err := s.ListTasks()
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("ListTasks() len = %d, want 1", len(list))
	}

	if err := s.DeleteTask(task.ID); err != nil {
		t.Fatalf("DeleteTask() error = %v", err)
	}
	if _, err := s.GetTask(task.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetTask() after delete error = %v, want ErrNotFound", err)
	}
}

func TestGoalCRUD(t *testing.T) {
	s := newTestStore(t)

	goal := &model.Goal{
		ID:        "atm-g-test-1",
		Name:      "ship v1",
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := s.CreateGoal(goal); err != nil {
		t.Fatalf("CreateGoal() error = %v", err)
	}

	got, err := s.GetGoal(goal.ID)
	if err != nil {
		t.Fatalf("GetGoal() error = %v", err)
	}
	if got.Name != goal.Name {
		t.Errorf("GetGoal().Name = %q, want %q", got.Name, goal.Name)
	}

	got.Name = "ship v2"
	if err := s.UpdateGoal(got); err != nil {
		t.Fatalf("UpdateGoal() error = %v", err)
	}

	list, err := s.ListGoals()
	if err != nil {
		t.Fatalf("ListGoals() error = %v", err)
	}
	if len(list) != 1 || list[0].Name != "ship v2" {
		t.Fatalf("ListGoals() = %+v, want single updated goal", list)
	}

	if err := s.DeleteGoal(goal.ID); err != nil {
		t.Fatalf("DeleteGoal() error = %v", err)
	}
	if _, err := s.GetGoal(goal.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("GetGoal() after delete error = %v, want ErrNotFound", err)
	}
}

func newTestStore(t *testing.T) *Store {
	t.Helper()
	s, err := Init(t.TempDir())
	if err != nil {
		t.Fatalf("Init() error = %v", err)
	}
	return s
}
