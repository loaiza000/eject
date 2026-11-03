package cli

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func execute(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := New("v0.0.0-test")
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(&out)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return out.String(), err
}

func TestHelp(t *testing.T) {
	out, err := execute(t, "--help")
	if err != nil || !strings.Contains(out, "doctor") || !strings.Contains(out, "version") {
		t.Fatalf("help = %q, %v", out, err)
	}
}

func TestVersionJSON(t *testing.T) {
	out, err := execute(t, "version", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]string
	if err := json.Unmarshal([]byte(out), &result); err != nil || result["version"] != "v0.0.0-test" {
		t.Fatalf("version = %q, %v", out, err)
	}
}

func TestDoctorJSON(t *testing.T) {
	out, err := execute(t, "doctor", "--json")
	if err != nil {
		t.Fatal(err)
	}
	var result doctorReport
	if err := json.Unmarshal([]byte(out), &result); err != nil || result.SchemaVersion != "1" || result.GoVersion == "" || result.OS == "" {
		t.Fatalf("doctor = %q, %v", out, err)
	}
}

func TestUnknownCommand(t *testing.T) {
	if _, err := execute(t, "unknown-command"); err == nil {
		t.Fatal("unknown command must fail")
	}
}
