// Package planner turns compatibility evidence into an unexecuted target proposal.
package planner

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"slices"
	"sort"
	"strings"

	"github.com/loaiza000/eject/internal/domain"
)

// Build proposes Compose configuration steps; it never performs a migration.
func Build(ctx context.Context, infra domain.Infrastructure, report domain.CompatibilityReport) (domain.Plan, error) {
	source := domain.Infrastructure{SchemaVersion: infra.SchemaVersion, Resources: slices.Clone(infra.Resources), Dependencies: slices.Clone(infra.Dependencies), Findings: slices.Clone(infra.Findings)}
	for i := range source.Resources {
		source.Resources[i].Sources = slices.Clone(source.Resources[i].Sources)
	}
	plan := domain.Plan{
		SchemaVersion: "2", Target: "compose", SourceSchemaVersion: infra.SchemaVersion,
		Source:          &source,
		SourceResources: []string{}, Compatibility: report, Steps: []domain.PlanStep{},
		Blockers: []domain.PlanIssue{}, ManualActions: []domain.PlanIssue{},
		RequiredVariables: []string{}, Limitations: []string{}, RecommendedChecks: []string{},
	}
	if infra.SchemaVersion != "1" || report.SchemaVersion != "1" || report.Target != "compose" {
		return plan, errors.New("incompatible infrastructure or compatibility report version/target")
	}
	variables := map[string]bool{}
	plannedContainers := map[string]bool{}
	for _, r := range infra.Resources {
		if err := ctx.Err(); err != nil {
			return plan, err
		}
		plan.SourceResources = append(plan.SourceResources, r.ID)
		if r.Provider != "aws" {
			continue
		}
		switch r.Kind {
		case "compute.container":
			if r.Properties["image"] == "" {
				continue
			}
			plannedContainers[r.ID] = true
			for key, value := range r.Properties {
				if (strings.HasPrefix(key, "env_name_") || strings.HasPrefix(key, "secret_name_")) && value != "" {
					variables[value] = true
				}
			}
			plan.Steps = append(plan.Steps, domain.PlanStep{
				ID: stableID("container", r.ID), SourceResources: []string{r.ID},
				TargetResource: "compose:service:" + safeName(r.Name) + "-" + shortHash(r.ID),
				Description:    "Propose a Compose service using the recorded container image and non-public container ports.",
				DependsOn:      []string{},
				Preconditions:  []string{"Verify image access and architecture", "Supply environment and secret values securely", "Review runtime, health and network settings"},
				ExpectedStatus: "CONFIGURATION_PROPOSED",
			})
		case "database":
			if r.Properties["engine"] != "postgres" {
				continue
			}
			plan.Steps = append(plan.Steps, domain.PlanStep{
				ID: stableID("postgres", r.ID), SourceResources: []string{r.ID},
				TargetResource: "compose:service:" + safeName(r.Name) + "-" + shortHash(r.ID),
				Description:    "Propose an empty PostgreSQL Compose service; no RDS data or configuration is transferred.",
				DependsOn:      []string{},
				Preconditions:  []string{"Verify PostgreSQL version, extensions and configuration", "Design and test a separate data migration"},
				ExpectedStatus: "CONFIGURATION_PROPOSED",
			})
		}
	}
	taskContainers := map[string][]string{}
	serviceTasks := map[string]string{}
	for _, d := range infra.Dependencies {
		if d.Status != "CONFIRMED" {
			continue
		}
		switch d.Relation {
		case "contains":
			taskContainers[d.From] = append(taskContainers[d.From], d.To)
		case "runs":
			serviceTasks[d.From] = d.To
		}
	}
	for _, r := range infra.Resources {
		if r.Provider != "aws" || r.Kind != "compute.service" {
			continue
		}
		taskID := serviceTasks[r.ID]
		containers := taskContainers[taskID]
		if taskID == "" || len(containers) == 0 {
			continue
		}
		depends := make([]string, 0, len(containers))
		for _, containerID := range containers {
			if plannedContainers[containerID] {
				depends = append(depends, stableID("container", containerID))
			}
		}
		if len(depends) != len(containers) {
			continue
		}
		sort.Strings(depends)
		plan.Steps = append(plan.Steps, domain.PlanStep{
			ID: stableID("service-review", r.ID), SourceResources: []string{r.ID, taskID},
			TargetResource: "compose:project", Description: "Review the ECS service and task relationship against the proposed Compose services.",
			DependsOn:      depends,
			Preconditions:  []string{"Confirm all task containers have valid image mappings", "Review scaling, networking and load balancing"},
			ExpectedStatus: "REVIEW_REQUIRED",
		})
	}
	for _, result := range report.Results {
		issue := domain.PlanIssue{Code: result.Code, Resource: result.Resource, Description: result.Recommendation}
		if result.Severity == "blocker" {
			plan.Blockers = append(plan.Blockers, issue)
		}
		if result.Status == domain.ManualActionRequired {
			plan.ManualActions = append(plan.ManualActions, issue)
		}
	}
	for variable := range variables {
		plan.RequiredVariables = append(plan.RequiredVariables, variable)
	}
	plan.Limitations = append(plan.Limitations,
		"This plan is a proposal only; no files are generated and no infrastructure is changed.",
		"A valid Compose configuration would not prove application or data equivalence.",
	)
	if hasKind(infra.Resources, "database") {
		plan.Limitations = append(plan.Limitations, "Database data migration, rollback and cutover are not implemented.")
	}
	if hasKind(infra.Resources, "object_storage") {
		plan.Limitations = append(plan.Limitations, "S3 objects, version history, IAM and policies are not migrated or mapped.")
	}
	plan.RecommendedChecks = append(plan.RecommendedChecks,
		"Review every compatibility result and resolve blockers before attempting migration.",
		"Validate image access, runtime behavior, network exposure and secrets in a separate environment.",
	)
	sort.Strings(plan.SourceResources)
	plan.Source.Sort()
	sort.Strings(plan.RequiredVariables)
	sort.Strings(plan.Limitations)
	sort.Strings(plan.RecommendedChecks)
	sort.Slice(plan.Steps, func(a, b int) bool { return plan.Steps[a].ID < plan.Steps[b].ID })
	sort.Slice(plan.Blockers, func(a, b int) bool { return issueLess(plan.Blockers[a], plan.Blockers[b]) })
	sort.Slice(plan.ManualActions, func(a, b int) bool { return issueLess(plan.ManualActions[a], plan.ManualActions[b]) })
	return plan, nil
}

func hasKind(resources []domain.Resource, kind string) bool {
	for _, r := range resources {
		if r.Kind == kind {
			return true
		}
	}
	return false
}

func issueLess(a, b domain.PlanIssue) bool {
	if a.Resource != b.Resource {
		return a.Resource < b.Resource
	}
	if a.Code != b.Code {
		return a.Code < b.Code
	}
	return a.Description < b.Description
}

func shortHash(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:8])
}

func stableID(kind, resource string) string { return "step-" + kind + "-" + shortHash(resource) }

func safeName(value string) string {
	var b strings.Builder
	lastDash := false
	for _, r := range strings.ToLower(value) {
		if b.Len() >= 30 {
			break
		}
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
			lastDash = false
		} else if b.Len() > 0 && !lastDash {
			b.WriteByte('-')
			lastDash = true
		}
	}
	name := strings.Trim(b.String(), "-")
	if name == "" {
		return "resource"
	}
	return name
}
