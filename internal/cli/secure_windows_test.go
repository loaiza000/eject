//go:build windows

package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"

	"github.com/loaiza000/eject/internal/generator"
	"golang.org/x/sys/windows"
)

func testPermissiveDirectory(t *testing.T) string {
	t.Helper()
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "permissive")
	sd, err := windows.SecurityDescriptorFromString(fmt.Sprintf(
		"D:P(A;OICI;FA;;;%s)(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;GR;;;WD)", user.User.Sid.String()))
	if err != nil {
		t.Fatal(err)
	}
	sa := &windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.CreateDirectory(name, sa); err != nil {
		t.Fatal(err)
	}
	return path
}

func windowsACL(t *testing.T, path string) string {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	return sd.String()
}

func assertPrivateACL(t *testing.T, path string) {
	t.Helper()
	acl := windowsACL(t, path)
	for _, trustee := range []string{";;;WD)", ";;;AU)", "S-1-5-21-3178792518-1144263595-2431071690-1003"} {
		if strings.Contains(acl, trustee) {
			t.Fatalf("unexpected reader on %s: %s", path, acl)
		}
	}
}

func TestWindowsOutputAndForceDoNotInheritReaders(t *testing.T) {
	parent := testPermissiveDirectory(t)
	before := windowsACL(t, parent)
	path := filepath.Join(parent, "inspection.json")
	if err := writeOutput(path, []byte(`{"sensitive":"metadata"}`), false); err != nil {
		t.Fatal(err)
	}
	assertPrivateACL(t, path)
	if err := writeOutput(path, []byte(`{"sensitive":"replaced"}`), true); err != nil {
		t.Fatal(err)
	}
	assertPrivateACL(t, path)
	if got := windowsACL(t, parent); got != before {
		t.Fatalf("parent ACL changed: %s -> %s", before, got)
	}
}

func TestWindowsGenerationFilesDoNotInheritReaders(t *testing.T) {
	parent := testPermissiveDirectory(t)
	before := windowsACL(t, parent)
	path := filepath.Join(parent, "project")
	bundle := generator.Bundle{Files: map[string][]byte{
		"plan.json":       []byte(`{"sensitive":"metadata"}`),
		"generation.json": []byte(`{"complete":true}`),
	}}
	if err := writeBundle(context.Background(), path, bundle); err != nil {
		t.Fatal(err)
	}
	assertPrivateACL(t, path)
	for name := range bundle.Files {
		assertPrivateACL(t, filepath.Join(path, name))
	}
	if got := windowsACL(t, parent); got != before {
		t.Fatalf("parent ACL changed: %s -> %s", before, got)
	}
	if _, err := os.Stat(filepath.Join(path, "plan.json")); err != nil {
		t.Fatal(err)
	}
}

func TestWindowsExistingPermissiveGenerationRejected(t *testing.T) {
	parent := testPermissiveDirectory(t)
	path := filepath.Join(parent, "legacy")
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	bundle := generator.Bundle{Files: map[string][]byte{"plan.json": []byte("same")}}
	if err := os.WriteFile(filepath.Join(path, "plan.json"), []byte("same"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := writeBundle(context.Background(), path, bundle); err == nil {
		t.Fatal("permissive existing generation accepted")
	}
	if data, err := os.ReadFile(filepath.Join(path, "plan.json")); err != nil || string(data) != "same" {
		t.Fatalf("legacy project was changed: %q, %v", data, err)
	}
}
