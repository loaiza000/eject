package verification

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/loaiza000/eject/internal/domain"
	"github.com/loaiza000/eject/internal/generator"
)

func staticFixture(t *testing.T) (string, generator.Bundle) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "tests", "golden", "plan.json"))
	if err != nil {
		t.Fatal(err)
	}
	var plan domain.Plan
	if err := json.Unmarshal(data, &plan); err != nil {
		t.Fatal(err)
	}
	bundle, err := generator.Build(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	for name, content := range bundle.Files {
		if err := os.WriteFile(filepath.Join(dir, name), content, 0600); err != nil {
			t.Fatal(err)
		}
	}
	return dir, bundle
}
func TestStaticIntegrity(t *testing.T) {
	dir, bundle := staticFixture(t)
	report := Static(context.Background(), dir, bundle)
	if report.Outcome != Passed || report.GenerationStatus != "PARTIAL" {
		t.Fatalf("report=%+v", report)
	}
	if err := os.Remove(filepath.Join(dir, "generation.json")); err != nil {
		t.Fatal(err)
	}
	report = Static(context.Background(), dir, bundle)
	if report.Outcome != Failed {
		t.Fatal("missing completion marker accepted")
	}
}
func TestStaticModifiedAndUnsafeEntries(t *testing.T) {
	dir, bundle := staticFixture(t)
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if report := Static(context.Background(), dir, bundle); report.Outcome != Failed {
		t.Fatal("modified file accepted")
	}
	if err := os.Symlink(filepath.Join(dir, "plan.json"), filepath.Join(dir, "extra")); err == nil {
		if report := Static(context.Background(), dir, bundle); report.Outcome != Failed {
			t.Fatal("symlink accepted")
		}
	}
}

func TestStaticRejectsAlteredCompletionMarker(t *testing.T) {
	dir, bundle := staticFixture(t)
	if err := os.WriteFile(filepath.Join(dir, "generation.json"), []byte(`{"schema_version":"1","files":{}}`), 0600); err != nil {
		t.Fatal(err)
	}
	report := Static(context.Background(), dir, bundle)
	if report.Outcome != Failed {
		t.Fatal("altered marker accepted")
	}
	for _, check := range report.Checks {
		if check.Code == "COMPLETION_MARKER" && check.Status == Failed {
			return
		}
	}
	t.Fatal("altered marker incorrectly reported complete")
}
