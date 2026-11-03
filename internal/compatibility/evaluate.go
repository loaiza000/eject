// Package compatibility evaluates explicit, evidence-based target rules.
package compatibility

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/loaiza000/eject/internal/domain"
)

type resourceRule struct {
	kind     string
	evaluate func(domain.Resource) []domain.CompatibilityResult
}

var postgres16Version = regexp.MustCompile(`^16\.[0-9]+(?:\.[0-9]+)?$`)

// Evaluate assesses an IR against the planned Compose target without probing a runtime.
func Evaluate(ctx context.Context, infra domain.Infrastructure, target string) (domain.CompatibilityReport, error) {
	report := domain.CompatibilityReport{SchemaVersion: "1", Target: target, Results: []domain.CompatibilityResult{}}
	if target != "compose" {
		return report, fmt.Errorf("unsupported target %q", target)
	}
	if infra.SchemaVersion != "1" {
		return report, fmt.Errorf("unsupported infrastructure schema_version %q", infra.SchemaVersion)
	}
	rules := []resourceRule{
		{"compute.container", containerRules},
		{"compute.service", serviceRules},
		{"compute.task", taskRules},
		{"database", databaseRules},
		{"object_storage", storageRules},
	}
	resources := make(map[string]domain.Resource, len(infra.Resources))
	confirmedRelations := map[string]map[string]bool{}
	seenDependencies := map[string]bool{}
	confirmedRuns := map[string]string{}
	for _, d := range infra.Dependencies {
		key := d.From + "\x00" + d.Relation + "\x00" + d.To + "\x00" + d.Status
		if seenDependencies[key] {
			return report, fmt.Errorf("duplicate dependency %q -> %q", d.From, d.To)
		}
		seenDependencies[key] = true
		if d.Status == "CONFIRMED" {
			if d.Relation == "runs" {
				if prior := confirmedRuns[d.From]; prior != "" && prior != d.To {
					return report, fmt.Errorf("conflicting confirmed task dependencies for %q", d.From)
				}
				confirmedRuns[d.From] = d.To
			}
			if confirmedRelations[d.From] == nil {
				confirmedRelations[d.From] = map[string]bool{}
			}
			confirmedRelations[d.From][d.Relation] = true
		}
	}
	for _, r := range infra.Resources {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		if r.ID == "" || r.Kind == "" {
			return report, errors.New("resource ID and kind are required")
		}
		if _, exists := resources[r.ID]; exists {
			return report, fmt.Errorf("duplicate resource ID %q", r.ID)
		}
		resources[r.ID] = r
		matched := false
		for _, rule := range rules {
			if r.Provider == "aws" && rule.kind == r.Kind {
				report.Results = append(report.Results, rule.evaluate(r)...)
				matched = true
				break
			}
		}
		if !matched {
			report.Results = append(report.Results, finding("RESOURCE_KIND_UNKNOWN", r.ID, "resource_kind", domain.Unknown, "warning", "No Compose mapping rule exists for this provider and resource kind.", r.Provider+":"+r.Kind, "Review this resource manually."))
		}
		if r.Provider == "aws" && r.Kind == "compute.service" && !confirmedRelations[r.ID]["runs"] {
			report.Results = append(report.Results, finding("ECS_SERVICE_TASK_UNKNOWN", r.ID, "task_dependency", domain.Unknown, "blocker", "No confirmed task definition relationship is available for this ECS service.", sourceEvidence(r), "Supply the DescribeServices task definition and matching task snapshot."))
		}
		if r.Provider == "aws" && r.Kind == "compute.task" && !confirmedRelations[r.ID]["contains"] {
			report.Results = append(report.Results, finding("ECS_TASK_CONTAINERS_UNKNOWN", r.ID, "container_dependency", domain.Unknown, "blocker", "No confirmed container relationship is available for this ECS task.", sourceEvidence(r), "Supply the matching task definition snapshot."))
		}
		for _, cap := range r.Capabilities {
			if !knownCapability(r.Kind, cap) {
				report.Results = append(report.Results, finding("CAPABILITY_UNMAPPED", r.ID, cap, domain.Unknown, "warning", "No rule covers this required capability.", sourceEvidence(r), "Assess this capability before migration."))
			}
		}
	}
	for _, d := range infra.Dependencies {
		if err := ctx.Err(); err != nil {
			return report, err
		}
		if _, ok := resources[d.From]; !ok {
			return report, fmt.Errorf("dependency source %q is absent", d.From)
		}
		if d.To == "" {
			return report, errors.New("dependency target is empty")
		}
		if _, ok := resources[d.To]; !ok {
			report.Results = append(report.Results, finding("DEPENDENCY_TARGET_MISSING", d.From, "dependency", domain.ManualActionRequired, "blocker", "A referenced resource is absent from the inventory.", d.Evidence, "Provide the missing snapshot or verify the reference: "+d.To))
		}
		if d.Status != "CONFIRMED" {
			report.Results = append(report.Results, finding("DEPENDENCY_UNVERIFIED", d.From, "dependency", domain.Unknown, "warning", "This dependency is not confirmed by the snapshots.", d.Evidence, "Verify the relationship before migration."))
		}
	}
	for _, f := range infra.Findings {
		if f.Code == "MISSING_REFERENCE" || f.Code == "UNSUPPORTED_ENGINE" || f.Code == "KMS_MAPPING_REQUIRED" || f.Code == "VERSIONING_UNKNOWN" {
			continue
		}
		report.Results = append(report.Results, finding("SOURCE_FINDING", f.Resource, "discovery", domain.Unknown, "warning", "An unclassified discovery finding needs review.", f.Code+": "+f.Message, "Review the source finding."))
	}
	report.Sort()
	return report, nil
}

