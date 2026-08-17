// Command persist-release implements the PersistRelease Step Functions
// task (spec section 10/30): the final step that atomically writes the
// release record and advances the repository's latest-version pointer.
package main

import (
	"context"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"go.opentelemetry.io/otel"

	"github.com/ucpr/releascope/internal/bootstrap"
	"github.com/ucpr/releascope/internal/lambdautil"
	"github.com/ucpr/releascope/internal/service"
	"github.com/ucpr/releascope/internal/storage"
)

var tracer = otel.Tracer("releascope/cmd/persist-release")

func main() {
	ctx := context.Background()

	app, err := bootstrap.Init(ctx, "releascope-persist-release")
	if err != nil {
		panic(err)
	}
	defer app.Shutdown(ctx)

	deps := service.PersistDeps{
		Releases: storage.NewReleaseStore(dynamodb.NewFromConfig(app.AWS), app.Config.ReleaseTableName, app.Config.RepositoryTableName),
	}

	handler := func(ctx context.Context, in service.PersistInput) (*service.PersistOutput, error) {
		ctx, span := lambdautil.StartInvocation(ctx, tracer, "persist-release")
		defer span.End()

		out, err := service.Persist(ctx, deps, in)
		return out, lambdautil.WrapError(err)
	}

	lambda.Start(handler)
}
