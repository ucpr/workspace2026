// Package github implements a small, purpose-built GitHub REST API
// client for Releascope. It intentionally does not depend on
// google/go-github: Releascope only needs three read endpoints, and a
// hand-rolled client keeps the dependency footprint (and Lambda cold
// start) small while making the client/API interface trivial to mock in
// tests.
package github

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/ucpr/releascope/internal/apperr"
	"github.com/ucpr/releascope/internal/fault"
	"github.com/ucpr/releascope/internal/telemetry"
)

var tracer = otel.Tracer("releascope/internal/github")

// API is the interface consumed by application/service code, allowing
// tests to substitute a mock instead of hitting the network.
type API interface {
	GetLatestRelease(ctx context.Context, owner, repo string) (*Release, error)
	GetReleaseByTag(ctx context.Context, owner, repo, tag string) (*Release, error)
	CompareCommits(ctx context.Context, owner, repo, base, head string) (*CompareResult, error)
	ListMergedPullRequests(ctx context.Context, owner, repo string, since, until time.Time) ([]PullRequest, error)
}

// Client is the default API implementation backed by net/http. Its
// Transport is wrapped with otelhttp so every outbound call to
// api.github.com produces an HTTP client span (spec section 16: "外部
// HTTP API の client span") without any per-call instrumentation code.
type Client struct {
	httpClient *http.Client
	baseURL    string
	token      string
}

// NewClient builds a Client. token may be empty for unauthenticated
// (heavily rate-limited) access, which is useful for local development.
func NewClient(baseURL, token string) *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: otelhttp.NewTransport(http.DefaultTransport),
		},
		baseURL: baseURL,
		token:   token,
	}
}

var _ API = (*Client)(nil)

func (c *Client) GetLatestRelease(ctx context.Context, owner, repo string) (*Release, error) {
	ctx, span := tracer.Start(ctx, "github.get_latest_release", trace.WithAttributes(
		attribute.String("release.repository", owner+"/"+repo),
	))
	defer span.End()

	var rel Release
	path := fmt.Sprintf("/repos/%s/%s/releases/latest", owner, repo)
	if err := c.get(ctx, path, &rel); err != nil {
		apperr.RecordSpanError(ctx, err)
		return nil, err
	}
	span.SetAttributes(attribute.String("release.version", rel.TagName))
	return &rel, nil
}

func (c *Client) GetReleaseByTag(ctx context.Context, owner, repo, tag string) (*Release, error) {
	ctx, span := tracer.Start(ctx, "github.get_release_by_tag", trace.WithAttributes(
		attribute.String("release.repository", owner+"/"+repo),
		attribute.String("release.version", tag),
	))
	defer span.End()

	var rel Release
	path := fmt.Sprintf("/repos/%s/%s/releases/tags/%s", owner, repo, tag)
	if err := c.get(ctx, path, &rel); err != nil {
		apperr.RecordSpanError(ctx, err)
		return nil, err
	}
	return &rel, nil
}

func (c *Client) CompareCommits(ctx context.Context, owner, repo, base, head string) (*CompareResult, error) {
	ctx, span := tracer.Start(ctx, "github.compare", trace.WithAttributes(
		attribute.String("release.repository", owner+"/"+repo),
		attribute.String("github.compare.base", base),
		attribute.String("github.compare.head", head),
	))
	defer span.End()

	var cmp CompareResult
	path := fmt.Sprintf("/repos/%s/%s/compare/%s...%s", owner, repo, base, head)
	if err := c.get(ctx, path, &cmp); err != nil {
		apperr.RecordSpanError(ctx, err)
		return nil, err
	}
	span.SetAttributes(
		attribute.Int("github.compare.total_commits", cmp.TotalCommits),
		attribute.Int("github.compare.files_changed", len(cmp.Files)),
	)
	return &cmp, nil
}