func finding(code, resource, capability string, status domain.CompatibilityStatus, severity, explanation, evidence, recommendation string) domain.CompatibilityResult {
	return domain.CompatibilityResult{Code: code, Resource: resource, Capability: capability, Status: status, Severity: severity, Explanation: explanation, Evidence: evidence, Recommendation: recommendation, Target: "compose"}
}

func sourceEvidence(r domain.Resource) string {
	sources := append([]string(nil), r.Sources...)
	sort.Strings(sources)
	return strings.Join(sources, ", ")
}

func knownCapability(kind, cap string) bool {
	switch kind {
	case "compute.container":
		return cap == "container_image"
	case "database":
		return cap == "persistent_database"
	default:
		return false
	}
}

func serviceRules(r domain.Resource) []domain.CompatibilityResult {
	return []domain.CompatibilityResult{
		finding("ECS_SERVICE_MAPPING", r.ID, "service", domain.PartiallySupported, "warning", "A Compose service can represent a container workload, but ECS scheduling and scaling semantics are not captured.", sourceEvidence(r), "Review deployment count, scaling, load balancing and networking manually."),
	}
}

func taskRules(r domain.Resource) []domain.CompatibilityResult {
	results := []domain.CompatibilityResult{
		finding("ECS_TASK_MAPPING", r.ID, "task_definition", domain.PartiallySupported, "warning", "Container definitions can inform Compose services; ECS task semantics are not equivalent.", sourceEvidence(r), "Review CPU, memory, roles, logs and task lifecycle outside this plan."),
	}
	mode := r.Properties["network_mode"]
	if mode == "" {
		results = append(results, finding("ECS_NETWORK_UNKNOWN", r.ID, "network", domain.Unknown, "warning", "The task network mode is missing.", sourceEvidence(r), "Obtain and review the ECS network mode."))
	} else {
		results = append(results, finding("ECS_NETWORK_REVIEW", r.ID, "network", domain.ManualActionRequired, "warning", "ECS network mode does not map automatically to Compose networking.", "network_mode="+mode, "Design Compose networks and access controls manually."))
	}
	return results
}

func containerRules(r domain.Resource) []domain.CompatibilityResult {
	image := r.Properties["image"]
	results := make([]domain.CompatibilityResult, 0, 4)
	if image == "" {
		results = append(results, finding("ECS_IMAGE_MISSING", r.ID, "container_image", domain.Unsupported, "blocker", "No image reference is available.", sourceEvidence(r), "Supply an image reference before planning a Compose service."))
	} else if strings.Contains(image, ".dkr.ecr.") {
		results = append(results, finding("ECS_PRIVATE_ECR_IMAGE", r.ID, "container_image", domain.ManualActionRequired, "blocker", "A private ECR image requires access or an image copy procedure.", "image="+image, "Arrange registry authentication or copy the image to an accessible registry."))
	} else {
		results = append(results, finding("ECS_IMAGE_REFERENCE", r.ID, "container_image", domain.PartiallySupported, "warning", "The image reference can be proposed for Compose, but pull access and runtime behavior are unverified.", "image="+image, "Verify image availability, architecture and runtime configuration."))
	}
	keys := make([]string, 0, len(r.Properties))
	for key := range r.Properties {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		value := r.Properties[key]
		switch {
		case strings.HasPrefix(key, "port_"):
			portText, protocol, hasSeparator := strings.Cut(value, "/")
			port, err := strconv.Atoi(portText)
			if err != nil || port < 1 || port > 65535 || !hasSeparator || (protocol != "tcp" && protocol != "udp") {
				results = append(results, finding("ECS_PORT_INVALID", r.ID, "container_port", domain.Unsupported, "blocker", "The container port or protocol cannot be mapped safely.", key+"="+value, "Correct the port mapping in the source inventory."))
			} else {
				results = append(results, finding("ECS_PORT_MAPPING", r.ID, "container_port", domain.Supported, "info", "The container port and protocol have a Compose representation; public exposure is not implied.", key+"="+value, "Decide separately whether this port should be published."))
			}
		case strings.HasPrefix(key, "env_name_"):
			results = append(results, finding("ECS_ENV_VALUE_REQUIRED", r.ID, "environment", domain.ManualActionRequired, "warning", "The snapshot records the variable name but its value was intentionally discarded.", "variable="+value, "Provide the value through a secure local mechanism."))
		case strings.HasPrefix(key, "secret_name_"):
			results = append(results, finding("ECS_SECRET_MAPPING_REQUIRED", r.ID, "secret", domain.ManualActionRequired, "blocker", "The ECS secret reference was not retained and has no automatic Compose mapping.", "variable="+value, "Provide a secret through an approved local mechanism; do not put it in the plan."))
		}
	}
	return results
}

