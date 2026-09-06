package service

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/ucpr/atama/internal/store"
)

// newGitBackedTestService inits a git repo and an atama store in the same
// temp directory, since worktree operations require the store's parent to
// be inside a git repo.
func newGitBackedTestService(t *testing.T) *Service {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-q", "-m", "init")

	st, err := store.Init(dir)
	if err != nil {
		t.Fatalf("store.Init() error = %v", err)
	}
	return New(st)
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}

func TestCreateWorktree(t *testing.T) {
	svc := newGitBackedTestService(t)
	task, err := svc.AddTask(AddTaskInput{Title: "do the thing"})
	if err != nil {
		t.Fatalf("AddTask() error = %v", err)
	}

	updated, err := svc.CreateWorktree(task.ID, CreateWorktreeInput{})
	if err != nil {
		t.Fatalf("CreateWorktree() error = %v", err)
	}
	if updated.Execution.Worktree == nil {
		t.Fatal("Execution.Worktree = nil, want set")
	}
	if updated.Execution.Worktree.Branch != "atama/"+task.ID {
		t.Errorf("Branch = %q, want %q", updated.Execution.Worktree.Branch, "atama/"+task.ID)
	}
	if _, err := os.Stat(filepath.Join(updated.Execution.Worktree.Path, "README.md")); err != nil {
		t.Errorf("worktree checkout missing README.md: %v", err)
	}

	if _, err := svc.CreateWorktree(task.ID, CreateWorktreeInput{}); err == nil {
		t.Error("CreateWorktree() a second time = nil error, want error (already has a worktree)")
	}
}

func TestCreateWorktree_CustomOptions(t *testing.T) {
	svc := newGitBackedTestService(t)
	task, _ := svc.AddTask(AddTaskInput{Title: "custom"})

	customDir := filepath.Join(t.TempDir(), "my-worktree")
	updated, err := svc.CreateWorktree(task.ID, CreateWorktreeInput{Branch: "feature/custom", Dir: customDir})
	if err != nil {
		t.Fatalf("CreateWorktree() error = %v", err)
	}
	if updated.Execution.Worktree.Branch != "feature/custom" {
		t.Errorf("Branch = %q, want feature/custom", updated.Execution.Worktree.Branch)
	}
	if updated.Execution.Worktree.Path != customDir {
		t.Errorf("Path = %q, want %q", updated.Execution.Worktree.Path, customDir)
	}
}

func TestRemoveWorktree(t *testing.T) {
	svc := newGitBackedTestService(t)
	task, _ := svc.AddTask(AddTaskInput{Title: "temp"})
	created, err := svc.CreateWorktree(task.ID, CreateWorktreeInput{})
	if err != nil {
		t.Fatalf("CreateWorktree() error = %v", err)
	}
	path := created.Execution.Worktree.Path

	updated, err := svc.RemoveWorktree(task.ID, false)
	if err != nil {
		t.Fatalf("RemoveWorktree() error = %v", err)
	}
	if updated.Execution.Worktree != nil {
		t.Error("Execution.Worktree still set after RemoveWorktree()")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("worktree dir still exists after RemoveWorktree(): err = %v", err)
	}

	if _, err := svc.RemoveWorktree(task.ID, false); err == nil {
		t.Error("RemoveWorktree() with no worktree = nil error, want error")
	}
}
