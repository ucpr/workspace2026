// Package awsconfig provides a single place to load an aws.Config with
// OpenTelemetry instrumentation attached, so every AWS SDK call
// (DynamoDB, S3, Bedrock, SFN) automatically produces a client span
// (spec section 16/section 2: "AWS SDK instrumentation") without each
// call site needing to know about it.
package awsconfig

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"go.opentelemetry.io/contrib/instrumentation/github.com/aws/aws-sdk-go-v2/otelaws"
)

// Load returns an aws.Config with the default credential/region chain
// and otelaws middleware installed.
func Load(ctx context.Context) (aws.Config, error) {
	cfg, err := config.LoadDefaultConfig(ctx)
	if err != nil {
		return aws.Config{}, err
	}
	otelaws.AppendMiddlewares(&cfg.APIOptions)
	return cfg, nil
}
