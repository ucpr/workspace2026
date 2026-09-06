// Package model defines the core data types persisted by atama: tasks and
// goals, along with their embedded GitHub sync and execution metadata.
package model

import "time"

// Status is a task's workflow state. Transitions between statuses are
// unconstrained (see requirements §5.1); any status may follow any other.
type Status string

// Task statuses.
const (
	StatusBacklog    Status = "backlog"
	StatusReady      Status = "ready"
	StatusInProgress Status = "in_progress"
	StatusBlocked    Status = "blocked"
	StatusInReview   Status = "in_review"
	StatusDone       Status = "done"
	StatusCancelled  Status = "cancelled"
)

// Valid reports whether s is one of the known statuses.
func (s Status) Valid() bool {
	switch s {
	case StatusBacklog, StatusReady, StatusInProgress, StatusBlocked, StatusInReview, StatusDone, StatusCancelled:
		return true
	default:
		return false
	}
}

// Terminal reports whether s represents a completed task that no longer
// blocks dependents once reached (done) or should not (cancelled per the
// default policy).
func (s Status) Terminal() bool {
	return s == StatusDone || s == StatusCancelled
}

// Priority is a task's fixed-enum priority level.
type Priority string

// Task priorities.
const (
	PriorityLow    Priority = "low"
	PriorityMedium Priority = "medium"
	PriorityHigh   Priority = "high"
	PriorityUrgent Priority = "urgent"
)

// Valid reports whether p is one of the known priorities.
func (p Priority) Valid() bool {
	switch p {
	case PriorityLow, PriorityMedium, PriorityHigh, PriorityUrgent:
		return true
	default:
		return false
	}
}

// Task is a unit of work, roughly corresponding to a GitHub Issue plus
// dependency and execution metadata (requirements §6).
type Task struct {
	ID        string     `yaml:"id"`
	Title     string     `yaml:"title"`
	Body      string     `yaml:"body"`
	Status    Status     `yaml:"status"`
	Priority  Priority   `yaml:"priority"`
	Labels    []string   `yaml:"labels,omitempty"`
	Assignees []string   `yaml:"assignees,omitempty"`
	DependsOn []string   `yaml:"depends_on,omitempty"`
	GoalIDs   []string   `yaml:"goal_ids,omitempty"`
	CreatedAt time.Time  `yaml:"created_at"`
	UpdatedAt time.Time  `yaml:"updated_at"`
	ClosedAt  *time.Time `yaml:"closed_at,omitempty"`

	GitHub    *GitHubMeta `yaml:"github,omitempty"`
	Comments  []Comment   `yaml:"comments,omitempty"`
	Execution Execution   `yaml:"execution,omitempty"`
}

// GitHubMeta records the GitHub Issue a task is linked to, once exported or
// imported (requirements §5.4, §6).
type GitHubMeta struct {
	Repo            string    `yaml:"repo"`
	IssueNumber     int       `yaml:"issue_number"`
	URL             string    `yaml:"url"`
	SyncedAt        time.Time `yaml:"synced_at"`
	RemoteUpdatedAt time.Time `yaml:"remote_updated_at"`
}

// Comment is a GitHub Issue comment kept in bidirectional sync
// (requirements §5.4).
type Comment struct {
	ID              string     `yaml:"id"`
	GitHubCommentID *int64     `yaml:"github_comment_id"`
	Author          string     `yaml:"author"`
	Body            string     `yaml:"body"`
	CreatedAt       time.Time  `yaml:"created_at"`
	UpdatedAt       time.Time  `yaml:"updated_at"`
	SyncedAt        *time.Time `yaml:"synced_at,omitempty"`
	DeletedAt       *time.Time `yaml:"deleted_at,omitempty"`
}

// Execution is reserved for the future executor-hook extension
// (requirements §5.3, §8.3): v0.1 only stores and passes through this data,
// it never launches anything.
type Execution struct {
	Executor string   `yaml:"executor,omitempty"`
	LastRun  *RunInfo `yaml:"last_run,omitempty"`
}

// RunInfo records the outcome of an executor invocation.
type RunInfo struct {
	StartedAt  *time.Time `yaml:"started_at,omitempty"`
	FinishedAt *time.Time `yaml:"finished_at,omitempty"`
	ExitCode   *int       `yaml:"exit_code,omitempty"`
	LogPath    string     `yaml:"log_path,omitempty"`
}
