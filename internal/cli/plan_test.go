package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"context"
	"github.com/loaiza000/eject/internal/compatibility"
	"github.com/loaiza000/eject/internal/domain"
	"github.com/loaiza000/eject/internal/planner"
)

func TestPlanCommand(t *testing.T) {
	manifest := filepath.Join("..", "..", "examples", "aws-basic", "manifest.json")
	irJSON, err := execute(t, "inspect", "--manifest", manifest)
	if err != nil {
		t.Fatal(err)
	}
	input := filepath.Join(t.TempDir(), "infrastructure.json")
	if err := os.WriteFile(input, []byte(irJSON), 0600); err != nil {
		t.Fatal(err)
	}
	planJSON, err := execute(t, "plan", "--input", input, "--target", "compose")
	if err != nil {
		t.Fatal(err)
	}
	var plan domain.Plan
	if err := json.Unmarshal([]byte(planJSON), &plan); err != nil || plan.SchemaVersion != "2" || plan.Source == nil || len(plan.Steps) == 0 {
		t.Fatalf("plan=%q, error=%v", planJSON, err)
	}
	reportJSON, err := execute(t, "plan", "--input", input, "--report", "compatibility")
	if err != nil {
		t.Fatal(err)
	}
	var report domain.CompatibilityReport
	if err := json.Unmarshal([]byte(reportJSON), &report); err != nil || report.SchemaVersion != "1" || len(report.Results) == 0 {
		t.Fatalf("report=%q, error=%v", reportJSON, err)
	}
	output := filepath.Join(t.TempDir(), "plan.json")
	if err := os.WriteFile(output, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := execute(t, "plan", "--input", input, "--output", output); err == nil {
		t.Fatal("overwrite without --force accepted")
	}
	data, _ := os.ReadFile(output)
	if string(data) != "keep" {
		t.Fatal("existing output changed")
	}
	if _, err := execute(t, "plan", "--input", input, "--output", output, "--force"); err != nil {
		t.Fatal(err)
	}
}

func TestPlanRejectsUnexpectedSourceProperty(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "tests", "golden", "infrastructure.json"))
	if err != nil {
		t.Fatal(err)
	}
	var infra domain.Infrastructure
	if err := json.Unmarshal(data, &infra); err != nil {
		t.Fatal(err)
	}
	for i := range infra.Resources {
		if infra.Resources[i].Kind == "compute.container" {
			infra.Resources[i].Properties["password"] = "private-value"
		}
	}
	data, err = json.Marshal(infra)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "source.json")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := execute(t, "plan", "--input", path); err == nil {
		t.Fatal("unrecognized sensitive property embedded in plan")
	}
}

func TestPlanInvalidInput(t *testing.T) {
	if _, err := execute(t, "plan"); err == nil {
		t.Fatal("missing input accepted")
	}
	valid := filepath.Join("..", "..", "tests", "golden", "infrastructure.json")
	if _, err := execute(t, "plan", "--input", valid, "--target", "kubernetes"); err == nil {
		t.Fatal("unsupported target accepted")
	}
	if _, err := execute(t, "plan", "--input", valid, "--report", "unknown"); err == nil {
		t.Fatal("unsupported report accepted")
	}
	path := filepath.Join(t.TempDir(), "ir.json")
	for _, tc := range []struct{ name, body, want string }{
		{"malformed", "{", "unexpected EOF"},
		{"unknown-schema", `{"schema_version":"2","resources":[],"dependencies":[],"findings":[]}`, "unsupported infrastructure"},
		{"unknown-field", `{"schema_version":"1","resources":[],"dependencies":[],"findings":[],"extra":true}`, "unknown field"},
		{"deep", `{"x":` + strings.Repeat("[", 65) + "0" + strings.Repeat("]", 65) + "}", "nesting exceeds"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if err := os.WriteFile(path, []byte(tc.body), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := execute(t, "plan", "--input", path)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error=%v, want %q", err, tc.want)
			}
		})
	}
	if err := os.WriteFile(path, []byte(strings.Repeat("x", maxIRSize+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := execute(t, "plan", "--input", path); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversize error=%v", err)
	}
}

func TestPlanRejectsSymlinkInput(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "real.json")
	if err := os.WriteFile(target, []byte(`{"schema_version":"1","resources":[],"dependencies":[],"findings":[]}`), 0600); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link.json")
	if err := os.Symlink(target, link); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	if _, err := execute(t, "plan", "--input", link); err == nil {
		t.Fatal("symlink input accepted")
	}
}

func FuzzPlanInput(f *testing.F) {
	seed, err := os.ReadFile(filepath.Join("..", "..", "tests", "golden", "infrastructure.json"))
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	f.Add([]byte(`{"schema_version":"1","resources":[],"dependencies":[],"findings":[]}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > maxIRSize {
			return
		}
		path := filepath.Join(t.TempDir(), "ir.json")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		infra, err := loadInfrastructure(path)
		if err != nil {
			return
		}
		report, err := compatibility.Evaluate(context.Background(), infra, "compose")
		if err != nil {
			return
		}
		_, _ = planner.Build(context.Background(), infra, report)
	})
}
