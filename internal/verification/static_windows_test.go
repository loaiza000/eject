//go:build windows

package verification

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"golang.org/x/sys/windows"
)

func grantEveryoneRead(t *testing.T, path string) {
	t.Helper()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	sd, err := windows.SecurityDescriptorFromString(fmt.Sprintf(
		"D:P(A;;FA;;;%s)(A;;FA;;;SY)(A;;FA;;;BA)(A;;GR;;;WD)", user.User.Sid.String()))
	if err != nil {
		t.Fatal(err)
	}
	dacl, _, err := sd.DACL()
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.SetNamedSecurityInfo(path, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, dacl, nil); err != nil {
		t.Fatal(err)
	}
}

func TestWindowsStaticRejectsPermissiveGeneratedFile(t *testing.T) {
	dir, bundle := staticFixture(t)
	grantEveryoneRead(t, filepath.Join(dir, "README.md"))
	report := Static(context.Background(), dir, bundle)
	if report.Outcome != Failed || !reportHas(report, "WINDOWS_ACL", Failed) {
		t.Fatalf("permissive generated file accepted: %+v", report)
	}
}

func TestWindowsStaticRejectsPermissiveEnv(t *testing.T) {
	dir, bundle := staticFixture(t)
	path := filepath.Join(dir, ".env")
	if err := os.WriteFile(path, []byte("SYNTHETIC=example\n"), 0600); err != nil {
		t.Fatal(err)
	}
	grantEveryoneRead(t, path)
	report := Static(context.Background(), dir, bundle)
	if report.Outcome != Failed || !reportHas(report, "LOCAL_ENV", Failed) {
		t.Fatalf("permissive .env accepted: %+v", report)
	}
}
