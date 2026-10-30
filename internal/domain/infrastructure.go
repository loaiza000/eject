// Package domain defines provider-neutral infrastructure records.
package domain

import "sort"

// Infrastructure is the versioned output of offline inspection.
type Infrastructure struct {
	SchemaVersion string       `json:"schema_version"`
	Resources     []Resource   `json:"resources"`
	Dependencies  []Dependency `json:"dependencies"`
	Findings      []Finding    `json:"findings"`
}

// Resource records what the supplied evidence actually establishes.
type Resource struct {
	ID           string            `json:"id"`
	Kind         string            `json:"kind"`
	Provider     string            `json:"provider"`
	Name         string            `json:"name"`
	Properties   map[string]string `json:"properties,omitempty"`
	Capabilities []string          `json:"capabilities,omitempty"`
	Sources      []string          `json:"sources"`
	Missing      []string          `json:"missing,omitempty"`
	Status       string            `json:"status"`
}

// Dependency links resources with an explicit evidence classification.
type Dependency struct {
	From     string `json:"from"`
	To       string `json:"to"`
	Relation string `json:"relation"`
	Status   string `json:"status"`
	Evidence string `json:"evidence"`
}

// Finding captures missing or unsupported evidence without pretending certainty.
type Finding struct {
	Code     string `json:"code"`
	Resource string `json:"resource,omitempty"`
	Message  string `json:"message"`
}

// Sort makes JSON output independent of manifest entry order.
func (i *Infrastructure) Sort() {
	sort.Slice(i.Resources, func(a, b int) bool { return i.Resources[a].ID < i.Resources[b].ID })
	for n := range i.Resources {
		sort.Strings(i.Resources[n].Sources)
	}
	sort.Slice(i.Dependencies, func(a, b int) bool {
		x, y := i.Dependencies[a], i.Dependencies[b]
		if x.From != y.From {
			return x.From < y.From
		}
		if x.To != y.To {
			return x.To < y.To
		}
		return x.Relation < y.Relation
	})
	sort.Slice(i.Findings, func(a, b int) bool {
		x, y := i.Findings[a], i.Findings[b]
		if x.Code != y.Code {
			return x.Code < y.Code
		}
		if x.Resource != y.Resource {
			return x.Resource < y.Resource
		}
		return x.Message < y.Message
	})
}
