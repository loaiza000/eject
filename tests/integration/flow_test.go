package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loaiza000/eject/internal/cli"
	"github.com/loaiza000/eject/internal/domain"
)

func TestOfflineFlowGolden(t *testing.T) {
	dir := t.TempDir()
	manifest := filepath.Join("..", "..", "examples", "aws-basic", "manifest.json")
	irPath := filepath.Join(dir, "infrastructure.json")
	planPath := filepath.Join(dir, "plan.json")
	reportPath := filepath.Join(dir, "compatibility.json")
	for _, args := range [][]string{
		{"inspect", "--manifest", manifest, "--output", irPath},
		{"plan", "--input", irPath, "--target", "compose", "--output", planPath},
		{"plan", "--input", irPath, "--target", "compose", "--report", "compatibility", "--output", reportPath},
	} {
		cmd := cli.New("test")
		cmd.SetArgs(args)
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		if err := cmd.ExecuteContext(context.Background()); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	for _, tc := range []struct{ name, path string }{
		{"infrastructure.json", irPath},
		{"plan.json", planPath},
		{"compatibility.json", reportPath},
	} {
		got, err := os.ReadFile(tc.path)
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile(filepath.Join("..", "golden", tc.name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("%s differs from reviewed golden file", tc.name)
		}
	}
	generated := filepath.Join(dir, "generated")
	cmd := cli.New("test")
	cmd.SetArgs([]string{"generate", "--plan", planPath, "--output", generated})
	cmd.SetOut(&bytes.Buffer{})
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"compose.yaml", ".env.example", "README.md", "limitations.json", "plan.json", "generation.json"} {
		got, err := os.ReadFile(filepath.Join(generated, name))
		if err != nil {
			t.Fatal(err)
		}
		want, err := os.ReadFile(filepath.Join("..", "golden", "generated", name))
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(got, want) {
			t.Fatalf("generated %s differs from reviewed golden file", name)
		}
	}
	cmd = cli.New("test")
	cmd.SetArgs([]string{"verify", "--project", generated})
	var verification bytes.Buffer
	cmd.SetOut(&verification)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(verification.Bytes(), []byte(`"outcome": "PASSED"`)) {
		t.Fatal("static verification did not pass")
	}
}

func TestCompatibleECSContainerFlow(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "golden", "infrastructure.json"))
	if err != nil {
		t.Fatal(err)
	}
	var infra domain.Infrastructure
	if err := json.Unmarshal(data, &infra); err != nil {
		t.Fatal(err)
	}
	for i := range infra.Resources {
		if infra.Resources[i].Kind == "compute.container" {
			delete(infra.Resources[i].Properties, "secret_name_0")
		}
	}
	data, err = json.Marshal(infra)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	input := filepath.Join(dir, "infrastructure.json")
	plan := filepath.Join(dir, "plan.json")
	output := filepath.Join(dir, "output")
	if err := os.WriteFile(input, data, 0600); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"plan", "--input", input, "--output", plan}, {"generate", "--plan", plan, "--output", output}} {
		cmd := cli.New("test")
		cmd.SetArgs(args)
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		if err := cmd.ExecuteContext(context.Background()); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	compose, err := os.ReadFile(filepath.Join(output, "compose.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	for _, text := range []string{"nginx:1.27", "8080/tcp", "postgres:16", "driver: bridge"} {
		if !strings.Contains(string(compose), text) {
			t.Fatalf("missing %q", text)
		}
	}
	if strings.Contains(string(compose), "ports:") {
		t.Fatal("published port without evidence")
	}
}
