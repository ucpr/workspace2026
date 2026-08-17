// Package fault centralizes fault injection for the Releascope
// Observability Playground. Every client package (github, bedrock,
// storage) calls into this package at its boundary instead of
// implementing its own ad-hoc failure simulation, so business logic
// never branches on "am I in test mode".
//
// Two activation styles are supported and can be combined:
//
//   - FAULT_MODE=github_timeout            (single named mode)
//   - FAULT_GITHUB_TIMEOUT=true            (per-fault boolean flags)
package fault

import (
	"context"
	"errors"
	"os"
	"strconv"
	"time"
)

// Kind identifies a specific injectable fault.
type Kind string

const (
	GitHubTimeout          Kind = "github_timeout"
	GitHubRateLimit        Kind = "github_rate_limit"
	GitHubServerError      Kind = "github_server_error"
	BedrockTimeout         Kind = "bedrock_timeout"
	BedrockThrottling      Kind = "bedrock_throttling"
	BedrockInvalidResponse Kind = "bedrock_invalid_response"
	DynamoDBError          Kind = "dynamodb_error"
)

// envFlag maps a Kind to the boolean env var that toggles it individually.
var envFlag = map[Kind]string{
	GitHubTimeout:          "FAULT_GITHUB_TIMEOUT",
	GitHubRateLimit:        "FAULT_GITHUB_RATE_LIMIT",
	GitHubServerError:      "FAULT_GITHUB_SERVER_ERROR",
	BedrockTimeout:         "FAULT_BEDROCK_TIMEOUT",
	BedrockThrottling:      "FAULT_BEDROCK_THROTTLING",
	BedrockInvalidResponse: "FAULT_BEDROCK_INVALID_RESPONSE",
	DynamoDBError:          "FAULT_DYNAMODB_ERROR",
}

// ErrInjected is wrapped by every fault-injected error so callers/tests
// can distinguish injected faults from organic failures with errors.Is.
var ErrInjected = errors.New("fault: injected failure")

// Active reports whether the given fault kind is currently enabled, via
// either FAULT_MODE=<kind> or the kind's dedicated FAULT_<KIND>=true flag.
func Active(kind Kind) bool {
	if mode := os.Getenv("FAULT_MODE"); mode != "" && Kind(mode) == kind {
		return true
	}
	if flag, ok := envFlag[kind]; ok {
		if v, err := strconv.ParseBool(os.Getenv(flag)); err == nil && v {
			return true
		}
	}
	return false
}

// MaybeInject blocks for the given delay (simulating a timeout) and/or
// returns an injected error when the given fault kind is active. Call
// this at the very top of a client method, before any real network I/O.
//
// The delay is bounded by ctx cancellation so injected timeouts still
// respect caller-supplied deadlines instead of hanging tests forever.
func MaybeInject(ctx context.Context, kind Kind, delay time.Duration) error {
	if !Active(kind) {
		return nil
	}
	if delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timer.C:
		}
	}
	return &InjectedError{Kind: kind}
}

// InjectedError is returned when a fault has been injected.
type InjectedError struct {
	Kind Kind
}

func (e *InjectedError) Error() string {
	return "fault: injected " + string(e.Kind)
}

func (e *InjectedError) Unwrap() error {
	return ErrInjected
}
