// Command analyze-release implements the AnalyzeWithLLM Step Functions
// task (spec section 8).
package main

import (
	"context"

	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"go.opentelemetry.io/otel"

	"github.com/ucpr/releascope/internal/bedrock"
	"github.com/ucpr/releascope/internal/bootstrap"
	"github.com/ucpr/releascope/internal/lambdautil"
	"github.com/ucpr/releascope/internal/service"
	"github.com/ucpr/releascope/internal/storage"
)

var tracer = otel.Tracer("releascope/cmd/analyze-release")

func main() {
	ctx := context.Background()

	app, err := bootstrap.Init(ctx, "releascope-analyze-release")
	if err != nil {
		panic(err)
	}
	defer app.Shutdown(ctx)

	deps := service.AnalyzeDeps{
		Bedrock: bedrock.NewClient(bedrockruntime.NewFromConfig(app.AWS), app.Config.BedrockModelID),
		Data:    storage.NewDataStore(s3.NewFromConfig(app.AWS), app.Config.DataBucketName),
	}

	handler := func(ctx context.Context, in service.AnalyzeInput) (*service.AnalyzeOutput, error) {
		ctx, span := lambdautil.StartInvocation(ctx, tracer, "analyze-release")
		defer span.End()

		out, err := service.Analyze(ctx, deps, in)
		return out, lambdautil.WrapError(err)
	}

	lambda.Start(handler)
}
