//go:build !windows

package cli

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/loaiza000/eject/internal/verification"
)

func TestVerifyFailsOnPermissiveUnixOutput(t *testing.T) {
	project := verifyFixture(t)
	if err := os.Chmod(filepath.Join(project, "plan.json"), 0644); err != nil {
		t.Fatal(err)
	}
	report, err := readVerify(t, project)
	var exitErr *ExitError
	if !errors.As(err, &exitErr) || exitErr.Code != 4 || report.Outcome != verification.Failed || !hasCheck(report, "UNIX_PERMISSIONS", verification.Failed) {
		t.Fatalf("permissive project was not rejected: report=%+v error=%v", report, err)
	}
}
