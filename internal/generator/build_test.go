package generator

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/loaiza000/eject/internal/compatibility"
	"github.com/loaiza000/eject/internal/domain"
	"github.com/loaiza000/eject/internal/planner"
)

func source(t *testing.T) domain.Infrastructure {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "tests", "golden", "infrastructure.json"))
	if err != nil {
		t.Fatal(err)
	}
	var infra domain.Infrastructure
	if err := json.Unmarshal(data, &infra); err != nil {
		t.Fatal(err)
	}
	return infra
}
func makePlan(t *testing.T, infra domain.Infrastructure) domain.Plan {
	t.Helper()
	report, err := compatibility.Evaluate(context.Background(), infra, "compose")
	if err != nil {
		t.Fatal(err)
	}
	plan, err := planner.Build(context.Background(), infra, report)
	if err != nil {
		t.Fatal(err)
	}
	return plan
}
func TestBuildExamplePartialAndDeterministic(t *testing.T) {
	plan := makePlan(t, source(t))
	a, err := Build(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Build(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if a.Status != "PARTIAL" || !bytes.Equal(a.Files["compose.yaml"], b.Files["compose.yaml"]) {
		t.Fatalf("status=%q, nondeterministic output", a.Status)
	}
	if bytes.Contains(a.Files["compose.yaml"], []byte("nginx")) {
		t.Fatal("blocked ECS container emitted")
	}
	if !bytes.Contains(a.Files["compose.yaml"], []byte("postgres:16")) || !bytes.Contains(a.Files["compose.yaml"], []byte("pg_isready")) {
		t.Fatal("PostgreSQL service or health check absent")
	}
	if bytes.Contains(a.Files["limitations.json"], []byte("no files are generated")) {
		t.Fatal("stale limitation")
	}
	if string(a.Files[".env.example"]) != "POSTGRES_PASSWORD=\n" {
		t.Fatalf("env example=%q", a.Files[".env.example"])
	}
}
func TestBuildContainerPortAndEnvironment(t *testing.T) {
	infra := source(t)
	for i := range infra.Resources {
		if infra.Resources[i].Kind == "compute.container" {
			delete(infra.Resources[i].Properties, "secret_name_0")
		}
	}
	plan := makePlan(t, infra)
	b, err := Build(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	compose := string(b.Files["compose.yaml"])
	for _, term := range []string{"nginx:1.27", "8080/tcp", "DATABASE_HOST", "driver: bridge"} {
		if !strings.Contains(compose, term) {
			t.Fatalf("missing %q", term)
		}
	}
	if strings.Contains(compose, "ports:") {
		t.Fatal("host port published without authorization")
	}
	if strings.Contains(string(b.Files[".env.example"]), "synthetic-secret-reference") {
		t.Fatal("secret leaked")
	}
}
func TestBuildBlockedStorageOnly(t *testing.T) {
	infra := source(t)
	infra.Resources = infra.Resources[:1]
	infra.Dependencies = nil
	plan := makePlan(t, infra)
	b, err := Build(context.Background(), plan)
	if err != nil {
		t.Fatal(err)
	}
	if b.Status != "BLOCKED" || b.Files["compose.yaml"] != nil || b.Files["limitations.json"] == nil {
		t.Fatalf("unsafe blocked output: %+v", b)
	}
}

func TestPostgresOutsideValidatedVersionExcluded(t *testing.T) {
	infra := source(t)
	for i := range infra.Resources {
		if infra.Resources[i].Kind == "database" {
			infra.Resources[i].Properties["engine_version"] = "18.1"
		}
	}
	b, err := Build(context.Background(), makePlan(t, infra))
	if err != nil {
		t.Fatal(err)
	}
	if b.Files["compose.yaml"] != nil || b.Status != "BLOCKED" {
		t.Fatalf("unvalidated version emitted: %+v", b)
	}
}
func TestRejectTamperedAndLegacyPlan(t *testing.T) {
	plan := makePlan(t, source(t))
	plan.Compatibility.Results[0].Status = domain.Supported
	if _, err := Build(context.Background(), plan); err == nil {
		t.Fatal("tampered status accepted")
	}
	plan = makePlan(t, source(t))
	plan.Steps[0].TargetResource = "compose:service:../../escape"
	if _, err := Build(context.Background(), plan); err == nil {
		t.Fatal("tampered target accepted")
	}
	plan = makePlan(t, source(t))
	plan.SchemaVersion = "1"
	plan.Source = nil
	if _, err := Build(context.Background(), plan); err == nil {
		t.Fatal("legacy plan accepted without source")
	}
}
func TestRejectUnsafeSource(t *testing.T) {
	for _, tc := range []struct{ key, value string }{{"image", "alpine:${TOKEN}"}, {"env_name_0", "BAD-NAME"}, {"password", "secret"}, {"port_0", "99999/tcp"}} {
		infra := source(t)
		for i := range infra.Resources {
			if infra.Resources[i].Kind == "compute.container" {
				infra.Resources[i].Properties[tc.key] = tc.value
			}
		}
		plan := makePlan(t, infra)
		if _, err := Build(context.Background(), plan); err == nil {
			t.Fatalf("unsafe %s accepted", tc.key)
		}
	}
}
