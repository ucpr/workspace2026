// Package store persists tasks and goals as one YAML file each under a
// `.atama` directory (requirements §7: local files, Git-friendly, one file
// per task). It is a pure persistence layer — dependency validation and
// other business rules live in package service.
package store

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/ucpr/atama/internal/model"
)

// DirName is the store directory created by Init and located by Open.
const DirName = ".atama"

// SchemaVersion is the current on-disk schema version written to
// config.yaml (requirements §7: schema version for future migrations).
const SchemaVersion = 1

// ErrNotFound is returned when a task or goal ID doesn't exist.
var ErrNotFound = errors.New("not found")

// ErrAlreadyInitialized is returned by Init when a store already exists.
var ErrAlreadyInitialized = errors.New("store already initialized")

// Config is the store's top-level configuration (config.yaml).
type Config struct {
	SchemaVersion int           `yaml:"schema_version"`
	GitHub        *GitHubConfig `yaml:"github,omitempty"`
}

// GitHubConfig names the single repository a store syncs with
// (requirements §5.4).
type GitHubConfig struct {
	Repo string `yaml:"repo"`
}

// Store is a handle to an initialized `.atama` directory.
type Store struct {
	root string // path to the .atama directory itself
}

func (s *Store) tasksDir() string   { return filepath.Join(s.root, "tasks") }
func (s *Store) goalsDir() string   { return filepath.Join(s.root, "goals") }
func (s *Store) configPath() string { return filepath.Join(s.root, "config.yaml") }
func (s *Store) lockPath() string   { return filepath.Join(s.root, ".lock") }

// Root returns the `.atama` directory path.
func (s *Store) Root() string { return s.root }

// Init creates a new store rooted at filepath.Join(dir, DirName).
func Init(dir string) (*Store, error) {
	root := filepath.Join(dir, DirName)
	if _, err := os.Stat(root); err == nil {
		return nil, ErrAlreadyInitialized
	}

	if err := os.MkdirAll(filepath.Join(root, "tasks"), 0o755); err != nil {
		return nil, fmt.Errorf("store: creating tasks dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(root, "goals"), 0o755); err != nil {
		return nil, fmt.Errorf("store: creating goals dir: %w", err)
	}

	s := &Store{root: root}
	cfg := &Config{SchemaVersion: SchemaVersion}
	if err := s.SaveConfig(cfg); err != nil {
		return nil, err
	}
	return s, nil
}

// Find walks up from dir looking for a `.atama` directory, the way git
// locates `.git`. It returns ErrNotFound if none is found before reaching
// the filesystem root.
func Find(dir string) (*Store, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, fmt.Errorf("store: resolving %q: %w", dir, err)
	}

	for {
		candidate := filepath.Join(abs, DirName)
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return &Store{root: candidate}, nil
		}
		parent := filepath.Dir(abs)
		if parent == abs {
			return nil, fmt.Errorf("store: no %s found searching upward from %q: %w", DirName, dir, ErrNotFound)
		}
		abs = parent
	}
}

// LoadConfig reads config.yaml.
func (s *Store) LoadConfig() (*Config, error) {
	var cfg Config
	if err := readYAML(s.configPath(), &cfg); err != nil {
		return nil, fmt.Errorf("store: loading config: %w", err)
	}
	return &cfg, nil
}

// SaveConfig writes config.yaml.
func (s *Store) SaveConfig(cfg *Config) error {
	return s.withLock(func() error {
		return writeYAML(s.configPath(), cfg)
	})
}

func taskPath(dir, id string) string {
	return filepath.Join(dir, sanitizeID(id)+".yaml")
}

func sanitizeID(id string) string {
	// IDs are generated internally (atm-<uuid>) but guard against path
	// traversal in case a caller passes an untrusted value through.
	return strings.ReplaceAll(strings.ReplaceAll(id, "/", "_"), "..", "_")
}

// CreateTask writes a new task file. It fails if a task with the same ID
// already exists.
func (s *Store) CreateTask(t *model.Task) error {
	return s.withLock(func() error {
		path := taskPath(s.tasksDir(), t.ID)
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("store: task %s already exists", t.ID)
		}
		return writeYAML(path, t)
	})
}

