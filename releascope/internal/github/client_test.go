package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/ucpr/releascope/internal/apperr"
)

func TestGetLatestRelease(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/open-telemetry/opentelemetry-collector/releases/latest" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("X-RateLimit-Remaining", "4999")
		_ = json.NewEncoder(w).Encode(Release{
			TagName:     "v0.100.0",
			Name:        "v0.100.0",
			PublishedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			HTMLURL:     "https://github.com/open-telemetry/opentelemetry-collector/releases/tag/v0.100.0",
		})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "")
	rel, err := c.GetLatestRelease(context.Background(), "open-telemetry", "opentelemetry-collector")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if rel.TagName != "v0.100.0" {
		t.Errorf("got tag %q, want v0.100.0", rel.TagName)
	}
}

func TestGetLatestRelease_RateLimited(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "")
	_, err := c.GetLatestRelease(context.Background(), "o", "r")
	if err == nil {
		t.Fatal("expected error")
	}
	if got := apperr.CategoryOf(err); got != apperr.GitHubRateLimit {
		t.Errorf("category = %s, want %s", got, apperr.GitHubRateLimit)
	}
	if !apperr.IsRetryable(err) {
		t.Error("expected rate limit error to be retryable")
	}
}

func TestGetLatestRelease_ServerError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "")
	_, err := c.GetLatestRelease(context.Background(), "o", "r")
	if got := apperr.CategoryOf(err); got != apperr.GitHubServerError {
		t.Errorf("category = %s, want %s", got, apperr.GitHubServerError)
	}
	if !apperr.IsRetryable(err) {
		t.Error("expected server error to be retryable")
	}
}

func TestGetLatestRelease_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "")
	_, err := c.GetLatestRelease(context.Background(), "o", "r")
	if got := apperr.CategoryOf(err); got != apperr.GitHubClientError {
		t.Errorf("category = %s, want %s", got, apperr.GitHubClientError)
	}
	if apperr.IsRetryable(err) {
		t.Error("expected client error to be non-retryable")
	}
}

func TestCompareCommits(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/o/r/compare/v1...v2" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(CompareResult{TotalCommits: 3, Status: "ahead"})
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "")
	cmp, err := c.CompareCommits(context.Background(), "o", "r", "v1", "v2")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cmp.TotalCommits != 3 {
		t.Errorf("got %d commits, want 3", cmp.TotalCommits)
	}
}

func TestListMergedPullRequests_FiltersByWindowAndStopsPaging(t *testing.T) {
	since := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	until := time.Date(2026, 2, 1, 0, 0, 0, 0, time.UTC)

	page1 := []PullRequest{
		{Number: 3, MergedAt: time.Date(2026, 1, 20, 0, 0, 0, 0, time.UTC)}, // in window
		{Number: 2, MergedAt: time.Date(2026, 1, 10, 0, 0, 0, 0, time.UTC)}, // in window
		{Number: 1, MergedAt: time.Date(2025, 12, 1, 0, 0, 0, 0, time.UTC)}, // before window -> stop paging
	}
	requests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests > 1 {
			t.Fatalf("expected paging to stop after first page, got request %d", requests)
		}
		_ = json.NewEncoder(w).Encode(page1)
	}))
	defer srv.Close()

	c := NewClient(srv.URL, "")
	prs, err := c.ListMergedPullRequests(context.Background(), "o", "r", since, until)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(prs) != 2 {
		t.Fatalf("got %d PRs, want 2", len(prs))
	}
}
