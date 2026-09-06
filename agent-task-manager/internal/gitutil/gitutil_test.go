package gitutil

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// initRepo creates a git repo with one commit in a fresh temp dir, isolated
// from the developer's global git config (author identity comes from env
// vars so tests don't depend on `git config --global` being set).
func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run(t, dir, "init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("hello\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	run(t, dir, "add", "README.md")
	run(t, dir, "commit", "-q", "-m", "init")
	return dir
}

func run(t *testing.T, dir string, args ...string) {
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

func TestRepoRoot(t *testing.T) {
	dir := initRepo(t)
	sub := filepath.Join(dir, "nested", "project")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	root, err := RepoRoot(sub)
	if err != nil {
		t.Fatalf("RepoRoot() error = %v", err)
	}
	// Resolve symlinks (e.g. macOS /tmp -> /private/tmp) before comparing.
	wantResolved, _ := filepath.EvalSymlinks(dir)
	gotResolved, _ := filepath.EvalSymlinks(root)
	if gotResolved != wantResolved {
		t.Errorf("RepoRoot() = %q, want %q", gotResolved, wantResolved)
	}
}

func TestAddListRemoveWorktree(t *testing.T) {
	repo := initRepo(t)
	wtPath := filepath.Join(t.TempDir(), "wt1")

	if err := AddWorktree(repo, wtPath, "atama/test-task", "HEAD"); err != nil {
		t.Fatalf("AddWorktree() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(wtPath, "README.md")); err != nil {
		t.Fatalf("worktree checkout missing README.md: %v", err)
	}

	entries, err := ListWorktrees(repo)
	if err != nil {
		t.Fatalf("ListWorktrees() error = %v", err)
	}
	found := false
	for _, e := range entries {
		resolved, _ := filepath.EvalSymlinks(e.Path)
		wantResolved, _ := filepath.EvalSymlinks(wtPath)
		if resolved == wantResolved {
			found = true
			if e.Branch != "atama/test-task" {
				t.Errorf("entry.Branch = %q, want %q", e.Branch, "atama/test-task")
			}
		}
	}
	if !found {
		t.Fatalf("ListWorktrees() = %+v, want an entry for %s", entries, wtPath)
	}

	if err := RemoveWorktree(repo, wtPath, false); err != nil {
		t.Fatalf("RemoveWorktree() error = %v", err)
	}
	if _, err := os.Stat(wtPath); !os.IsNotExist(err) {
		t.Errorf("worktree dir still exists after RemoveWorktree(): err = %v", err)
	}
}

func TestAddWorktree_DuplicateBranchFails(t *testing.T) {
	repo := initRepo(t)
	wt1 := filepath.Join(t.TempDir(), "wt1")
	wt2 := filepath.Join(t.TempDir(), "wt2")

	if err := AddWorktree(repo, wt1, "atama/dup", "HEAD"); err != nil {
		t.Fatalf("AddWorktree() error = %v", err)
	}
	if err := AddWorktree(repo, wt2, "atama/dup", "HEAD"); err == nil {
		t.Error("AddWorktree() with a branch already checked out = nil error, want error")
	}
}
