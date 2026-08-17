// Command api implements the read API and manual-trigger endpoint
// (spec sections 12-13) behind API Gateway's Lambda proxy integration.
package main

import (
	"context"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/sfn"
	"go.opentelemetry.io/otel"

	"github.com/ucpr/releascope/internal/apihandlers"
	"github.com/ucpr/releascope/internal/bootstrap"
	"github.com/ucpr/releascope/internal/lambdautil"
	"github.com/ucpr/releascope/internal/storage"
)

var tracer = otel.Tracer("releascope/cmd/api")

func main() {
	ctx := context.Background()

	app, err := bootstrap.Init(ctx, "releascope-api")
	if err != nil {
		panic(err)
	}
	defer app.Shutdown(ctx)

	ddb := dynamodb.NewFromConfig(app.AWS)
	deps := apihandlers.Deps{
		Repos:               storage.NewRepositoryStore(ddb, app.Config.RepositoryTableName),
		Releases:            storage.NewReleaseStore(ddb, app.Config.ReleaseTableName, app.Config.RepositoryTableName),
		SFN:                 sfn.NewFromConfig(app.AWS),
		StateMachineArn:     app.Config.StateMachineArn,
		DefaultRepositories: app.Config.Repositories,
	}

	handler := func(ctx context.Context, req events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
		ctx, span := lambdautil.StartInvocation(ctx, tracer, "api")
		defer span.End()

		return apihandlers.Route(ctx, deps, req)
	}

	lambda.Start(handler)
}
