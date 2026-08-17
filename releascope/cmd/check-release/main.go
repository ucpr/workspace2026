// Command check-release is the CheckLatestRelease Step Functions task
// (spec section 5/29-Phase1). It also doubles as the `make check-release`
// local CLI entrypoint per spec section 24: when not running inside the
// real Lambda runtime, main() invokes the handler directly against
// os.Stdout instead of calling lambda.Start.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.opentelemetry.io/otel"

	"github.com/ucpr/releascope/internal/bootstrap"
	"github.com/ucpr/releascope/internal/github"
	"github.com/ucpr/releascope/internal/lambdautil"
	"github.com/ucpr/releascope/internal/service"
	"github.com/ucpr/releascope/internal/storage"
)

var tracer = otel.Tracer("releascope/cmd/check-release")

func main() {
	ctx := context.Background()

	app, err := bootstrap.Init(ctx, "releascope-check-release")
	if err != nil {
		fmt.Fprintln(os.Stderr, "bootstrap: "+err.Error())
		os.Exit(1)
	}
	defer app.Shutdown(ctx)

	deps := service.CheckReleaseDeps{
		GitHub: github.NewClient(app.Config.GitHubAPIBaseURL, app.Config.GitHubToken),
		Repos:  storage.NewRepositoryStore(dynamodb.NewFromConfig(app.AWS), app.Config.RepositoryTableName),
		Data:   storage.NewDataStore(s3.NewFromConfig(app.AWS), app.Config.DataBucketName),
	}

	handler := func(ctx context.Context, in service.CheckReleaseInput) (*service.CheckReleaseOutput, error) {
		ctx, span := lambdautil.StartInvocation(ctx, tracer, "check-release")
		defer span.End()

		out, err := service.CheckRelease(ctx, deps, in)
		return out, lambdautil.WrapError(err)
	}

	if !lambdautil.IsLambdaRuntime() {
		runLocal(ctx, app, handler)
		return
	}
	lambda.Start(handler)
}

// runLocal implements `make check-release REPOSITORY=...`.
func runLocal(ctx context.Context, app *bootstrap.App, handler func(context.Context, service.CheckReleaseInput) (*service.CheckReleaseOutput, error)) {
	repo := os.Getenv("REPOSITORY")
	if repo == "" && len(app.Config.Repositories) > 0 {
		repo = app.Config.Repositories[0]
	}
	if repo == "" {
		fmt.Fprintln(os.Stderr, "usage: REPOSITORY=owner/repo make check-release")
		os.Exit(1)
	}

	out, err := handler(ctx, service.CheckReleaseInput{Repository: repo})
	if err != nil {
		slog.ErrorContext(ctx, "check-release failed", "error", err)
		os.Exit(1)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(out)
}
