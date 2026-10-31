package cli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loaiza000/eject/internal/generator"
)

func TestGenerateCommandAndIdempotence(t *testing.T) {
	plan := filepath.Join("..", "..", "tests", "golden", "plan.json")
	output := filepath.Join(t.TempDir(), "output")
	for i := 0; i < 2; i++ {
		result, err := execute(t, "generate", "--plan", plan, "--output", output)
		if err != nil || !strings.Contains(result, "PARTIAL") {
			t.Fatalf("generate %d: %q %v", i, result, err)
		}
	}
	if _, err := os.Stat(filepath.Join(output, "compose.yaml")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "README.md"), []byte("user edits"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := execute(t, "generate", "--plan", plan, "--output", output); err == nil {
		t.Fatal("changed output overwritten")
	}
	data, _ := os.ReadFile(filepath.Join(output, "README.md"))
	if string(data) != "user edits" {
		t.Fatal("user file changed")
	}
}
func TestGenerateInputAndOutputSafety(t *testing.T) {
	plan := filepath.Join("..", "..", "tests", "golden", "plan.json")
	dir := t.TempDir()
	for _, input := range []string{"", "[]", "{", strings.Repeat("[", 65), `{"schema_version":"2","extra":true}`} {
		path := filepath.Join(dir, "bad.json")
		if err := os.WriteFile(path, []byte(input), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadPlan(path); err == nil {
			t.Fatalf("invalid plan accepted: %q", input[:min(len(input), 40)])
		}
	}
	output := filepath.Join(dir, "existing")
	if err := os.Mkdir(output, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(output, "other"), []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := execute(t, "generate", "--plan", plan, "--output", output); err == nil {
		t.Fatal("nonempty directory accepted")
	}
	if data, _ := os.ReadFile(filepath.Join(output, "other")); string(data) != "keep" {
		t.Fatal("existing file changed")
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(output, link); err == nil {
		if _, err := execute(t, "generate", "--plan", plan, "--output", link); err == nil {
			t.Fatal("symlink output accepted")
		}
		if _, err := execute(t, "generate", "--plan", plan, "--output", filepath.Join(link, "child")); err == nil {
			t.Fatal("symlink parent accepted")
		}
	}
}
func TestWriteBundleRefusesUnknownName(t *testing.T) {
	path := filepath.Join(t.TempDir(), "output")
	err := writeBundle(context.Background(), path, generator.Bundle{Files: map[string][]byte{"../escape": []byte("bad")}})
	if err == nil {
		t.Fatal("unsafe filename accepted")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("failed bundle left output directory")
	}
}
func TestLoadPlanRejectsSymlink(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "p.json")
	link := filepath.Join(dir, "link.json")
	if err := os.WriteFile(path, []byte("{}"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(path, link); err != nil {
		t.Skip(err)
	}
	if _, err := loadPlan(link); err == nil {
		t.Fatal("symlink plan accepted")
	}
}
func TestWriteBundleDoesNotOverwriteFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "output")
	if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	err := writeBundle(context.Background(), path, generator.Bundle{Files: map[string][]byte{"README.md": []byte("new")}})
	if err == nil {
		t.Fatal("file output accepted")
	}
	data, _ := os.ReadFile(path)
	if !bytes.Equal(data, []byte("keep")) {
		t.Fatal("existing file changed")
	}
}

func TestInterruptedGenerationLeavesNoCompletionMarker(t *testing.T) {
	path := filepath.Join(t.TempDir(), "output")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err := writeBundle(ctx, path, generator.Bundle{Files: map[string][]byte{"README.md": []byte("proposal"), "generation.json": []byte("marker")}})
	if err == nil {
		t.Fatal("canceled generation succeeded")
	}
	if info, err := os.Stat(path); err != nil || !info.IsDir() {
		t.Fatalf("partial directory not retained: %v", err)
	}
	if _, err := os.Stat(filepath.Join(path, "generation.json")); !os.IsNotExist(err) {
		t.Fatal("partial generation has a completion marker")
	}
}

func FuzzGeneratePlan(f *testing.F) {
	seed, err := os.ReadFile(filepath.Join("..", "..", "tests", "golden", "plan.json"))
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	f.Add([]byte(`{"schema_version":"2"}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > maxPlanSize {
			t.Skip()
		}
		path := filepath.Join(t.TempDir(), "plan.json")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		plan, err := loadPlan(path)
		if err == nil {
			_, _ = generator.Build(context.Background(), plan)
		}
	})
}
