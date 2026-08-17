// Package config centralizes environment variable driven configuration
// for all Releascope Lambdas and the local CLI. Keeping this in one place
// means Lambda handlers stay thin and unit tests can construct a Config
// directly instead of mutating process environment.
package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds every environment-driven setting used across Releascope
// binaries. Not every field is required by every binary; each cmd/*
// entrypoint reads only what it needs.
type Config struct {
	AWSRegion string

	// Repositories is the default set of "owner/repo" strings this
	// deployment tracks. Individual Step Functions executions pass a
	// single repository explicitly, so this is mainly used by the local
	// CLI and the API's /repositories listing fallback.
	Repositories []string

	RepositoryTableName string
	ReleaseTableName    string
	DataBucketName      string

	GitHubTokenParam string // SSM Parameter Store name holding the GitHub token
	GitHubToken      string // resolved token value (populated by caller after SSM lookup, or set directly for local dev)
	GitHubAPIBaseURL string

	BedrockModelID string
	BedrockRegion  string

	StateMachineArn string

	OTLPEndpoint string
	ServiceName  string
	Environment  string

	LogLevel string

	FaultMode string
}

// FromEnv loads configuration from process environment variables,
// applying sane defaults for local development.
func FromEnv() Config {
	return Config{
		AWSRegion:           getEnv("AWS_REGION", "us-east-1"),
		Repositories:        splitCSV(getEnv("REPOSITORIES", "open-telemetry/opentelemetry-collector,open-telemetry/opentelemetry-collector-contrib")),
		RepositoryTableName: getEnv("REPOSITORY_TABLE_NAME", "releascope-repositories"),
		ReleaseTableName:    getEnv("RELEASE_TABLE_NAME", "releascope-releases"),
		DataBucketName:      getEnv("DATA_BUCKET_NAME", "releascope-data"),
		GitHubTokenParam:    getEnv("GITHUB_TOKEN_PARAM", ""),
		GitHubToken:         getEnv("GITHUB_TOKEN", ""),
		GitHubAPIBaseURL:    getEnv("GITHUB_API_BASE_URL", "https://api.github.com"),
		BedrockModelID:      getEnv("BEDROCK_MODEL_ID", "anthropic.claude-3-5-sonnet-20241022-v2:0"),
		BedrockRegion:       getEnv("BEDROCK_REGION", getEnv("AWS_REGION", "us-east-1")),
		StateMachineArn:     getEnv("STATE_MACHINE_ARN", ""),
		OTLPEndpoint:        getEnv("OTEL_EXPORTER_OTLP_ENDPOINT", ""),
		ServiceName:         getEnv("OTEL_SERVICE_NAME", ""),
		Environment:         getEnv("ENVIRONMENT", "dev"),
		LogLevel:            getEnv("LOG_LEVEL", "info"),
		FaultMode:           getEnv("FAULT_MODE", ""),
	}
}

func getEnv(key, def string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return def
}

func splitCSV(v string) []string {
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// GetEnvBool reads a boolean environment variable used by fault injection
// toggles (e.g. FAULT_GITHUB_TIMEOUT=true).
func GetEnvBool(key string) bool {
	v, ok := os.LookupEnv(key)
	if !ok {
		return false
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return false
	}
	return b
}

// SplitOwnerRepo splits "owner/repo" into its two parts, returning an
// error for malformed input so callers fail fast on bad configuration.
func SplitOwnerRepo(repository string) (owner, repo string, err error) {
	parts := strings.SplitN(repository, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("invalid repository %q, expected owner/repo", repository)
	}
	return parts[0], parts[1], nil
}
