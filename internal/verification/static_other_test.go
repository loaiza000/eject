//go:build !windows

package verification

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestStaticRejectsPermissiveUnixModes(t *testing.T) {
	for _, tc := range []struct {
		name string
		path func(string) string
		mode os.FileMode
	}{
		{"project", func(dir string) string { return dir }, 0755},
		{"generated-file", func(dir string) string { return filepath.Join(dir, "plan.json") }, 0644},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir, bundle := staticFixture(t)
			if err := os.Chmod(tc.path(dir), tc.mode); err != nil {
				t.Fatal(err)
			}
			report := Static(context.Background(), dir, bundle)
			if report.Outcome != Failed || !reportHas(report, "UNIX_PERMISSIONS", Failed) {
				t.Fatalf("permissive project accepted: %+v", report)
			}
		})
	}
}
