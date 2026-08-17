package service

import (
	"context"
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/ucpr/releascope/internal/apperr"
	"github.com/ucpr/releascope/internal/config"
	"github.com/ucpr/releascope/internal/github"
	"github.com/ucpr/releascope/internal/release"
	"github.com/ucpr/releascope/internal/storage"
	"github.com/ucpr/releascope/internal/telemetry"
)

// FetchContextDeps are shared by all three fetch-context actions.
type FetchContextDeps struct {
	GitHub github.API
	Data   *storage.DataStore
}

// FetchContextInput is the union of fields any of the three parallel
// fetch-context actions might need; the Step Functions ASL sends each
// branch only the subset relevant to its Action (see
// infra/terraform/statemachine.asl.json), everything else is left zero.
type FetchContextInput struct {
	Action              string            `json:"action"` // "compare" | "pull_requests" | "extract_changes"
	Repository          string            `json:"repository"`
	PreviousVersion     string            `json:"previousVersion"`
	CurrentVersion      string            `json:"currentVersion"`
	TargetCommitish     string            `json:"targetCommitish"`
	PreviousPublishedAt time.Time         `json:"previousPublishedAt"`
	CurrentPublishedAt  time.Time         `json:"currentPublishedAt"`
	CurrentReleaseS3URI string            `json:"currentReleaseS3Uri"`
	TraceContext        map[string]string `json:"traceContext"`
}

// FetchContextOutput is a superset covering all three actions' results;
// only the fields relevant to the requested Action are populated.
type FetchContextOutput struct {
	// These three fields are deliberately NOT omitempty: the Step
	// Functions ASL dereferences them by JSONPath
	// ($.contextResults[i].result.xxx) in the AnalyzeWithLLM task, and a
	// missing (vs. merely empty) key makes that a hard "path not found"
	// runtime error rather than a harmless empty value. See
	// infra/terraform/statemachine.asl.json.tftpl.
	CompareS3URI       string   `json:"compareS3Uri"`
	PullRequestsS3URI  string   `json:"pullRequestsS3Uri"`
	AffectedComponents []string `json:"affectedComponents"`

	TotalCommits     int `json:"totalCommits,omitempty"`
	FilesChanged     int `json:"filesChanged,omitempty"`
	PullRequestCount int `json:"pullRequestCount,omitempty"`
}

// FetchContext dispatches to the action-specific handler. Running all
// three actions as one Lambda function invoked with different `action`
// values (rather than three separate Lambda functions) keeps the
// deployable unit count down while still giving Step Functions'
// Parallel state three genuinely independent, concurrently-scheduled
// Task states to retry/catch individually.
func FetchContext(ctx context.Context, deps FetchContextDeps, in FetchContextInput) (*FetchContextOutput, error) {
	ctx = telemetry.ExtractMap(ctx, in.TraceContext)

	owner, repo, err := config.SplitOwnerRepo(in.Repository)
	if err != nil {
		return nil, apperr.New(apperr.InvalidInput, false, err)
	}

	switch in.Action {
	case "compare":
		return fetchCompare(ctx, deps, owner, repo, in)
	case "pull_requests":
		return fetchPullRequests(ctx, deps, owner, repo, in)
	case "extract_changes":
		return extractChanges(ctx, deps, owner, repo, in)
	default:
		return nil, apperr.New(apperr.InvalidInput, false, fmt.Errorf("unknown fetch-context action %q", in.Action))
	}
}

func fetchCompare(ctx context.Context, deps FetchContextDeps, owner, repo string, in FetchContextInput) (*FetchContextOutput, error) {
	ctx, span := tracer.Start(ctx, "release.fetch_context.compare", trace.WithAttributes(
		attribute.String("release.repository", in.Repository),
	))
	defer span.End()

	if in.PreviousVersion == "" {
		// First-ever release check for this repository: there is
		// nothing to diff against.
		return &FetchContextOutput{}, nil
	}

	head := in.TargetCommitish
	if head == "" {
		head = in.CurrentVersion
	}
	cmp, err := deps.GitHub.CompareCommits(ctx, owner, repo, in.PreviousVersion, head)
	if err != nil {
		return nil, err
	}

	key := storage.ReleaseDataKey(owner, repo, in.CurrentVersion, "compare.json")
	uri, err := deps.Data.PutJSON(ctx, key, cmp)
	if err != nil {
		return nil, err
	}

	return &FetchContextOutput{CompareS3URI: uri, TotalCommits: cmp.TotalCommits, FilesChanged: len(cmp.Files)}, nil
}

func fetchPullRequests(ctx context.Context, deps FetchContextDeps, owner, repo string, in FetchContextInput) (*FetchContextOutput, error) {
	ctx, span := tracer.Start(ctx, "release.fetch_context.pull_requests", trace.WithAttributes(
		attribute.String("release.repository", in.Repository),
	))
	defer span.End()

	if in.PreviousVersion == "" || in.PreviousPublishedAt.IsZero() {
		return &FetchContextOutput{}, nil
	}

	prs, err := deps.GitHub.ListMergedPullRequests(ctx, owner, repo, in.PreviousPublishedAt, in.CurrentPublishedAt)
	if err != nil {
		return nil, err
	}

	key := storage.ReleaseDataKey(owner, repo, in.CurrentVersion, "pull-requests.json")
	uri, err := deps.Data.PutJSON(ctx, key, prs)
	if err != nil {
		return nil, err
	}

	return &FetchContextOutput{PullRequestsS3URI: uri, PullRequestCount: len(prs)}, nil
}

func extractChanges(ctx context.Context, deps FetchContextDeps, owner, repo string, in FetchContextInput) (*FetchContextOutput, error) {
	_, span := tracer.Start(ctx, "release.fetch_context.extract_changes", trace.WithAttributes(
		attribute.String("release.repository", in.Repository),
	))
	defer span.End()

	if in.CurrentReleaseS3URI == "" {
		return &FetchContextOutput{AffectedComponents: []string{}}, nil
	}

	var rel github.Release
	if err := deps.Data.GetJSONByURI(ctx, in.CurrentReleaseS3URI, &rel); err != nil {
		return nil, err
	}

	components := release.ExtractComponents(rel.Body, rel.Name)
	span.SetAttributes(attribute.Int("release.affected_components.count", len(components)))
	return &FetchContextOutput{AffectedComponents: components}, nil
}
