//go:build !windows

package cli

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGenerateRejectsPermissiveExistingUnixOutput(t *testing.T) {
	plan := filepath.Join("..", "..", "tests", "golden", "plan.json")
	for _, tc := range []struct {
		name string
		path func(string) string
		mode os.FileMode
	}{
		{"project", func(dir string) string { return dir }, 0755},
		{"generated-file", func(dir string) string { return filepath.Join(dir, "plan.json") }, 0644},
	} {
		t.Run(tc.name, func(t *testing.T) {
			output := filepath.Join(t.TempDir(), "project")
			if _, err := execute(t, "generate", "--plan", plan, "--output", output); err != nil {
				t.Fatal(err)
			}
			if err := os.Chmod(tc.path(output), tc.mode); err != nil {
				t.Fatal(err)
			}
			if _, err := execute(t, "generate", "--plan", plan, "--output", output); err == nil {
				t.Fatal("permissive existing output accepted")
			}
		})
	}
}
