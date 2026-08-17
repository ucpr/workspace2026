// Package service implements the use-case layer shared by every
// Lambda's handler (kept thin per spec section 14) and by the local CLI
// (spec section 24: `make check-release` must not need a real Lambda
// runtime). None of these functions import aws-lambda-go, so they are
// runnable and unit-testable outside Lambda entirely.
package service

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/ucpr/releascope/internal/apperr"
	"github.com/ucpr/releascope/internal/config"
	"github.com/ucpr/releascope/internal/github"
	"github.com/ucpr/releascope/internal/release"
	"github.com/ucpr/releascope/internal/storage"
	"github.com/ucpr/releascope/internal/telemetry"
)

var tracer = otel.Tracer("releascope/internal/service")

// CheckReleaseDeps are the collaborators CheckRelease needs. Using an
// interface for GitHub (github.API) and concrete stores makes it
// straightforward to substitute a mock GitHub client in tests while
// still exercising real (in-memory or moto-backed, if desired) storage.
type CheckReleaseDeps struct {
	GitHub github.API
	Repos  *storage.RepositoryStore
	Data   *storage.DataStore
}

// CheckReleaseInput is the CheckLatestRelease Step Functions task's
// input contract.
type CheckReleaseInput struct {
	Repository string `json:"repository"`
}

// CheckReleaseOutput is the CheckLatestRelease Step Functions task's
// output contract. Only small scalar fields and S3 URIs are returned
// (spec section 7): the full release bodies are written to S3 by this
// function and referenced by URI so downstream Step Functions state
// payloads stay well under the 256KB limit.
type CheckReleaseOutput struct {
	Repository           string    `json:"repository"`
	IsNew                bool      `json:"isNew"`
	PreviousVersion      string    `json:"previousVersion"`
	CurrentVersion       string    `json:"currentVersion"`
	PublishedAt          time.Time `json:"publishedAt"`
	PreviousPublishedAt  time.Time `json:"previousPublishedAt,omitempty"`
	ReleaseURL           string    `json:"releaseUrl"`
	TargetCommitish      string    `json:"targetCommitish"`
	CurrentReleaseS3URI  string    `json:"currentReleaseS3Uri,omitempty"`
	PreviousReleaseS3URI string    `json:"previousReleaseS3Uri,omitempty"`
	// Not omitempty: downstream Step Functions tasks dereference this by
	// JSONPath, and a missing key is a hard runtime error there, unlike
	// an empty object (see FetchContextOutput's comment above).
	TraceContext map[string]string `json:"traceContext"`
}

// CheckRelease implements spec section 6/29-Phase1: fetch the latest
// GitHub release, compare it against the stored pointer, and - if new -
// snapshot both the current and previous release metadata to S3 so the
// rest of the pipeline never needs to re-fetch them from GitHub.
func CheckRelease(ctx context.Context, deps CheckReleaseDeps, in CheckReleaseInput) (*CheckReleaseOutput, error) {
	ctx, span := tracer.Start(ctx, "release.check", trace.WithAttributes(
		attribute.String("release.repository", in.Repository),
	))
	defer span.End()
	telemetry.ReleaseCheckTotal.Add(ctx, 1)

	owner, repo, err := config.SplitOwnerRepo(in.Repository)
	if err != nil {
		apperr.RecordSpanError(ctx, err)
		return nil, apperr.New(apperr.InvalidInput, false, err)
	}

	storedRepo, err := deps.Repos.Get(ctx, in.Repository)
	if err != nil {
		return nil, err
	}
	storedVersion := ""
	if storedRepo != nil {
		storedVersion = storedRepo.LatestVersion
	}

	latest, err := deps.GitHub.GetLatestRelease(ctx, owner, repo)
	if err != nil {
		return nil, err
	}

	isNew := isNewReleaseSpan(ctx, storedVersion, latest.TagName)
	span.SetAttributes(
		attribute.Bool("release.new", isNew),
		attribute.String("release.version", latest.TagName),
		attribute.String("release.previous_version", storedVersion),
	)

	out := &CheckReleaseOutput{
		Repository:      in.Repository,
		IsNew:           isNew,
		PreviousVersion: storedVersion,
		CurrentVersion:  latest.TagName,
		PublishedAt:     latest.PublishedAt,
		ReleaseURL:      latest.HTMLURL,
		TargetCommitish: latest.TargetCommitish,
	}

	if !isNew {
		if err := deps.Repos.MarkChecked(ctx, in.Repository, time.Now()); err != nil {
			return nil, err
		}
		return out, nil
	}

	telemetry.ReleaseNewTotal.Add(ctx, 1)

	currentKey := storage.ReleaseDataKey(owner, repo, latest.TagName, "release.json")
	uri, err := deps.Data.PutJSON(ctx, currentKey, latest)
	if err != nil {
		return nil, err
	}
	out.CurrentReleaseS3URI = uri

	if storedVersion != "" {
		prev, err := deps.GitHub.GetReleaseByTag(ctx, owner, repo, storedVersion)
		if err != nil {
			return nil, err
		}
		prevKey := storage.ReleaseDataKey(owner, repo, latest.TagName, "previous-release.json")
		prevURI, err := deps.Data.PutJSON(ctx, prevKey, prev)
		if err != nil {
			return nil, err
		}
		out.PreviousReleaseS3URI = prevURI
		out.PreviousPublishedAt = prev.PublishedAt
	}

	// Serialize this span's context into the output so downstream Step
	// Functions tasks (which run in separate Lambda invocations, hence
	// separate processes) can extract it and continue the same trace
	// instead of starting a disconnected one. See README's Trace
	// Propagation section for the full boundary-by-boundary breakdown.
	out.TraceContext = telemetry.InjectMap(ctx)

	return out, nil
}

func isNewReleaseSpan(ctx context.Context, stored, latest string) bool {
	_, span := tracer.Start(ctx, "release.compare_version", trace.WithAttributes(
		attribute.String("release.previous_version", stored),
		attribute.String("release.version", latest),
	))
	defer span.End()
	isNew := release.IsNewRelease(stored, latest)
	span.SetAttributes(attribute.Bool("release.new", isNew))
	return isNew
}
