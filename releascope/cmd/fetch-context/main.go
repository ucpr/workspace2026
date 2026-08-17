// Command fetch-context implements all three branches of the
// FetchReleaseContext Parallel state (Compare, FetchPRs, ExtractChanges
// - spec section 5), selected at runtime by the `action` field of the
// input event. Running one Lambda function with an action router,
// rather than three separate functions, keeps the deployable unit count
// down while Step Functions still schedules the three branches as
// independent, concurrently-retried Task states (see
// infra/terraform/statemachine.asl.json).
package main

import (
	"context"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.opentelemetry.io/otel"

	"github.com/ucpr/releascope/internal/bootstrap"
	"github.com/ucpr/releascope/internal/github"
	"github.com/ucpr/releascope/internal/lambdautil"
	"github.com/ucpr/releascope/internal/service"
	"github.com/ucpr/releascope/internal/storage"
)

var tracer = otel.Tracer("releascope/cmd/fetch-context")

func main() {
	ctx := context.Background()

	app, err := bootstrap.Init(ctx, "releascope-fetch-context")
	if err != nil {
		panic(err)
	}
	defer app.Shutdown(ctx)

	deps := service.FetchContextDeps{
		GitHub: github.NewClient(app.Config.GitHubAPIBaseURL, app.Config.GitHubToken),
		Data:   storage.NewDataStore(s3.NewFromConfig(app.AWS), app.Config.DataBucketName),
	}

	handler := func(ctx context.Context, in service.FetchContextInput) (*service.FetchContextOutput, error) {
		ctx, span := lambdautil.StartInvocation(ctx, tracer, "fetch-context")
		defer span.End()

		out, err := service.FetchContext(ctx, deps, in)
		return out, lambdautil.WrapError(err)
	}

	lambda.Start(handler)
}