// GetTask reads a single task by ID.
func (s *Store) GetTask(id string) (*model.Task, error) {
	var t model.Task
	path := taskPath(s.tasksDir(), id)
	if err := readYAML(path, &t); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("store: task %s: %w", id, ErrNotFound)
		}
		return nil, fmt.Errorf("store: loading task %s: %w", id, err)
	}
	return &t, nil
}

// ListTasks reads every task in the store.
func (s *Store) ListTasks() ([]*model.Task, error) {
	entries, err := os.ReadDir(s.tasksDir())
	if err != nil {
		return nil, fmt.Errorf("store: listing tasks: %w", err)
	}
	tasks := make([]*model.Task, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		var t model.Task
		path := filepath.Join(s.tasksDir(), e.Name())
		if err := readYAML(path, &t); err != nil {
			return nil, fmt.Errorf("store: loading %s: %w", path, err)
		}
		tasks = append(tasks, &t)
	}
	return tasks, nil
}

// UpdateTask overwrites an existing task file. It fails if the task doesn't
// exist yet — use CreateTask for new tasks.
func (s *Store) UpdateTask(t *model.Task) error {
	return s.withLock(func() error {
		path := taskPath(s.tasksDir(), t.ID)
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("store: task %s: %w", t.ID, ErrNotFound)
		}
		return writeYAML(path, t)
	})
}

// DeleteTask removes a task file.
func (s *Store) DeleteTask(id string) error {
	return s.withLock(func() error {
		path := taskPath(s.tasksDir(), id)
		if err := os.Remove(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("store: task %s: %w", id, ErrNotFound)
			}
			return fmt.Errorf("store: deleting task %s: %w", id, err)
		}
		return nil
	})
}

// CreateGoal writes a new goal file. It fails if a goal with the same ID
// already exists.
func (s *Store) CreateGoal(g *model.Goal) error {
	return s.withLock(func() error {
		path := taskPath(s.goalsDir(), g.ID)
		if _, err := os.Stat(path); err == nil {
			return fmt.Errorf("store: goal %s already exists", g.ID)
		}
		return writeYAML(path, g)
	})
}

// GetGoal reads a single goal by ID.
func (s *Store) GetGoal(id string) (*model.Goal, error) {
	var g model.Goal
	path := taskPath(s.goalsDir(), id)
	if err := readYAML(path, &g); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("store: goal %s: %w", id, ErrNotFound)
		}
		return nil, fmt.Errorf("store: loading goal %s: %w", id, err)
	}
	return &g, nil
}

// ListGoals reads every goal in the store.
func (s *Store) ListGoals() ([]*model.Goal, error) {
	entries, err := os.ReadDir(s.goalsDir())
	if err != nil {
		return nil, fmt.Errorf("store: listing goals: %w", err)
	}
	goals := make([]*model.Goal, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".yaml") {
			continue
		}
		var g model.Goal
		path := filepath.Join(s.goalsDir(), e.Name())
		if err := readYAML(path, &g); err != nil {
			return nil, fmt.Errorf("store: loading %s: %w", path, err)
		}
		goals = append(goals, &g)
	}
	return goals, nil
}

// UpdateGoal overwrites an existing goal file.
func (s *Store) UpdateGoal(g *model.Goal) error {
	return s.withLock(func() error {
		path := taskPath(s.goalsDir(), g.ID)
		if _, err := os.Stat(path); err != nil {
			return fmt.Errorf("store: goal %s: %w", g.ID, ErrNotFound)
		}
		return writeYAML(path, g)
	})
}

// DeleteGoal removes a goal file.
func (s *Store) DeleteGoal(id string) error {
	return s.withLock(func() error {
		path := taskPath(s.goalsDir(), id)
		if err := os.Remove(path); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				return fmt.Errorf("store: goal %s: %w", id, ErrNotFound)
			}
			return fmt.Errorf("store: deleting goal %s: %w", id, err)
		}
		return nil
	})
}

func readYAML(path string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, v)
}

// writeYAML writes v to path atomically (write to a temp file, then
// rename) so readers never observe a partially written file.
func writeYAML(path string, v any) error {
	data, err := yaml.Marshal(v)
	if err != nil {
		return fmt.Errorf("store: marshaling %s: %w", path, err)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return fmt.Errorf("store: writing %s: %w", tmp, err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("store: renaming %s to %s: %w", tmp, path, err)
	}
	return nil
}
