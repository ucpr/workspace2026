package github

import "time"

// Release mirrors the subset of GitHub's release object Releascope
// actually consumes (spec section 6).
type Release struct {
	TagName         string    `json:"tag_name"`
	Name            string    `json:"name"`
	PublishedAt     time.Time `json:"published_at"`
	HTMLURL         string    `json:"html_url"`
	Body            string    `json:"body"`
	TargetCommitish string    `json:"target_commitish"`
	Draft           bool      `json:"draft"`
	Prerelease      bool      `json:"prerelease"`
}

// CompareResult mirrors GitHub's "compare two commits" response, trimmed
// to the fields used for release-note generation.
type CompareResult struct {
	Status       string        `json:"status"`
	AheadBy      int           `json:"ahead_by"`
	BehindBy     int           `json:"behind_by"`
	TotalCommits int           `json:"total_commits"`
	Commits      []Commit      `json:"commits"`
	Files        []ChangedFile `json:"files"`
	HTMLURL      string        `json:"html_url"`
}

// Commit is a single commit entry as returned by the compare API.
type Commit struct {
	SHA    string `json:"sha"`
	Commit struct {
		Message string `json:"message"`
		Author  struct {
			Name string    `json:"name"`
			Date time.Time `json:"date"`
		} `json:"author"`
	} `json:"commit"`
	HTMLURL string `json:"html_url"`
}

// ChangedFile is a single file entry as returned by the compare API.
type ChangedFile struct {
	Filename  string `json:"filename"`
	Status    string `json:"status"`
	Additions int    `json:"additions"`
	Deletions int    `json:"deletions"`
	Changes   int    `json:"changes"`
}

// PullRequest mirrors the subset of GitHub's pull request object used
// for release context and component extraction.
type PullRequest struct {
	Number   int       `json:"number"`
	Title    string    `json:"title"`
	Body     string    `json:"body"`
	HTMLURL  string    `json:"html_url"`
	State    string    `json:"state"`
	MergedAt time.Time `json:"merged_at"`
	User     struct {
		Login string `json:"login"`
	} `json:"user"`
	Labels []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

// RateLimit captures the subset of GitHub rate limit response headers
// Releascope surfaces as telemetry (github.rate_limit.remaining).
type RateLimit struct {
	Limit     int64
	Remaining int64
	Reset     time.Time
}
