// Package gitutil wraps the `git worktree` plumbing atama uses to give a
// delegated task its own isolated working directory and branch, so
// multiple agents can work on different tasks in the same repo
// concurrently without stepping on each other's checkout.
package gitutil

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// RepoRoot returns the top-level directory of the git repository containing
// dir (works from any subdirectory, including a nested project inside a
// monorepo).
func RepoRoot(dir string) (string, error) {
	out, err := runGit(dir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// AddWorktree creates a new worktree at path on a new branch starting from
// startPoint (e.g. "HEAD"), by running `git worktree add` with repoDir as
// the working directory (any path inside the repo works).
func AddWorktree(repoDir, path, branch, startPoint string) error {
	_, err := runGit(repoDir, "worktree", "add", "-b", branch, path, startPoint)
	return err
}

// RemoveWorktree removes the worktree at path. force passes --force,
// discarding any uncommitted changes in that worktree.
func RemoveWorktree(repoDir, path string, force bool) error {
	args := []string{"worktree", "remove"}
	if force {
		args = append(args, "--force")
	}
	args = append(args, path)
	_, err := runGit(repoDir, args...)
	return err
}

// Entry is one worktree reported by `git worktree list`.
type Entry struct {
	Path     string
	Head     string
	Branch   string // empty if detached
	Detached bool
}

// ListWorktrees returns every worktree registered against the repository
// containing repoDir.
func ListWorktrees(repoDir string) ([]Entry, error) {
	out, err := runGit(repoDir, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, err
	}
	return parseWorktreeList(out), nil
}

func parseWorktreeList(out string) []Entry {
	var entries []Entry
	var cur *Entry
	for _, line := range strings.Split(out, "\n") {
		switch {
		case strings.HasPrefix(line, "worktree "):
			if cur != nil {
				entries = append(entries, *cur)
			}
			cur = &Entry{Path: strings.TrimPrefix(line, "worktree ")}
		case strings.HasPrefix(line, "HEAD "):
			if cur != nil {
				cur.Head = strings.TrimPrefix(line, "HEAD ")
			}
		case strings.HasPrefix(line, "branch "):
			if cur != nil {
				cur.Branch = strings.TrimPrefix(strings.TrimPrefix(line, "branch "), "refs/heads/")
			}
		case line == "detached":
			if cur != nil {
				cur.Detached = true
			}
		}
	}
	if cur != nil {
		entries = append(entries, *cur)
	}
	return entries
}

func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return "", fmt.Errorf("gitutil: git %s: %s", strings.Join(args, " "), msg)
	}
	return stdout.String(), nil
}
