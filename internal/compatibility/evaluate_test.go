package compatibility

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loaiza000/eject/internal/discovery/aws"
	"github.com/loaiza000/eject/internal/domain"
)

func exampleIR(t *testing.T) domain.Infrastructure {
	t.Helper()
	ir, err := aws.Inspect(context.Background(), filepath.Join("..", "..", "examples", "aws-basic", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	return ir
}

func resultByCode(report domain.CompatibilityReport, code string) (domain.CompatibilityResult, bool) {
	for _, result := range report.Results {
		if result.Code == code {
			return result, true
		}
	}
	return domain.CompatibilityResult{}, false
}

func TestExampleRules(t *testing.T) {
	report, err := Evaluate(context.Background(), exampleIR(t), "compose")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		code   string
		status domain.CompatibilityStatus
	}{
		{"ECS_IMAGE_REFERENCE", domain.PartiallySupported},
		{"ECS_PORT_MAPPING", domain.Supported},
		{"ECS_SECRET_MAPPING_REQUIRED", domain.ManualActionRequired},
		{"RDS_POSTGRES_ENGINE", domain.Supported},
		{"RDS_DATA_MIGRATION_REQUIRED", domain.ManualActionRequired},
		{"S3_BACKEND_UNSUPPORTED", domain.Unsupported},
	} {
		r, ok := resultByCode(report, tc.code)
		if !ok || r.Status != tc.status {
			t.Fatalf("rule %s = %+v, found %t", tc.code, r, ok)
		}
		if r.Code == "" || r.Resource == "" || r.Capability == "" || r.Severity == "" || r.Explanation == "" || r.Evidence == "" || r.Recommendation == "" || r.Target != "compose" {
			t.Fatalf("incomplete result: %+v", r)
		}
	}
	data, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "synthetic-secret-reference") || strings.Contains(string(data), "synthetic.example.invalid") {
		t.Fatal("secret or ECS environment value leaked")
	}
}

func TestEdgeRules(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*domain.Infrastructure)
		code   string
		status domain.CompatibilityStatus
	}{
		{"private-ecr", func(ir *domain.Infrastructure) {
			for i := range ir.Resources {
				if ir.Resources[i].Kind == "compute.container" {
					ir.Resources[i].Properties["image"] = "123456789012.dkr.ecr.us-east-1.amazonaws.com/app:1"
				}
			}
		}, "ECS_PRIVATE_ECR_IMAGE", domain.ManualActionRequired},
		{"invalid-port", func(ir *domain.Infrastructure) {
			for i := range ir.Resources {
				if ir.Resources[i].Kind == "compute.container" {
					ir.Resources[i].Properties["port_0"] = "0/tcp"
				}
			}
		}, "ECS_PORT_INVALID", domain.Unsupported},
		{"unsupported-engine", func(ir *domain.Infrastructure) {
			for i := range ir.Resources {
				if ir.Resources[i].Kind == "database" {
					ir.Resources[i].Properties["engine"] = "mysql"
				}
			}
		}, "RDS_ENGINE_UNSUPPORTED", domain.Unsupported},
		{"missing-reference", func(ir *domain.Infrastructure) {
			for i := range ir.Dependencies {
				if ir.Dependencies[i].Relation == "runs" {
					ir.Dependencies[i].To = "missing"
				}
			}
		}, "DEPENDENCY_TARGET_MISSING", domain.ManualActionRequired},
		{"no-task-evidence", func(ir *domain.Infrastructure) { ir.Dependencies = ir.Dependencies[1:] }, "ECS_SERVICE_TASK_UNKNOWN", domain.Unknown},
		{"unmapped-capability", func(ir *domain.Infrastructure) {
			ir.Resources[0].Capabilities = append(ir.Resources[0].Capabilities, "custom_feature")
		}, "CAPABILITY_UNMAPPED", domain.Unknown},
		{"unknown-provider", func(ir *domain.Infrastructure) { ir.Resources[0].Provider = "other" }, "RESOURCE_KIND_UNKNOWN", domain.Unknown},
		{"unknown-version", func(ir *domain.Infrastructure) {
			for i := range ir.Resources {
				if ir.Resources[i].Kind == "database" {
					delete(ir.Resources[i].Properties, "engine_version")
				}
			}
		}, "RDS_VERSION_UNKNOWN", domain.Unknown},
		{"unmapped-postgres-version", func(ir *domain.Infrastructure) {
			for i := range ir.Resources {
				if ir.Resources[i].Kind == "database" {
					ir.Resources[i].Properties["engine_version"] = "18.1"
				}
			}
		}, "RDS_VERSION_UNMAPPED", domain.Unsupported},
		{"kms", func(ir *domain.Infrastructure) {
			for i := range ir.Resources {
				if ir.Resources[i].Kind == "object_storage" {
					ir.Resources[i].Properties["encryption_0"] = "aws:kms"
				}
			}
		}, "S3_KMS_REVIEW", domain.ManualActionRequired},
		{"encryption-unknown", func(ir *domain.Infrastructure) {
			for i := range ir.Resources {
				if ir.Resources[i].Kind == "object_storage" {
					delete(ir.Resources[i].Properties, "encryption_0")
				}
			}
		}, "S3_ENCRYPTION_UNKNOWN", domain.Unknown},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ir := exampleIR(t)
			tc.change(&ir)
			report, err := Evaluate(context.Background(), ir, "compose")
			if err != nil {
				t.Fatal(err)
			}
			r, ok := resultByCode(report, tc.code)
			if !ok || r.Status != tc.status {
				t.Fatalf("rule %s = %+v, found %t", tc.code, r, ok)
			}
		})
	}
}

