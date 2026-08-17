package bedrock

import (
	"testing"

	"github.com/ucpr/releascope/internal/apperr"
)

func TestParseResponse_Valid(t *testing.T) {
	raw := []byte(`{
		"stop_reason": "tool_use",
		"content": [
			{"type": "text", "text": "Let me analyze this."},
			{"type": "tool_use", "name": "emit_release_analysis", "input": {
				"summary": "Adds prometheus receiver improvements.",
				"breakingChanges": [],
				"notableChanges": [{"component": "prometheusreceiver", "title": "New feature", "description": "..."}],
				"deprecations": [],
				"risk": "low",
				"migrationRequired": false,
				"recommendation": "Safe to upgrade."
			}}
		]
	}`)

	resp, err := parseResponse(raw)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Analysis.Risk != "low" {
		t.Errorf("risk = %q, want low", resp.Analysis.Risk)
	}
	if len(resp.Analysis.NotableChanges) != 1 {
		t.Fatalf("got %d notable changes, want 1", len(resp.Analysis.NotableChanges))
	}
	if string(resp.RawJSON) != string(raw) {
		t.Error("RawJSON should preserve the original response for debugging")
	}
}

func TestParseResponse_MissingToolUse(t *testing.T) {
	raw := []byte(`{"stop_reason": "end_turn", "content": [{"type": "text", "text": "I cannot help with that."}]}`)

	_, err := parseResponse(raw)
	if err == nil {
		t.Fatal("expected error")
	}
	if got := apperr.CategoryOf(err); got != apperr.BedrockInvalidResponse {
		t.Errorf("category = %s, want %s", got, apperr.BedrockInvalidResponse)
	}
	if !apperr.IsRetryable(err) {
		t.Error("expected invalid response to be retryable (model output is stochastic)")
	}
}

func TestParseResponse_MalformedEnvelope(t *testing.T) {
	_, err := parseResponse([]byte(`not json`))
	if got := apperr.CategoryOf(err); got != apperr.BedrockInvalidResponse {
		t.Errorf("category = %s, want %s", got, apperr.BedrockInvalidResponse)
	}
}

func TestParseResponse_MalformedToolInput(t *testing.T) {
	raw := []byte(`{
		"stop_reason": "tool_use",
		"content": [{"type": "tool_use", "name": "emit_release_analysis", "input": "not-an-object"}]
	}`)
	_, err := parseResponse(raw)
	if got := apperr.CategoryOf(err); got != apperr.BedrockInvalidResponse {
		t.Errorf("category = %s, want %s", got, apperr.BedrockInvalidResponse)
	}
}
