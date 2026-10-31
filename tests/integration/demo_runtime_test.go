package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/loaiza000/eject/internal/cli"
	"github.com/loaiza000/eject/internal/verification"
)

// Run only with EJECT_RUNTIME_TEST=1 after preparing trusted local images.
func TestAuthorizedDemoRuntime(t *testing.T) {
	if os.Getenv("EJECT_RUNTIME_TEST") != "1" {
		t.Skip("set EJECT_RUNTIME_TEST=1 to authorize local Docker startup")
	}
	dir := t.TempDir()
	manifest := filepath.Join("..", "..", "examples", "demo", "manifest.json")
	infra := filepath.Join(dir, "infrastructure.json")
	plan := filepath.Join(dir, "plan.json")
	project := filepath.Join(dir, "project")
	for _, args := range [][]string{{"inspect", "--manifest", manifest, "--output", infra}, {"plan", "--input", infra, "--output", plan}, {"generate", "--plan", plan, "--output", project}} {
		cmd := cli.New("test")
		cmd.SetArgs(args)
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetErr(&bytes.Buffer{})
		if err := cmd.ExecuteContext(context.Background()); err != nil {
			t.Fatalf("%v: %v", args, err)
		}
	}
	env := "DATABASE_HOST=eject-demo-db-2a98991fc0368e39\nPOSTGRES_PASSWORD=synthetic-local-only\n"
	if err := os.WriteFile(filepath.Join(project, ".env"), []byte(env), 0600); err != nil {
		t.Fatal(err)
	}
	cmd := cli.New("test")
	cmd.SetArgs([]string{"verify", "--project", project, "--runtime", "--allow-start", "--http-check", "backend-0c64dac5bad49d23:8080:/health"})
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&bytes.Buffer{})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("runtime verification: %v\n%s", err, out.String())
	}
	var report verification.Report
	if err := json.Unmarshal(out.Bytes(), &report); err != nil {
		t.Fatal(err)
	}
	if report.Outcome != verification.Passed {
		t.Fatalf("runtime outcome=%s", report.Outcome)
	}
	for _, code := range []string{"STARTUP", "HEALTHCHECK", "POSTGRES_QUERY", "HTTP", "CLEANUP"} {
		found := false
		for _, check := range report.Checks {
			if check.Code == code && check.Status == verification.Passed {
				found = true
			}
		}
		if !found {
			t.Fatalf("missing passing runtime check %s", code)
		}
	}
}