func TestInvalidInput(t *testing.T) {
	ir := exampleIR(t)
	if _, err := Evaluate(context.Background(), ir, "kubernetes"); err == nil {
		t.Fatal("unsupported target accepted")
	}
	ir.SchemaVersion = "2"
	if _, err := Evaluate(context.Background(), ir, "compose"); err == nil {
		t.Fatal("unsupported schema accepted")
	}
	ir = exampleIR(t)
	ir.Resources = append(ir.Resources, ir.Resources[0])
	if _, err := Evaluate(context.Background(), ir, "compose"); err == nil {
		t.Fatal("duplicate resource accepted")
	}
	ir = exampleIR(t)
	ir.Dependencies[0].From = "missing"
	if _, err := Evaluate(context.Background(), ir, "compose"); err == nil {
		t.Fatal("missing dependency source accepted")
	}
	ir = exampleIR(t)
	ir.Dependencies = append(ir.Dependencies, ir.Dependencies[0])
	if _, err := Evaluate(context.Background(), ir, "compose"); err == nil {
		t.Fatal("duplicate dependency accepted")
	}
	ir = exampleIR(t)
	conflict := ir.Dependencies[0]
	conflict.To = "other-task"
	ir.Dependencies = append(ir.Dependencies, conflict)
	if _, err := Evaluate(context.Background(), ir, "compose"); err == nil {
		t.Fatal("conflicting task dependency accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Evaluate(ctx, exampleIR(t), "compose"); err == nil {
		t.Fatal("cancelled context accepted")
	}
}

func TestDeterministicOrder(t *testing.T) {
	ir := exampleIR(t)
	a, err := Evaluate(context.Background(), ir, "compose")
	if err != nil {
		t.Fatal(err)
	}
	for i, j := 0, len(ir.Resources)-1; i < j; i, j = i+1, j-1 {
		ir.Resources[i], ir.Resources[j] = ir.Resources[j], ir.Resources[i]
	}
	b, err := Evaluate(context.Background(), ir, "compose")
	if err != nil {
		t.Fatal(err)
	}
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	if string(left) != string(right) {
		t.Fatal("evaluation depends on resource order")
	}
}
