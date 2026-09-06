package github

import (
	"context"
	"fmt"
)

// fakeAPI is an in-memory API implementation for tests, so sync logic can
// be exercised without a real GitHub repository.
type fakeAPI struct {
	issues      map[int]*Issue
	comments    map[int][]*Comment // by issue number
	nextIssue   int
	nextComment int64
}

func newFakeAPI() *fakeAPI {
	return &fakeAPI{
		issues:      make(map[int]*Issue),
		comments:    make(map[int][]*Comment),
		nextIssue:   1,
		nextComment: 1,
	}
}

func (f *fakeAPI) ListIssues(ctx context.Context) ([]*Issue, error) {
	var out []*Issue
	for _, iss := range f.issues {
		cp := *iss
		out = append(out, &cp)
	}
	return out, nil
}

func (f *fakeAPI) GetIssue(ctx context.Context, number int) (*Issue, error) {
	iss, ok := f.issues[number]
	if !ok {
		return nil, fmt.Errorf("fakeAPI: issue #%d not found", number)
	}
	cp := *iss
	return &cp, nil
}

func (f *fakeAPI) CreateIssue(ctx context.Context, in IssueInput) (*Issue, error) {
	n := f.nextIssue
	f.nextIssue++
	state := in.State
	if state == "" {
		state = "open"
	}
	iss := &Issue{
		Number:    n,
		Title:     in.Title,
		Body:      in.Body,
		State:     state,
		Labels:    in.Labels,
		Assignees: in.Assignees,
		URL:       fmt.Sprintf("https://github.com/test/repo/issues/%d", n),
	}
	f.issues[n] = iss
	cp := *iss
	return &cp, nil
}

func (f *fakeAPI) UpdateIssue(ctx context.Context, number int, in IssueInput) (*Issue, error) {
	iss, ok := f.issues[number]
	if !ok {
		return nil, fmt.Errorf("fakeAPI: issue #%d not found", number)
	}
	iss.Title = in.Title
	iss.Body = in.Body
	iss.Labels = in.Labels
	iss.Assignees = in.Assignees
	if in.State != "" {
		iss.State = in.State
	}
	cp := *iss
	return &cp, nil
}

func (f *fakeAPI) ListComments(ctx context.Context, number int) ([]*Comment, error) {
	var out []*Comment
	for _, c := range f.comments[number] {
		cp := *c
		out = append(out, &cp)
	}
	return out, nil
}

func (f *fakeAPI) CreateComment(ctx context.Context, number int, body string) (*Comment, error) {
	c := &Comment{ID: f.nextComment, Author: "remote-user", Body: body}
	f.nextComment++
	f.comments[number] = append(f.comments[number], c)
	cp := *c
	return &cp, nil
}

func (f *fakeAPI) UpdateComment(ctx context.Context, commentID int64, body string) (*Comment, error) {
	for _, list := range f.comments {
		for _, c := range list {
			if c.ID == commentID {
				c.Body = body
				cp := *c
				return &cp, nil
			}
		}
	}
	return nil, fmt.Errorf("fakeAPI: comment %d not found", commentID)
}

func (f *fakeAPI) DeleteComment(ctx context.Context, commentID int64) error {
	for number, list := range f.comments {
		for i, c := range list {
			if c.ID == commentID {
				f.comments[number] = append(list[:i], list[i+1:]...)
				return nil
			}
		}
	}
	return fmt.Errorf("fakeAPI: comment %d not found", commentID)
}
