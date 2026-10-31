package domain

import "sort"

// CompatibilityStatus describes a capability's mapping to a target.
type CompatibilityStatus string

const (
	Supported            CompatibilityStatus = "SUPPORTED"
	PartiallySupported   CompatibilityStatus = "PARTIALLY_SUPPORTED"
	ManualActionRequired CompatibilityStatus = "MANUAL_ACTION_REQUIRED"
	Unsupported          CompatibilityStatus = "UNSUPPORTED"
	Unknown              CompatibilityStatus = "UNKNOWN"
)

// CompatibilityResult is an evidence-based rule outcome.
type CompatibilityResult struct {
	Code           string              `json:"code"`
	Resource       string              `json:"resource"`
	Capability     string              `json:"capability"`
	Status         CompatibilityStatus `json:"status"`
	Severity       string              `json:"severity"`
	Explanation    string              `json:"explanation"`
	Evidence       string              `json:"evidence"`
	Recommendation string              `json:"recommendation"`
	Target         string              `json:"target"`
}

// CompatibilityReport is a versioned assessment for one target.
type CompatibilityReport struct {
	SchemaVersion string                `json:"schema_version"`
	Target        string                `json:"target"`
	Results       []CompatibilityResult `json:"results"`
}

// Sort gives the report a stable order independent of IR resource order.
func (r *CompatibilityReport) Sort() {
	sort.Slice(r.Results, func(a, b int) bool {
		x, y := r.Results[a], r.Results[b]
		if x.Resource != y.Resource {
			return x.Resource < y.Resource
		}
		if x.Code != y.Code {
			return x.Code < y.Code
		}
		if x.Capability != y.Capability {
			return x.Capability < y.Capability
		}
		return x.Evidence < y.Evidence
	})
}