func databaseRules(r domain.Resource) []domain.CompatibilityResult {
	engine := r.Properties["engine"]
	results := make([]domain.CompatibilityResult, 0, 3)
	switch engine {
	case "postgres":
		results = append(results, finding("RDS_POSTGRES_ENGINE", r.ID, "database_engine", domain.Supported, "info", "The PostgreSQL engine family has a Compose service representation; version and extensions remain unverified.", "engine=postgres", "Check exact version, extensions and configuration."))
	case "":
		results = append(results, finding("RDS_ENGINE_UNKNOWN", r.ID, "database_engine", domain.Unknown, "blocker", "The database engine is absent.", sourceEvidence(r), "Obtain the engine type before planning."))
	default:
		results = append(results, finding("RDS_ENGINE_UNSUPPORTED", r.ID, "database_engine", domain.Unsupported, "blocker", "Only PostgreSQL is in scope for the Compose plan.", "engine="+engine, "Plan this database separately."))
	}
	if engine == "postgres" {
		results = append(results, finding("RDS_DATA_MIGRATION_REQUIRED", r.ID, "database_data", domain.ManualActionRequired, "blocker", "RDS metadata contains no database contents; EJECT cannot transfer them.", sourceEvidence(r), "Design, test and execute a separate data migration and cutover."))
		if r.Properties["engine_version"] == "" {
			results = append(results, finding("RDS_VERSION_UNKNOWN", r.ID, "database_version", domain.Unknown, "warning", "The PostgreSQL engine version is absent.", sourceEvidence(r), "Obtain the exact version and extension inventory."))
		} else if !postgres16Version.MatchString(r.Properties["engine_version"]) {
			results = append(results, finding("RDS_VERSION_UNMAPPED", r.ID, "database_version", domain.Unsupported, "blocker", "The local PostgreSQL volume layout is validated only for major version 16.", "engine_version="+r.Properties["engine_version"], "Review this PostgreSQL version and its image layout before generating a local service."))
		}
	}
	return results
}

func storageRules(r domain.Resource) []domain.CompatibilityResult {
	results := []domain.CompatibilityResult{
		finding("S3_BACKEND_UNSUPPORTED", r.ID, "object_storage", domain.Unsupported, "blocker", "No S3-compatible Compose backend mapping is implemented.", sourceEvidence(r), "Select and validate a storage target separately."),
		finding("S3_DATA_AND_IAM_REVIEW", r.ID, "objects_and_permissions", domain.ManualActionRequired, "blocker", "Snapshots do not include objects, IAM permissions or bucket policies.", sourceEvidence(r), "Inventory objects and review access controls outside EJECT."),
	}
	versioning := r.Properties["versioning"]
	if versioning == "" {
		results = append(results, finding("S3_VERSIONING_UNKNOWN", r.ID, "versioning", domain.Unknown, "warning", "Versioning status is unknown from the supplied snapshots.", sourceEvidence(r), "Confirm versioning behavior and retained object versions."))
	} else {
		results = append(results, finding("S3_VERSIONING_REVIEW", r.ID, "versioning", domain.ManualActionRequired, "warning", "S3 versioning behavior is not mapped to a target.", "versioning="+versioning, "Decide how to preserve version history and delete behavior."))
	}
	encryptionFound := false
	for key, value := range r.Properties {
		if !strings.HasPrefix(key, "encryption_") {
			continue
		}
		encryptionFound = true
		code := "S3_ENCRYPTION_REVIEW"
		if strings.HasPrefix(value, "aws:kms") {
			code = "S3_KMS_REVIEW"
		}
		results = append(results, finding(code, r.ID, "encryption", domain.ManualActionRequired, "warning", "S3 encryption settings have no automatic target equivalence.", key+"="+value, "Review key management and encryption at the chosen destination."))
	}
	if !encryptionFound {
		results = append(results, finding("S3_ENCRYPTION_UNKNOWN", r.ID, "encryption", domain.Unknown, "warning", "No bucket encryption metadata was supplied.", sourceEvidence(r), "Confirm encryption requirements and key management."))
	}
	return results
}
