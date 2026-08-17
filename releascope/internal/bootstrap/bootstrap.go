// Package bootstrap is the one place every cmd/* binary's main()
// initializes shared infrastructure (config, logging, telemetry, AWS
// SDK config), so each Lambda's main.go stays a short list of "wire
// dependencies, define handler, call lambda.Start" (spec section 14:
// "Lambda Handler は薄く保つ").
package bootstrap

import (
	"context"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"

	"github.com/ucpr/releascope/internal/awsconfig"
	"github.com/ucpr/releascope/internal/config"
	"github.com/ucpr/releascope/internal/logging"
	"github.com/ucpr/releascope/internal/telemetry"
)

// App holds every shared dependency a cmd/* binary needs.
type App struct {
	Config   config.Config
	AWS      aws.Config
	Shutdown telemetry.ShutdownFunc
}

// Init loads configuration, sets up structured logging and the
// OpenTelemetry SDK, and loads an OTel-instrumented AWS SDK config.
// defaultServiceName becomes the OTel `service.name` resource attribute
// (distinguishing e.g. "releascope-check-release" from "releascope-api"
// in a shared trace backend) unless the operator overrides it via the
// OTEL_SERVICE_NAME environment variable (spec section 25).
func Init(ctx context.Context, defaultServiceName string) (*App, error) {
	cfg := config.FromEnv()
	logging.SetDefault(cfg.LogLevel)

	serviceName := cfg.ServiceName
	if serviceName == "" {
		serviceName = defaultServiceName
	}

	shutdown, err := telemetry.Setup(ctx, telemetry.Config{
		ServiceName:  serviceName,
		Environment:  cfg.Environment,
		OTLPEndpoint: cfg.OTLPEndpoint,
	})
	if err != nil {
		return nil, err
	}

	awsCfg, err := awsconfig.Load(ctx)
	if err != nil {
		return nil, err
	}

	if cfg.GitHubToken == "" && cfg.GitHubTokenParam != "" {
		token, err := resolveGitHubToken(ctx, ssm.NewFromConfig(awsCfg), cfg.GitHubTokenParam)
		if err != nil {
			return nil, err
		}
		cfg.GitHubToken = token
	}

	return &App{Config: cfg, AWS: awsCfg, Shutdown: shutdown}, nil
}

func resolveGitHubToken(ctx context.Context, client *ssm.Client, paramName string) (string, error) {
	out, err := client.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           aws.String(paramName),
		WithDecryption: aws.Bool(true),
	})
	if err != nil {
		return "", err
	}
	return aws.ToString(out.Parameter.Value), nil
}
