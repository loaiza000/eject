package domain

// Plan is a versioned proposal. It does not describe executed changes.
type Plan struct {
	SchemaVersion       string              `json:"schema_version"`
	Target              string              `json:"target"`
	SourceSchemaVersion string              `json:"source_schema_version"`
	Source              *Infrastructure     `json:"source,omitempty"`
	SourceResources     []string            `json:"source_resources"`
	Compatibility       CompatibilityReport `json:"compatibility"`
	Steps               []PlanStep          `json:"steps"`
	Blockers            []PlanIssue         `json:"blockers"`
	ManualActions       []PlanIssue         `json:"manual_actions"`
	RequiredVariables   []string            `json:"required_variables"`
	Limitations         []string            `json:"limitations"`
	RecommendedChecks   []string            `json:"recommended_checks"`
}

// PlanStep describes a proposed configuration transformation, not an action EJECT performs.
type PlanStep struct {
	ID              string   `json:"id"`
	SourceResources []string `json:"source_resources"`
	TargetResource  string   `json:"target_resource"`
	Description     string   `json:"description"`
	DependsOn       []string `json:"depends_on"`
	Preconditions   []string `json:"preconditions"`
	ExpectedStatus  string   `json:"expected_status"`
}

// PlanIssue makes a blocker or manual action traceable to a rule and resource.
type PlanIssue struct {
	Code        string `json:"code"`
	Resource    string `json:"resource"`
	Description string `json:"description"`
}
