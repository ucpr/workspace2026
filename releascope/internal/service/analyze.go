package service

import (
	"context"
	"encoding/json"
	"sort"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/ucpr/releascope/internal/bedrock"
	"github.com/ucpr/releascope/internal/config"
	"github.com/ucpr/releascope/internal/github"
	"github.com/ucpr/releascope/internal/release"
	"github.com/ucpr/releascope/internal/storage"
	"github.com/ucpr/releascope/internal/telemetry"
)

// Token/payload safety limits (spec section 7). These are conservative
// truncations, not exact tokenizer-based counts - simple and good
// enough for an MVP, revisit only if Bedrock actually rejects a prompt.
const (
	maxReleaseNotesChars = 12000
	maxChangedFiles      = 50
	maxPullRequests      = 20
)

type AnalyzeDeps struct {
	Bedrock bedrock.API
	Data    *storage.DataStore
}

type AnalyzeInput struct {
	Repository          string            `json:"repository"`
	PreviousVersion     string            `json:"previousVersion"`
	CurrentVersion      string            `json:"currentVersion"`
	PublishedAt         time.Time         `json:"publishedAt"`
	ReleaseURL          string            `json:"releaseUrl"`
	CurrentReleaseS3URI string            `json:"currentReleaseS3Uri"`
	CompareS3URI        string            `json:"compareS3Uri"`
	PullRequestsS3URI   string            `json:"pullRequestsS3Uri"`
	ExtractedComponents []string          `json:"extractedComponents"`
	TraceContext        map[string]string `json:"traceContext"`
}

type AnalyzeOutput struct {
	Analysis            release.Analysis `json:"analysis"`
	LLMRawResponseS3URI string           `json:"llmRawResponseS3Uri"`
	// Not omitempty: PersistRelease's Step Functions task dereferences
	// this by JSONPath (see FetchContextOutput's comment above).
	TraceContext map[string]string `json:"traceContext"`
}

// Analyze implements spec section 8: assemble release context from S3,
// ask Bedrock for structured impact analysis, merge in
// regex-based component extraction (section 9), and persist the raw
// model response to S3 for debuggability.
func Analyze(ctx context.Context, deps AnalyzeDeps, in AnalyzeInput) (*AnalyzeOutput, error) {
	ctx = telemetry.ExtractMap(ctx, in.TraceContext)
	ctx, span := tracer.Start(ctx, "release.analyze", trace.WithAttributes(
		attribute.String("release.repository", in.Repository),
		attribute.String("release.version", in.CurrentVersion),
	))
	defer span.End()

	var current github.Release
	if in.CurrentReleaseS3URI != "" {
		if err := deps.Data.GetJSONByURI(ctx, in.CurrentReleaseS3URI, &current); err != nil {
			return nil, err
		}
	}

	var changedFiles []string
	if in.CompareS3URI != "" {
		var cmp github.CompareResult
		if err := deps.Data.GetJSONByURI(ctx, in.CompareS3URI, &cmp); err != nil {
			return nil, err
		}
		for i, f := range cmp.Files {
			if i >= maxChangedFiles {
				break
			}
			changedFiles = append(changedFiles, f.Filename)
		}
	}

	var prSummaries []bedrock.PullRequestSummary
	if in.PullRequestsS3URI != "" {
		var prs []github.PullRequest
		if err := deps.Data.GetJSONByURI(ctx, in.PullRequestsS3URI, &prs); err != nil {
			return nil, err
		}
		for i, pr := range prs {
			if i >= maxPullRequests {
				break
			}
			prSummaries = append(prSummaries, bedrock.PullRequestSummary{Number: pr.Number, Title: pr.Title})
		}
	}

	notes := current.Body
	if len(notes) > maxReleaseNotesChars {
		notes = notes[:maxReleaseNotesChars] + "\n...[truncated]"
	}

	req := bedrock.AnalyzeRequest{
		Repository:      in.Repository,
		PreviousVersion: in.PreviousVersion,
		CurrentVersion:  in.CurrentVersion,
		ReleaseNotes:    notes,
		PullRequests:    prSummaries,
		ChangedFiles:    changedFiles,
	}

	resp, err := deps.Bedrock.AnalyzeRelease(ctx, req)
	if err != nil {
		return nil, err
	}

	owner, repo, _ := config.SplitOwnerRepo(in.Repository)
	reqKey := storage.ReleaseDataKey(owner, repo, in.CurrentVersion, "llm-request.json")
	if _, err := deps.Data.PutJSON(ctx, reqKey, req); err != nil {
		return nil, err
	}
	respKey := storage.ReleaseDataKey(owner, repo, in.CurrentVersion, "llm-response.json")
	rawResponseURI, err := deps.Data.PutJSON(ctx, respKey, rawJSONEnvelope(resp.RawJSON))
	if err != nil {
		return nil, err
	}

	analysis := release.Analysis{
		Summary:            resp.Analysis.Summary,
		Risk:               release.Risk(resp.Analysis.Risk),
		MigrationRequired:  resp.Analysis.MigrationRequired,
		Recommendation:     resp.Analysis.Recommendation,
		AffectedComponents: mergeComponents(in.ExtractedComponents, resp.Analysis),
	}
	for _, c := range resp.Analysis.BreakingChanges {
		analysis.BreakingChanges = append(analysis.BreakingChanges, release.BreakingChange(c))
	}
	for _, c := range resp.Analysis.NotableChanges {
		analysis.NotableChanges = append(analysis.NotableChanges, release.NotableChange(c))
	}
	for _, c := range resp.Analysis.Deprecations {
		analysis.Deprecations = append(analysis.Deprecations, release.Deprecation(c))
	}

	span.SetAttributes(attribute.String("release.risk", string(analysis.Risk)))

	return &AnalyzeOutput{
		Analysis:            analysis,
		LLMRawResponseS3URI: rawResponseURI,
		TraceContext:        telemetry.InjectMap(ctx),
	}, nil
}

func mergeComponents(base []string, analysis bedrock.Analysis) []string {
	var texts []string
	texts = append(texts, analysis.Summary, analysis.Recommendation)
	for _, c := range analysis.BreakingChanges {
		texts = append(texts, c.Component, c.Title, c.Description)
	}
	for _, c := range analysis.NotableChanges {
		texts = append(texts, c.Component, c.Title, c.Description)
	}
	for _, c := range analysis.Deprecations {
		texts = append(texts, c.Component, c.Description)
	}

	set := map[string]struct{}{}
	for _, c := range base {
		set[c] = struct{}{}
	}
	for _, c := range release.ExtractComponents(texts...) {
		set[c] = struct{}{}
	}

	out := make([]string, 0, len(set))
	for c := range set {
		out = append(out, c)
	}
	sort.Strings(out)
	return out
}

// rawJSONEnvelope lets an already-serialized []byte be stored again via
// DataStore.PutJSON (which marshals its argument) without double
// escaping it as a JSON string.
type rawJSONEnvelope json.RawMessage

func (r rawJSONEnvelope) MarshalJSON() ([]byte, error) {
	if len(r) == 0 {
		return []byte("null"), nil
	}
	return r, nil
}
