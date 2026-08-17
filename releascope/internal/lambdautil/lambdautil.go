// Package lambdautil holds the tiny amount of glue code every Lambda
// handler in cmd/* needs, keeping the handlers themselves "thin" per
// spec section 14.
package lambdautil

import (
	"context"
	"os"
	"sync/atomic"

	"github.com/aws/aws-lambda-go/lambda/messages"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/ucpr/releascope/internal/apperr"
)

// WrapError converts a categorized *apperr.Error into the AWS Lambda Go
// runtime's messages.InvokeResponse_Error so its Type field - not a
// reflected Go struct name - becomes the Lambda's `errorType`.
//
// This matters because Step Functions Retry/Catch match ErrorEquals
// against exactly that errorType. Every error in this codebase is a
// *apperr.Error, so without this conversion the runtime's default
// reflection-based naming (lambda/errors.go:getErrorType) would report
// the same generic type name ("Error") for github_rate_limit,
// bedrock_throttling, storage_error, etc. - making them indistinguishable
// to a Retry/Catch ErrorEquals clause.
func WrapError(err error) error {
	if err == nil {
		return nil
	}
	return messages.InvokeResponse_Error{
		Type:    string(apperr.CategoryOf(err)),
		Message: err.Error(),
	}
}

var coldStart atomic.Bool

func init() { coldStart.Store(true) }

// StartInvocation opens the top-level span for one Lambda invocation,
// tagged with faas.coldstart (spec section 21, Scenario 013: "Lambda
// cold start" is one of the fault/observability scenarios this project
// exists to make visible). It intentionally does not pull in the
// otellambda contrib package: that package's automatic event-to-carrier
// extraction would conflict with the manual traceContext propagation
// internal/service already performs for Step-Functions-invoked
// Lambdas (see README's Trace Propagation section), and a single
// hand-rolled span here covers what this project actually needs.
func StartInvocation(ctx context.Context, tracer trace.Tracer, functionName string) (context.Context, trace.Span) {
	isCold := coldStart.Swap(false)
	return tracer.Start(ctx, functionName, trace.WithAttributes(
		attribute.String("cloud.provider", "aws"),
		attribute.String("cloud.platform", "aws_lambda"),
		attribute.String("faas.name", functionName),
		attribute.Bool("faas.coldstart", isCold),
	))
}

// IsLambdaRuntime reports whether the process is running inside the
// real AWS Lambda runtime, as opposed to the local CLI / `make
// check-release` entrypoint (spec section 24).
func IsLambdaRuntime() bool {
	return os.Getenv("AWS_LAMBDA_RUNTIME_API") != ""
}
