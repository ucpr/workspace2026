// Package apperr defines the closed set of error categories Releascope
// classifies failures into (section 20 of the spec). Classifying errors
// at the boundary where they occur lets callers make retry decisions and
// lets telemetry record a consistent `error.type` span attribute instead
// of raw, high-cardinality error strings.
package apperr

import (
	"context"
	"errors"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Category is a stable, low-cardinality error classification.
type Category string

const (
	GitHubRateLimit   Category = "github_rate_limit"
	GitHubTimeout     Category = "github_timeout"
	GitHubServerError Category = "github_server_error"
	GitHubClientError Category = "github_client_error"

	BedrockThrottling      Category = "bedrock_throttling"
	BedrockTimeout         Category = "bedrock_timeout"
	BedrockInvalidResponse Category = "bedrock_invalid_response"

	StorageError Category = "storage_error"
	InvalidInput Category = "invalid_input"
	Unknown      Category = "unknown"
)

// Error wraps an underlying error with a stable category and marks
// whether the caller should retry it. Step Functions Retry/Catch blocks
// key off the Category via the error name surfaced to the state machine
// (see cmd/*/main.go, which set the Lambda error type to this value).
type Error struct {
	Category  Category
	Retryable bool
	Err       error
}

func (e *Error) Error() string {
	if e.Err == nil {
		return string(e.Category)
	}
	return string(e.Category) + ": " + e.Err.Error()
}

func (e *Error) Unwrap() error { return e.Err }

// New builds a categorized error.
func New(category Category, retryable bool, err error) *Error {
	return &Error{Category: category, Retryable: retryable, Err: err}
}

// CategoryOf extracts the Category from err if it (or something it
// wraps) is an *Error, otherwise returns Unknown.
func CategoryOf(err error) Category {
	var ae *Error
	if errors.As(err, &ae) {
		return ae.Category
	}
	return Unknown
}

// IsRetryable reports whether err was classified as retryable.
func IsRetryable(err error) bool {
	var ae *Error
	if errors.As(err, &ae) {
		return ae.Retryable
	}
	return false
}

// RecordSpanError sets the span status to Error and attaches an
// exception event plus a low-cardinality error.type attribute,
// following OTel semantic conventions.
func RecordSpanError(ctx context.Context, err error) {
	if err == nil {
		return
	}
	span := trace.SpanFromContext(ctx)
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
	span.SetAttributes(attribute.String("error.type", string(CategoryOf(err))))
}
