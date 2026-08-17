package service

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/ucpr/releascope/internal/release"
	"github.com/ucpr/releascope/internal/storage"
	"github.com/ucpr/releascope/internal/telemetry"
)

type PersistDeps struct {
	Releases *storage.ReleaseStore
}

type PersistInput struct {
	Repository          string            `json:"repository"`
	PreviousVersion     string            `json:"previousVersion"`
	CurrentVersion      string            `json:"currentVersion"`
	PublishedAt         time.Time         `json:"publishedAt"`
	ReleaseURL          string            `json:"releaseUrl"`
	CurrentReleaseS3URI string            `json:"currentReleaseS3Uri"`
	LLMRawResponseS3URI string            `json:"llmRawResponseS3Uri"`
	Analysis            release.Analysis  `json:"analysis"`
	TraceContext        map[string]string `json:"traceContext"`
}

type PersistOutput struct {
	Repository string `json:"repository"`
	Version    string `json:"version"`
	Persisted  bool   `json:"persisted"`
}

// Persist implements the final Step Functions state: write the release
// record and advance the repository's latest-version pointer as one
// DynamoDB transaction (spec section 10/30).
func Persist(ctx context.Context, deps PersistDeps, in PersistInput) (*PersistOutput, error) {
	ctx = telemetry.ExtractMap(ctx, in.TraceContext)
	ctx, span := tracer.Start(ctx, "release.persist", trace.WithAttributes(
		attribute.String("release.repository", in.Repository),
		attribute.String("release.version", in.CurrentVersion),
	))
	defer span.End()

	rec := release.Record{
		Repository:          in.Repository,
		Version:             in.CurrentVersion,
		PreviousVersion:     in.PreviousVersion,
		PublishedAt:         in.PublishedAt,
		ReleaseURL:          in.ReleaseURL,
		Analysis:            in.Analysis,
		RawDataS3URI:        in.CurrentReleaseS3URI,
		LLMRawResponseS3URI: in.LLMRawResponseS3URI,
	}

	if err := deps.Releases.Persist(ctx, rec, time.Now()); err != nil {
		return nil, err
	}

	return &PersistOutput{Repository: in.Repository, Version: in.CurrentVersion, Persisted: true}, nil
}