// ListMergedPullRequests returns pull requests merged within
// [since, until]. GitHub has no "PRs between two tags" endpoint, so this
// walks the closed-PR list (sorted by update time, newest first) and
// stops once results fall behind `since`. Precision is best-effort by
// design (spec section 9: "完全な精度は MVP では要求しない") - a PR
// updated (e.g. re-labeled) after `until` but merged inside the window
// could theoretically be missed if it also fails the sort assumption,
// which is an accepted MVP tradeoff.
func (c *Client) ListMergedPullRequests(ctx context.Context, owner, repo string, since, until time.Time) ([]PullRequest, error) {
	ctx, span := tracer.Start(ctx, "github.list_pull_requests", trace.WithAttributes(
		attribute.String("release.repository", owner+"/"+repo),
	))
	defer span.End()

	const maxPages = 5
	const perPage = 50
	var matched []PullRequest

	for page := 1; page <= maxPages; page++ {
		path := fmt.Sprintf("/repos/%s/%s/pulls?state=closed&sort=updated&direction=desc&per_page=%d&page=%d", owner, repo, perPage, page)
		var prs []PullRequest
		if err := c.get(ctx, path, &prs); err != nil {
			apperr.RecordSpanError(ctx, err)
			return nil, err
		}
		if len(prs) == 0 {
			break
		}

		reachedWindowStart := false
		for _, pr := range prs {
			if pr.MergedAt.IsZero() {
				continue // closed without merge
			}
			if pr.MergedAt.After(since) && !pr.MergedAt.After(until) {
				matched = append(matched, pr)
			}
			if !pr.MergedAt.After(since) {
				// This page (sorted newest-updated-first) has reached
				// PRs merged at or before the window start; assume
				// older pages hold nothing newer and stop paging.
				reachedWindowStart = true
			}
		}
		if reachedWindowStart || len(prs) < perPage {
			break
		}
	}

	span.SetAttributes(attribute.Int("github.pull_requests.count", len(matched)))
	return matched, nil
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	if err := fault.MaybeInject(ctx, fault.GitHubTimeout, 31*time.Second); err != nil {
		telemetry.GitHubRequestErrorsTotal.Add(ctx, 1)
		return apperr.New(apperr.GitHubTimeout, true, err)
	}
	if err := fault.MaybeInject(ctx, fault.GitHubRateLimit, 0); err != nil {
		telemetry.GitHubRequestErrorsTotal.Add(ctx, 1)
		return apperr.New(apperr.GitHubRateLimit, true, err)
	}
	if err := fault.MaybeInject(ctx, fault.GitHubServerError, 0); err != nil {
		telemetry.GitHubRequestErrorsTotal.Add(ctx, 1)
		return apperr.New(apperr.GitHubServerError, true, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return apperr.New(apperr.InvalidInput, false, err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}

	telemetry.GitHubRequestTotal.Add(ctx, 1)
	resp, err := c.httpClient.Do(req)
	if err != nil {
		telemetry.GitHubRequestErrorsTotal.Add(ctx, 1)
		var netErr interface{ Timeout() bool }
		if errors.As(err, &netErr) && netErr.Timeout() {
			return apperr.New(apperr.GitHubTimeout, true, err)
		}
		return apperr.New(apperr.GitHubServerError, true, err)
	}
	defer resp.Body.Close()

	recordRateLimit(resp.Header)

	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		telemetry.GitHubRequestErrorsTotal.Add(ctx, 1)
		return apperr.New(apperr.GitHubRateLimit, true, fmt.Errorf("github rate limited: %s", resp.Status))
	case resp.StatusCode == http.StatusForbidden && resp.Header.Get("X-RateLimit-Remaining") == "0":
		telemetry.GitHubRequestErrorsTotal.Add(ctx, 1)
		return apperr.New(apperr.GitHubRateLimit, true, fmt.Errorf("github rate limit exhausted: %s", resp.Status))
	case resp.StatusCode >= 500:
		telemetry.GitHubRequestErrorsTotal.Add(ctx, 1)
		return apperr.New(apperr.GitHubServerError, true, fmt.Errorf("github server error: %s", resp.Status))
	case resp.StatusCode >= 400:
		telemetry.GitHubRequestErrorsTotal.Add(ctx, 1)
		return apperr.New(apperr.GitHubClientError, false, fmt.Errorf("github client error: %s", resp.Status))
	}

	if out == nil {
		return nil
	}
	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return apperr.New(apperr.GitHubClientError, false, fmt.Errorf("decode github response: %w", err))
	}
	return nil
}

func recordRateLimit(h http.Header) {
	remaining, err := strconv.ParseInt(h.Get("X-RateLimit-Remaining"), 10, 64)
	if err != nil {
		return
	}
	telemetry.SetGitHubRateLimitRemaining(remaining)
}
