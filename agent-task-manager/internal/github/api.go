// Package github implements import, export, and bidirectional sync between
// local tasks and a single GitHub repository's Issues (requirements §5.4).
//
// All GitHub access goes through the API interface so the sync logic can be
// tested against an in-memory fake instead of the network.
package github

import (
	"context"
	"time"
)

// Issue is the subset of a GitHub Issue that atama maps to a task.
type Issue struct {
	Number    int
	Title     string
	Body      string
	State     string // "open" or "closed"
	Labels    []string
	Assignees []string
	URL       string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// IssueInput is the writable subset of Issue used for create/update calls.
type IssueInput struct {
	Title     string
	Body      string
	State     string // "open" or "closed"
	Labels    []string
	Assignees []string
}

// Comment is a single Issue comment.
type Comment struct {
	ID        int64
	Author    string
	Body      string
	CreatedAt time.Time
	UpdatedAt time.Time
}

// API is a single repository's Issues surface. Implementations must scope
// every call to the repository they were constructed for.
type API interface {
	ListIssues(ctx context.Context) ([]*Issue, error)
	GetIssue(ctx context.Context, number int) (*Issue, error)
	CreateIssue(ctx context.Context, in IssueInput) (*Issue, error)
	UpdateIssue(ctx context.Context, number int, in IssueInput) (*Issue, error)

	ListComments(ctx context.Context, number int) ([]*Comment, error)
	CreateComment(ctx context.Context, number int, body string) (*Comment, error)
	UpdateComment(ctx context.Context, commentID int64, body string) (*Comment, error)
	DeleteComment(ctx context.Context, commentID int64) error
}
