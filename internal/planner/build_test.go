package planner

import (
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loaiza000/eject/internal/compatibility"
	"github.com/loaiza000/eject/internal/discovery/aws"
	"github.com/loaiza000/eject/internal/domain"
)

func example(t *testing.T) (domain.Infrastructure, domain.CompatibilityReport) {
	t.Helper()
	ir, err := aws.Inspect(context.Background(), filepath.Join("..", "..", "examples", "aws-basic", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	report, err := compatibility.Evaluate(context.Background(), ir, "compose")
	if err != nil {
		t.Fatal(err)
	}
	return ir, report
}

func TestBuildExample(t *testing.T) {
	ir, report := example(t)
	plan, err := Build(context.Background(), ir, report)
	if err != nil {
		t.Fatal(err)
	}
	if plan.SchemaVersion != "2" || plan.Target != "compose" || plan.Source == nil || len(plan.Steps) != 3 || len(plan.Blockers) == 0 || len(plan.ManualActions) == 0 {
		t.Fatalf("unexpected plan: %+v", plan)
	}
	if got := strings.Join(plan.RequiredVariables, ","); got != "DATABASE_HOST,DATABASE_PASSWORD" {
		t.Fatalf("variables = %q", got)
	}
	steps := map[string]domain.PlanStep{}
	for _, step := range plan.Steps {
		steps[step.ID] = step
		if step.ExpectedStatus == "" || len(step.Preconditions) == 0 || len(step.SourceResources) == 0 {
			t.Fatalf("incomplete step: %+v", step)
		}
	}
	for _, step := range plan.Steps {
		for _, dep := range step.DependsOn {
			if _, ok := steps[dep]; !ok {
				t.Fatalf("missing step dependency %q", dep)
			}
		}
		if strings.Contains(step.TargetResource, "bucket") {
			t.Fatalf("unimplemented S3 target planned: %+v", step)
		}
	}
	data, err := json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), "synthetic-secret-reference") || strings.Contains(string(data), "synthetic.example.invalid") {
		t.Fatal("secret leaked into plan")
	}
}

func TestBuildDeterministic(t *testing.T) {
	ir, report := example(t)
	a, err := Build(context.Background(), ir, report)
	if err != nil {
		t.Fatal(err)
	}
	for i, j := 0, len(ir.Resources)-1; i < j; i, j = i+1, j-1 {
		ir.Resources[i], ir.Resources[j] = ir.Resources[j], ir.Resources[i]
	}
	b, err := Build(context.Background(), ir, report)
	if err != nil {
		t.Fatal(err)
	}
	left, _ := json.Marshal(a)
	right, _ := json.Marshal(b)
	if string(left) != string(right) {
		t.Fatal("plan depends on resource order")
	}
}

func TestBuildRejectsReportMismatch(t *testing.T) {
	ir, report := example(t)
	report.Target = "other"
	if _, err := Build(context.Background(), ir, report); err == nil {
		t.Fatal("mismatched report accepted")
	}
}

func TestSafeName(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"Demo_DB", "demo-db"}, {"***", "resource"}, {"A/B\\C", "a-b-c"},
	} {
		if got := safeName(tc.in); got != tc.want {
			t.Fatalf("safeName(%q)=%q, want %q", tc.in, got, tc.want)
		}
	}
}
