package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/loaiza000/eject/internal/verification"
)

func verifyFixture(t *testing.T) string {
	t.Helper()
	output := filepath.Join(t.TempDir(), "project")
	plan := filepath.Join("..", "..", "tests", "golden", "plan.json")
	if _, err := execute(t, "generate", "--plan", plan, "--output", output); err != nil {
		t.Fatal(err)
	}
	return output
}
func readVerify(t *testing.T, project string, args ...string) (verification.Report, error) {
	t.Helper()
	all := append([]string{"verify", "--project", project}, args...)
	out, err := execute(t, all...)
	var report verification.Report
	if jsonErr := json.Unmarshal([]byte(out), &report); jsonErr != nil {
		t.Fatal(jsonErr)
	}
	return report, err
}
func hasCheck(report verification.Report, code string, status verification.Status) bool {
	for _, check := range report.Checks {
		if check.Code == code && check.Status == status {
			return true
		}
	}
	return false
}
func TestVerifyStaticAndAuthorization(t *testing.T) {
	project := verifyFixture(t)
	report, err := readVerify(t, project)
	if err != nil || report.Outcome != verification.Passed || !hasCheck(report, "COMPLETION_MARKER", verification.Passed) || !hasCheck(report, "RUNTIME", verification.NotVerified) {
		t.Fatalf("static report=%+v error=%v", report, err)
	}
	report, err = readVerify(t, project, "--runtime")
	if err != nil || report.Outcome != verification.Skipped || !hasCheck(report, "RUNTIME_AUTHORIZATION", verification.Skipped) {
		t.Fatalf("runtime without authorization=%+v error=%v", report, err)
	}
}

func TestVerifyRejectsUnusedRuntimeOptions(t *testing.T) {
	project := verifyFixture(t)
	for _, args := range [][]string{{"--allow-start"}, {"--allow-pull"}, {"--http-check", "service:8080:/health"}, {"--runtime", "--allow-pull"}} {
		all := append([]string{"verify", "--project", project}, args...)
		if _, err := execute(t, all...); err == nil {
			t.Fatalf("accepted invalid runtime flags: %v", args)
		}
	}
}
func TestVerifyDetectsChangedMissingAndExtraFiles(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(string) error
		code   string
	}{
		{"modified", func(dir string) error {
			return os.WriteFile(filepath.Join(dir, "compose.yaml"), []byte("services: {}\n"), 0600)
		}, "FILE_INTEGRITY"},
		{"missing-marker", func(dir string) error { return os.Remove(filepath.Join(dir, "generation.json")) }, "COMPLETION_MARKER"},
		{"extra", func(dir string) error { return os.WriteFile(filepath.Join(dir, "unexpected.txt"), []byte("x"), 0600) }, "UNEXPECTED_FILE"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			project := verifyFixture(t)
			if err := tc.change(project); err != nil {
				t.Fatal(err)
			}
			report, err := readVerify(t, project)
			if err == nil || report.Outcome != verification.Failed || !hasCheck(report, tc.code, verification.Failed) {
				t.Fatalf("undetected change: %+v %v", report, err)
			}
		})
	}
}
func TestVerifyRejectsPlanTampering(t *testing.T) {
	project := verifyFixture(t)
	path := filepath.Join(project, "plan.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var plan map[string]any
	if err := json.Unmarshal(data, &plan); err != nil {
		t.Fatal(err)
	}
	plan["target"] = "other"
	data, err = json.Marshal(plan)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	report, err := readVerify(t, project)
	if err == nil || !hasCheck(report, "PLAN_COHERENCE", verification.Failed) {
		t.Fatalf("tampering accepted: %+v", report)
	}
}

func TestVerifyDetectsPossibleSecret(t *testing.T) {
	project := verifyFixture(t)
	path := filepath.Join(project, "README.md")
	if err := os.WriteFile(path, []byte("AKIA"+"AAAAAAAAAAAAAAAA"), 0600); err != nil {
		t.Fatal(err)
	}
	report, err := readVerify(t, project)
	if err == nil || !hasCheck(report, "SECRET_SCAN", verification.Failed) {
		t.Fatalf("secret pattern missed: %+v", report)
	}
}
