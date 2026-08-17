// Package bedrock implements Releascope's Amazon Bedrock client. It
// requests structured output via Anthropic's tool-use mechanism (a
// forced tool call whose input_schema is the exact JSON shape Releascope
// needs) rather than asking the model to "reply with JSON" and hoping,
// which is what makes decode failures rare but still possible - and
// therefore still worth handling as a retryable error (spec section 8).
package bedrock

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/smithy-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/ucpr/releascope/internal/apperr"
	"github.com/ucpr/releascope/internal/fault"
	"github.com/ucpr/releascope/internal/telemetry"
)

var tracer = otel.Tracer("releascope/internal/bedrock")

const anthropicVersion = "bedrock-2023-05-31"

// API is the interface consumed by application code, allowing tests to
// substitute a mock instead of calling Bedrock.
type API interface {
	AnalyzeRelease(ctx context.Context, req AnalyzeRequest) (*Response, error)
}

// AnalyzeRequest carries the release context assembled by
// cmd/analyze-release into a single Bedrock prompt. Callers are
// responsible for keeping the fields within Bedrock's token limit (spec
// section 7) - typically by truncating commit/PR lists before calling.
type AnalyzeRequest struct {
	Repository      string
	PreviousVersion string
	CurrentVersion  string
	ReleaseNotes    string
	PullRequests    []PullRequestSummary
	ChangedFiles    []string
}

type PullRequestSummary struct {
	Number int
	Title  string
	Body   string
}

// Response bundles the parsed structured output with the raw model
// response so callers can persist both (spec section 8: "LLM の raw
// response もデバッグ可能な形で保持する").
type Response struct {
	Analysis Analysis
	RawJSON  []byte
}

// Analysis matches the exact schema from spec section 8.
type Analysis struct {
	Summary           string           `json:"summary"`
	BreakingChanges   []BreakingChange `json:"breakingChanges"`
	NotableChanges    []NotableChange  `json:"notableChanges"`
	Deprecations      []Deprecation    `json:"deprecations"`
	Risk              string           `json:"risk"`
	MigrationRequired bool             `json:"migrationRequired"`
	Recommendation    string           `json:"recommendation"`
}

type BreakingChange struct {
	Component   string `json:"component"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Migration   string `json:"migration"`
}

type NotableChange struct {
	Component   string `json:"component"`
	Title       string `json:"title"`
	Description string `json:"description"`
}

type Deprecation struct {
	Component   string `json:"component"`
	Description string `json:"description"`
}

// Client wraps bedrockruntime.Client.
type Client struct {
	client  *bedrockruntime.Client
	modelID string
}

func NewClient(client *bedrockruntime.Client, modelID string) *Client {
	return &Client{client: client, modelID: modelID}
}

var _ API = (*Client)(nil)

// messagesRequest/messagesResponse model the subset of Anthropic's
// Messages API (as served through Bedrock) that Releascope uses.
type messagesRequest struct {
	AnthropicVersion string     `json:"anthropic_version"`
	MaxTokens        int        `json:"max_tokens"`
	Messages         []message  `json:"messages"`
	Tools            []tool     `json:"tools"`
	ToolChoice       toolChoice `json:"tool_choice"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type tool struct {
	Name        string         `json:"name"`
	Description string         `json:"description"`
	InputSchema map[string]any `json:"input_schema"`
}

type toolChoice struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

type messagesResponse struct {
	Content    []contentBlock `json:"content"`
	StopReason string         `json:"stop_reason"`
}

type contentBlock struct {
	Type  string          `json:"type"`
	Text  string          `json:"text,omitempty"`
	Name  string          `json:"name,omitempty"`
	Input json.RawMessage `json:"input,omitempty"`
}

// AnalyzeRelease calls Bedrock and returns the parsed structured
// output. A malformed/missing tool_use block is returned as a
// BedrockInvalidResponse error, which Step Functions Retry can use to
// re-invoke the Lambda and try the (stochastic) model again.
func (c *Client) AnalyzeRelease(ctx context.Context, req AnalyzeRequest) (*Response, error) {
	ctx, span := tracer.Start(ctx, "bedrock.invoke_model", trace.WithAttributes(
		attribute.String("gen_ai.system", "aws.bedrock"),
		attribute.String("gen_ai.request.model", c.modelID),
		attribute.String("release.repository", req.Repository),
		attribute.String("release.version", req.CurrentVersion),
	))
	defer span.End()

	start := time.Now()
	telemetry.BedrockRequestTotal.Add(ctx, 1)

	resp, err := c.invoke(ctx, req)
	telemetry.BedrockRequestDuration.Record(ctx, time.Since(start).Seconds())
	if err != nil {
		telemetry.BedrockRequestErrorsTotal.Add(ctx, 1)
		apperr.RecordSpanError(ctx, err)
		return nil, err
	}

	span.SetAttributes(attribute.String("release.risk", resp.Analysis.Risk))
	return resp, nil
}

func (c *Client) invoke(ctx context.Context, req AnalyzeRequest) (*Response, error) {
	if err := fault.MaybeInject(ctx, fault.BedrockTimeout, 31*time.Second); err != nil {
		return nil, apperr.New(apperr.BedrockTimeout, true, err)
	}
	if err := fault.MaybeInject(ctx, fault.BedrockThrottling, 0); err != nil {
		return nil, apperr.New(apperr.BedrockThrottling, true, err)
	}
	if err := fault.MaybeInject(ctx, fault.BedrockInvalidResponse, 0); err != nil {
		return nil, apperr.New(apperr.BedrockInvalidResponse, true, err)
	}

	body := messagesRequest{
		AnthropicVersion: anthropicVersion,
		MaxTokens:        4096,
		Messages:         []message{{Role: "user", Content: buildPrompt(req)}},
		Tools: []tool{{
			Name:        toolName,
			Description: "Emit a structured analysis of the impact this release has on users of the library.",
			InputSchema: toolInputSchema,
		}},
		ToolChoice: toolChoice{Type: "tool", Name: toolName},
	}

	payload, err := json.Marshal(body)
	if err != nil {
		return nil, apperr.New(apperr.InvalidInput, false, err)
	}

	out, err := c.client.InvokeModel(ctx, &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(c.modelID),
		ContentType: aws.String("application/json"),
		Accept:      aws.String("application/json"),
		Body:        payload,
	})
	if err != nil {
		return nil, classifyBedrockError(err)
	}

	return parseResponse(out.Body)
}

