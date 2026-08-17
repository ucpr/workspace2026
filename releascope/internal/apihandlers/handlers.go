package apihandlers

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/sfn"

	"github.com/ucpr/releascope/internal/apperr"
	"github.com/ucpr/releascope/internal/telemetry"
)

type repositorySummary struct {
	Repository        string `json:"repository"`
	LatestVersion     string `json:"latestVersion,omitempty"`
	LatestPublishedAt string `json:"latestPublishedAt,omitempty"`
	LastCheckedAt     string `json:"lastCheckedAt,omitempty"`
}

// GET /repositories
func listRepositories(ctx context.Context, deps Deps) (events.APIGatewayProxyResponse, error) {
	summaries := make([]repositorySummary, 0, len(deps.DefaultRepositories))
	for _, repository := range deps.DefaultRepositories {
		repo, err := deps.Repos.Get(ctx, repository)
		if err != nil {
			return events.APIGatewayProxyResponse{}, err
		}
		if repo == nil {
			summaries = append(summaries, repositorySummary{Repository: repository})
			continue
		}
		summaries = append(summaries, repositorySummary{
			Repository:        repo.Repository,
			LatestVersion:     repo.LatestVersion,
			LatestPublishedAt: formatTime(repo.LatestPublishedAt),
			LastCheckedAt:     formatTime(repo.LastCheckedAt),
		})
	}
	return jsonResponse(200, map[string]any{"repositories": summaries}), nil
}

type releaseSummary struct {
	Version            string   `json:"version"`
	PublishedAt        string   `json:"publishedAt"`
	Summary            string   `json:"summary"`
	Risk               string   `json:"risk"`
	MigrationRequired  bool     `json:"migrationRequired"`
	AffectedComponents []string `json:"affectedComponents"`
}

// GET /repositories/{owner}/{repo}/releases
func listReleases(ctx context.Context, deps Deps, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	repository := fmt.Sprintf("%s/%s", pathParam(req, "owner"), pathParam(req, "repo"))
	limit := queryParamInt(req, "limit", defaultPageSize)
	cursor := req.QueryStringParameters["cursor"]

	records, nextCursor, err := deps.Releases.List(ctx, repository, limit, cursor)
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}

	summaries := make([]releaseSummary, 0, len(records))
	for _, rec := range records {
		summaries = append(summaries, releaseSummary{
			Version:            rec.Version,
			PublishedAt:        formatTime(rec.PublishedAt),
			Summary:            rec.Summary,
			Risk:               string(rec.Risk),
			MigrationRequired:  rec.MigrationRequired,
			AffectedComponents: rec.AffectedComponents,
		})
	}

	body := map[string]any{"releases": summaries}
	if nextCursor != "" {
		body["nextCursor"] = nextCursor
	}
	return jsonResponse(200, body), nil
}

// GET /repositories/{owner}/{repo}/releases/{version}
func getRelease(ctx context.Context, deps Deps, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	repository := fmt.Sprintf("%s/%s", pathParam(req, "owner"), pathParam(req, "repo"))
	version := pathParam(req, "version")

	rec, err := deps.Releases.Get(ctx, repository, version)
	if err != nil {
		return events.APIGatewayProxyResponse{}, err
	}
	if rec == nil {
		return jsonResponse(404, map[string]string{"error": "release not found"}), nil
	}
	return jsonResponse(200, rec), nil
}

// POST /repositories/{owner}/{repo}/check - manual trigger for
// Observability Playground use (spec section 13). API Gateway enforces
// IAM auth on this route (infra/terraform/apigateway.tf); this handler
// assumes any caller that reached it is already authorized.
func triggerCheck(ctx context.Context, deps Deps, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	repository := fmt.Sprintf("%s/%s", pathParam(req, "owner"), pathParam(req, "repo"))

	input, err := json.Marshal(map[string]any{
		"repositories": []string{repository},
		// Manual triggers originate from an authenticated API caller
		// who may itself be inside a trace (e.g. an internal ops tool);
		// forward it the same way Step Functions states forward trace
		// context between tasks (see internal/telemetry/propagation.go).
		"traceContext": telemetry.InjectMap(ctx),
	})
	if err != nil {
		return events.APIGatewayProxyResponse{}, apperr.New(apperr.InvalidInput, false, err)
	}

	out, err := deps.SFN.StartExecution(ctx, &sfn.StartExecutionInput{
		StateMachineArn: aws.String(deps.StateMachineArn),
		Input:           aws.String(string(input)),
	})
	if err != nil {
		return events.APIGatewayProxyResponse{}, apperr.New(apperr.StorageError, true, err)
	}

	return jsonResponse(202, map[string]string{"executionArn": aws.ToString(out.ExecutionArn)}), nil
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.Format(time.RFC3339)
}
