package telemetry

import (
	"context"
	"sync/atomic"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/metric"
)

// meter is obtained from the OTel global meter provider. Instruments
// created here work correctly even before Setup() runs: the OTel Go API
// delegates instrument calls to whatever MeterProvider is registered
// later (see go.opentelemetry.io/otel's global package), which is why
// package-level instrument creation is safe.
var meter = otel.Meter("releascope")

// Section 19 of the spec enumerates this exact metric set.
var (
	ReleaseCheckTotal, _          = meter.Int64Counter("release_check_total", metric.WithDescription("Number of release-check invocations"))
	ReleaseNewTotal, _            = meter.Int64Counter("release_new_total", metric.WithDescription("Number of new releases detected"))
	ReleaseAnalysisTotal, _       = meter.Int64Counter("release_analysis_total", metric.WithDescription("Number of LLM analysis attempts"))
	ReleaseAnalysisErrorsTotal, _ = meter.Int64Counter("release_analysis_errors_total", metric.WithDescription("Number of failed LLM analysis attempts"))
	ReleaseAnalysisDuration, _    = meter.Float64Histogram("release_analysis_duration", metric.WithUnit("s"), metric.WithDescription("LLM analysis duration"))

	GitHubRequestTotal, _       = meter.Int64Counter("github_request_total", metric.WithDescription("GitHub API requests"))
	GitHubRequestErrorsTotal, _ = meter.Int64Counter("github_request_errors_total", metric.WithDescription("Failed GitHub API requests"))

	BedrockRequestTotal, _       = meter.Int64Counter("bedrock_request_total", metric.WithDescription("Bedrock InvokeModel requests"))
	BedrockRequestErrorsTotal, _ = meter.Int64Counter("bedrock_request_errors_total", metric.WithDescription("Failed Bedrock InvokeModel requests"))
	BedrockRequestDuration, _    = meter.Float64Histogram("bedrock_request_duration", metric.WithUnit("s"), metric.WithDescription("Bedrock InvokeModel duration"))
)

var githubRateLimitRemainingValue atomic.Int64

func init() {
	// github_rate_limit_remaining changes only when a GitHub response
	// arrives, so it is modeled as an observable gauge fed by an atomic
	// value rather than a synchronous instrument recorded on every call.
	_, _ = meter.Int64ObservableGauge(
		"github_rate_limit_remaining",
		metric.WithDescription("Remaining GitHub API rate limit as of the last response"),
		metric.WithInt64Callback(func(_ context.Context, o metric.Int64Observer) error {
			o.Observe(githubRateLimitRemainingValue.Load())
			return nil
		}),
	)
}

// SetGitHubRateLimitRemaining records the most recently observed
// X-RateLimit-Remaining value from the GitHub API.
func SetGitHubRateLimitRemaining(v int64) {
	githubRateLimitRemainingValue.Store(v)
}
