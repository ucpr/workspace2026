package release

import "time"

// Repository is the "repositories" DynamoDB table item (spec section 10).
type Repository struct {
	Repository        string    `json:"repository" dynamodbav:"repository"`
	LatestVersion     string    `json:"latestVersion" dynamodbav:"latestVersion"`
	LatestPublishedAt time.Time `json:"latestPublishedAt" dynamodbav:"latestPublishedAt"`
	LastCheckedAt     time.Time `json:"lastCheckedAt" dynamodbav:"lastCheckedAt"`
	CreatedAt         time.Time `json:"createdAt" dynamodbav:"createdAt"`
	UpdatedAt         time.Time `json:"updatedAt" dynamodbav:"updatedAt"`
}

// Risk is the LLM's assessed impact level of a release.
type Risk string

const (
	RiskLow    Risk = "low"
	RiskMedium Risk = "medium"
	RiskHigh   Risk = "high"
)

// BreakingChange, NotableChange and Deprecation mirror the Bedrock
// structured output schema from spec section 8.
type BreakingChange struct {
	Component   string `json:"component" dynamodbav:"component"`
	Title       string `json:"title" dynamodbav:"title"`
	Description string `json:"description" dynamodbav:"description"`
	Migration   string `json:"migration" dynamodbav:"migration"`
}

type NotableChange struct {
	Component   string `json:"component" dynamodbav:"component"`
	Title       string `json:"title" dynamodbav:"title"`
	Description string `json:"description" dynamodbav:"description"`
}

type Deprecation struct {
	Component   string `json:"component" dynamodbav:"component"`
	Description string `json:"description" dynamodbav:"description"`
}

// Analysis is the exact structured output schema requested from
// Bedrock (spec section 8). AffectedComponents is Releascope's own
// addition layered on top of the LLM's response (spec section 9),
// merging LLM-identified components with the regex-based extraction in
// component.go.
type Analysis struct {
	Summary            string           `json:"summary" dynamodbav:"summary"`
	BreakingChanges    []BreakingChange `json:"breakingChanges" dynamodbav:"breakingChanges"`
	NotableChanges     []NotableChange  `json:"notableChanges" dynamodbav:"notableChanges"`
	Deprecations       []Deprecation    `json:"deprecations" dynamodbav:"deprecations"`
	Risk               Risk             `json:"risk" dynamodbav:"risk"`
	MigrationRequired  bool             `json:"migrationRequired" dynamodbav:"migrationRequired"`
	Recommendation     string           `json:"recommendation" dynamodbav:"recommendation"`
	AffectedComponents []string         `json:"affectedComponents" dynamodbav:"affectedComponents"`
}

// Record is the "releases" DynamoDB table item (spec section 10).
type Record struct {
	Repository      string    `json:"repository" dynamodbav:"repository"`
	Version         string    `json:"version" dynamodbav:"version"`
	PreviousVersion string    `json:"previousVersion" dynamodbav:"previousVersion"`
	PublishedAt     time.Time `json:"publishedAt" dynamodbav:"publishedAt"`
	ReleaseURL      string    `json:"releaseUrl" dynamodbav:"releaseUrl"`

	Analysis

	RawDataS3URI        string `json:"rawDataS3Uri" dynamodbav:"rawDataS3Uri"`
	LLMRawResponseS3URI string `json:"llmRawResponseS3Uri" dynamodbav:"llmRawResponseS3Uri"`

	CreatedAt time.Time `json:"createdAt" dynamodbav:"createdAt"`
}
