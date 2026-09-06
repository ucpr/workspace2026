package github

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	gogithub "github.com/google/go-github/v66/github"
	"golang.org/x/oauth2"
)

// ResolveToken returns a GitHub API token, preferring the GITHUB_TOKEN
// environment variable and falling back to `gh auth token` (requirements
// §5.4: auth delegates to the `gh` CLI by default, with an env var
// override; never store tokens in the repo).
func ResolveToken(ctx context.Context) (string, error) {
	if t := os.Getenv("GITHUB_TOKEN"); t != "" {
		return t, nil
	}
	out, err := exec.CommandContext(ctx, "gh", "auth", "token").Output()
	if err != nil {
		return "", fmt.Errorf("github: resolving token: set GITHUB_TOKEN or run `gh auth login`: %w", err)
	}
	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", fmt.Errorf("github: `gh auth token` returned an empty token; set GITHUB_TOKEN or run `gh auth login`")
	}
	return token, nil
}

// Client is the real API implementation, backed by google/go-github.
type Client struct {
	gh    *gogithub.Client
	owner string
	repo  string
}

// SplitRepo splits "owner/repo" into its parts.
func SplitRepo(repo string) (owner, name string, err error) {
	parts := strings.SplitN(repo, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("github: repo %q must be in owner/repo form", repo)
	}
	return parts[0], parts[1], nil
}

// NewClient builds a Client for repo ("owner/repo"), authenticated with
// token.
func NewClient(token, repo string) (*Client, error) {
	owner, name, err := SplitRepo(repo)
	if err != nil {
		return nil, err
	}
	ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
	tc := oauth2.NewClient(context.Background(), ts)
	return &Client{gh: gogithub.NewClient(tc), owner: owner, repo: name}, nil
}

func fromIssue(iss *gogithub.Issue) *Issue {
	labels := make([]string, 0, len(iss.Labels))
	for _, l := range iss.Labels {
		labels = append(labels, l.GetName())
	}
	assignees := make([]string, 0, len(iss.Assignees))
	for _, a := range iss.Assignees {
		assignees = append(assignees, a.GetLogin())
	}
	return &Issue{
		Number:    iss.GetNumber(),
		Title:     iss.GetTitle(),
		Body:      iss.GetBody(),
		State:     iss.GetState(),
		Labels:    labels,
		Assignees: assignees,
		URL:       iss.GetHTMLURL(),
		CreatedAt: iss.GetCreatedAt().Time,
		UpdatedAt: iss.GetUpdatedAt().Time,
	}
}

func (c *Client) ListIssues(ctx context.Context) ([]*Issue, error) {
	var all []*Issue
	opt := &gogithub.IssueListByRepoOptions{
		State:       "all",
		ListOptions: gogithub.ListOptions{PerPage: 100},
	}
	for {
		issues, resp, err := c.gh.Issues.ListByRepo(ctx, c.owner, c.repo, opt)
		if err != nil {
			return nil, fmt.Errorf("github: listing issues: %w", err)
		}
		for _, iss := range issues {
			if iss.IsPullRequest() {
				continue
			}
			all = append(all, fromIssue(iss))
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return all, nil
}

func (c *Client) GetIssue(ctx context.Context, number int) (*Issue, error) {
	iss, _, err := c.gh.Issues.Get(ctx, c.owner, c.repo, number)
	if err != nil {
		return nil, fmt.Errorf("github: getting issue %d: %w", number, err)
	}
	return fromIssue(iss), nil
}

func (c *Client) CreateIssue(ctx context.Context, in IssueInput) (*Issue, error) {
	req := &gogithub.IssueRequest{
		Title:     gogithub.String(in.Title),
		Body:      gogithub.String(in.Body),
		Labels:    &in.Labels,
		Assignees: &in.Assignees,
	}
	iss, _, err := c.gh.Issues.Create(ctx, c.owner, c.repo, req)
	if err != nil {
		return nil, fmt.Errorf("github: creating issue: %w", err)
	}
	return fromIssue(iss), nil
}

func (c *Client) UpdateIssue(ctx context.Context, number int, in IssueInput) (*Issue, error) {
	req := &gogithub.IssueRequest{
		Title:     gogithub.String(in.Title),
		Body:      gogithub.String(in.Body),
		Labels:    &in.Labels,
		Assignees: &in.Assignees,
	}
	if in.State != "" {
		req.State = gogithub.String(in.State)
	}
	iss, _, err := c.gh.Issues.Edit(ctx, c.owner, c.repo, number, req)
	if err != nil {
		return nil, fmt.Errorf("github: updating issue %d: %w", number, err)
	}
	return fromIssue(iss), nil
}

func fromComment(c *gogithub.IssueComment) *Comment {
	return &Comment{
		ID:        c.GetID(),
		Author:    c.GetUser().GetLogin(),
		Body:      c.GetBody(),
		CreatedAt: c.GetCreatedAt().Time,
		UpdatedAt: c.GetUpdatedAt().Time,
	}
}

func (c *Client) ListComments(ctx context.Context, number int) ([]*Comment, error) {
	var all []*Comment
	opt := &gogithub.IssueListCommentsOptions{ListOptions: gogithub.ListOptions{PerPage: 100}}
	for {
		comments, resp, err := c.gh.Issues.ListComments(ctx, c.owner, c.repo, number, opt)
		if err != nil {
			return nil, fmt.Errorf("github: listing comments on issue %d: %w", number, err)
		}
		for _, cm := range comments {
			all = append(all, fromComment(cm))
		}
		if resp.NextPage == 0 {
			break
		}
		opt.Page = resp.NextPage
	}
	return all, nil
}

func (c *Client) CreateComment(ctx context.Context, number int, body string) (*Comment, error) {
	cm, _, err := c.gh.Issues.CreateComment(ctx, c.owner, c.repo, number, &gogithub.IssueComment{Body: gogithub.String(body)})
	if err != nil {
		return nil, fmt.Errorf("github: creating comment on issue %d: %w", number, err)
	}
	return fromComment(cm), nil
}

func (c *Client) UpdateComment(ctx context.Context, commentID int64, body string) (*Comment, error) {
	cm, _, err := c.gh.Issues.EditComment(ctx, c.owner, c.repo, commentID, &gogithub.IssueComment{Body: gogithub.String(body)})
	if err != nil {
		return nil, fmt.Errorf("github: updating comment %d: %w", commentID, err)
	}
	return fromComment(cm), nil
}

func (c *Client) DeleteComment(ctx context.Context, commentID int64) error {
	if _, err := c.gh.Issues.DeleteComment(ctx, c.owner, c.repo, commentID); err != nil {
		return fmt.Errorf("github: deleting comment %d: %w", commentID, err)
	}
	return nil
}
