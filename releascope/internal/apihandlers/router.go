// Package apihandlers implements the read API from spec section 12 and
// the manual-trigger endpoint from section 13, behind an API Gateway
// REST API with explicit resources (not a {proxy+} catch-all), so path
// parameters like {owner}/{repo}/{version} arrive already parsed in
// events.APIGatewayProxyRequest.PathParameters.
package apihandlers

import (
	"context"
	"encoding/json"
	"strconv"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/ucpr/releascope/internal/apperr"
	"github.com/ucpr/releascope/internal/storage"
	"github.com/ucpr/releascope/internal/telemetry"
)

var tracer = otel.Tracer("releascope/internal/apihandlers")

// Deps are the API Lambda's collaborators.
type Deps struct {
	Repos               *storage.RepositoryStore
	Releases            *storage.ReleaseStore
	SFN                 *sfn.Client
	StateMachineArn     string
	DefaultRepositories []string
}

const defaultPageSize = 20

// Route dispatches an API Gateway proxy request to the matching
// handler. Trace context is extracted from the incoming HTTP headers
// first (spec section 17: genuine W3C Trace Context propagation across
// the client -> API Gateway -> Lambda boundary when the caller sends a
// `traceparent` header; a no-op, fresh trace root otherwise).
func Route(ctx context.Context, deps Deps, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	ctx = telemetry.ExtractHTTPHeaders(ctx, req.Headers)
	ctx, span := tracer.Start(ctx, "api.request", trace.WithAttributes(
		attribute.String("http.request.method", req.HTTPMethod),
		attribute.String("http.route", req.Resource),
	))
	defer span.End()

	var resp events.APIGatewayProxyResponse
	var err error

	switch {
	case req.Resource == "/repositories" && req.HTTPMethod == "GET":
		resp, err = listRepositories(ctx, deps)
	case req.Resource == "/repositories/{owner}/{repo}/releases" && req.HTTPMethod == "GET":
		resp, err = listReleases(ctx, deps, req)
	case req.Resource == "/repositories/{owner}/{repo}/releases/{version}" && req.HTTPMethod == "GET":
		resp, err = getRelease(ctx, deps, req)
	case req.Resource == "/repositories/{owner}/{repo}/check" && req.HTTPMethod == "POST":
		resp, err = triggerCheck(ctx, deps, req)
	default:
		resp = jsonResponse(404, map[string]string{"error": "not found"})
	}

	if err != nil {
		apperr.RecordSpanError(ctx, err)
		return errorResponse(err), nil
	}

	span.SetAttributes(attribute.Int("http.response.status_code", resp.StatusCode))
	return resp, nil
}

func errorResponse(err error) events.APIGatewayProxyResponse {
	status := 500
	if apperr.CategoryOf(err) == apperr.InvalidInput {
		status = 400
	}
	return jsonResponse(status, map[string]string{"error": err.Error()})
}

func jsonResponse(status int, body any) events.APIGatewayProxyResponse {
	b, err := json.Marshal(body)
	if err != nil {
		return events.APIGatewayProxyResponse{StatusCode: 500, Body: `{"error":"internal error"}`}
	}
	return events.APIGatewayProxyResponse{
		StatusCode: status,
		Headers:    map[string]string{"Content-Type": "application/json"},
		Body:       string(b),
	}
}

func pathParam(req events.APIGatewayProxyRequest, name string) string {
	return req.PathParameters[name]
}

func queryParamInt(req events.APIGatewayProxyRequest, name string, def int32) int32 {
	v, ok := req.QueryStringParameters[name]
	if !ok {
		return def
	}
	n, err := strconv.ParseInt(v, 10, 32)
	if err != nil || n <= 0 {
		return def
	}
	return int32(n)
}
