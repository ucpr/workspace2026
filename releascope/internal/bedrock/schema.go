package bedrock

// toolInputSchema is the JSON Schema handed to Bedrock's Anthropic
// Messages API as a forced tool call, guaranteeing the model's response
// is well-formed structured output rather than free text we would need
// to scrape a JSON blob out of. Fields mirror spec section 8 exactly
// (affectedComponents is intentionally excluded here: per spec section
// 9 that field is populated separately by internal/release's regex
// based extraction and merged in afterwards).
const toolName = "emit_release_analysis"

var toolInputSchema = map[string]any{
	"type": "object",
	"properties": map[string]any{
		"summary": map[string]any{"type": "string"},
		"breakingChanges": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"component":   map[string]any{"type": "string"},
					"title":       map[string]any{"type": "string"},
					"description": map[string]any{"type": "string"},
					"migration":   map[string]any{"type": "string"},
				},
				"required": []string{"component", "title", "description", "migration"},
			},
		},
		"notableChanges": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"component":   map[string]any{"type": "string"},
					"title":       map[string]any{"type": "string"},
					"description": map[string]any{"type": "string"},
				},
				"required": []string{"component", "title", "description"},
			},
		},
		"deprecations": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"component":   map[string]any{"type": "string"},
					"description": map[string]any{"type": "string"},
				},
				"required": []string{"component", "description"},
			},
		},
		"risk":              map[string]any{"type": "string", "enum": []string{"low", "medium", "high"}},
		"migrationRequired": map[string]any{"type": "boolean"},
		"recommendation":    map[string]any{"type": "string"},
	},
	"required": []string{"summary", "breakingChanges", "notableChanges", "deprecations", "risk", "migrationRequired", "recommendation"},
}