// parseResponse extracts the forced tool_use block from a raw Bedrock
// Messages API response body. It is a pure function (no AWS SDK types)
// specifically so LLM response parsing - explicitly called out as a
// unit test target in spec section 23 - can be tested without a mock
// Bedrock client.
func parseResponse(raw []byte) (*Response, error) {
	var msgResp messagesResponse
	if err := json.Unmarshal(raw, &msgResp); err != nil {
		return nil, apperr.New(apperr.BedrockInvalidResponse, true, fmt.Errorf("decode bedrock envelope: %w", err))
	}

	for _, block := range msgResp.Content {
		if block.Type == "tool_use" && block.Name == toolName {
			var analysis Analysis
			if err := json.Unmarshal(block.Input, &analysis); err != nil {
				return nil, apperr.New(apperr.BedrockInvalidResponse, true, fmt.Errorf("decode tool_use input: %w", err))
			}
			return &Response{Analysis: analysis, RawJSON: raw}, nil
		}
	}
	return nil, apperr.New(apperr.BedrockInvalidResponse, true, fmt.Errorf("no %s tool_use block in response (stop_reason=%s)", toolName, msgResp.StopReason))
}

func classifyBedrockError(err error) error {
	var apiErr smithy.APIError
	if errors.As(err, &apiErr) {
		switch apiErr.ErrorCode() {
		case "ThrottlingException", "ServiceQuotaExceededException":
			return apperr.New(apperr.BedrockThrottling, true, err)
		case "ModelTimeoutException":
			return apperr.New(apperr.BedrockTimeout, true, err)
		case "InternalServerException", "ServiceUnavailableException":
			return apperr.New(apperr.BedrockTimeout, true, err)
		}
	}
	var netErr interface{ Timeout() bool }
	if errors.As(err, &netErr) && netErr.Timeout() {
		return apperr.New(apperr.BedrockTimeout, true, err)
	}
	return apperr.New(apperr.BedrockTimeout, true, err)
}

func buildPrompt(req AnalyzeRequest) string {
	var b []byte
	b = append(b, fmt.Sprintf(
		"You are analyzing a new release of the OSS project %q for its users.\n\n"+
			"Previous version: %s\nCurrent version: %s\n\n"+
			"Explain what this release means for someone operating this software: what changed, "+
			"what breaks, what is deprecated, and whether they need to take migration action before upgrading. "+
			"Be specific about affected components. Use the release notes, pull requests, and changed files below as your source material.\n\n",
		req.Repository, req.PreviousVersion, req.CurrentVersion,
	)...)

	b = append(b, "## Release Notes\n"...)
	b = append(b, req.ReleaseNotes...)
	b = append(b, "\n\n## Pull Requests\n"...)
	for _, pr := range req.PullRequests {
		b = append(b, fmt.Sprintf("- #%d %s\n", pr.Number, pr.Title)...)
	}
	b = append(b, "\n## Changed Files\n"...)
	for _, f := range req.ChangedFiles {
		b = append(b, "- "+f+"\n"...)
	}
	return string(b)
}
