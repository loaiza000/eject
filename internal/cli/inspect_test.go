package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectOutputRefusesOverwrite(t *testing.T) {
	manifest := filepath.Join("..", "..", "examples", "aws-basic", "manifest.json")
	path := filepath.Join(t.TempDir(), "ir.json")
	if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := execute(t, "inspect", "--manifest", manifest, "--output", path); err == nil {
		t.Fatal("expected overwrite refusal")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "keep" {
		t.Fatalf("output changed: %q, %v", data, err)
	}
	if _, err := execute(t, "inspect", "--manifest", manifest, "--output", path, "--force"); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), `"schema_version": "1"`) {
		t.Fatalf("invalid output: %q, %v", data, err)
	}
	leftovers, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".eject-*.tmp"))
	if err != nil || len(leftovers) != 0 {
		t.Fatalf("temporary files remain: %v, %v", leftovers, err)
	}
}
