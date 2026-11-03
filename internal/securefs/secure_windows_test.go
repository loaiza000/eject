//go:build windows

package securefs

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"unsafe"

	"golang.org/x/sys/windows"
)

func acl(t *testing.T, path string, protected bool) string {
	t.Helper()
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		t.Fatal(err)
	}
	control, _, err := sd.Control()
	if err != nil || protected && control&windows.SE_DACL_PROTECTED == 0 {
		t.Fatalf("DACL is not protected for %s: %v", path, err)
	}
	return sd.String()
}

func TestPrivateArtifactsIgnorePermissiveParent(t *testing.T) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		t.Fatal(err)
	}
	parent := filepath.Join(t.TempDir(), "permissive")
	sddl := fmt.Sprintf("D:P(A;OICI;FA;;;%s)(A;OICI;FA;;;SY)(A;OICI;FA;;;BA)(A;OICI;GR;;;WD)", user.User.Sid.String())
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		t.Fatal(err)
	}
	sa := &windows.SecurityAttributes{Length: uint32(unsafe.Sizeof(windows.SecurityAttributes{})), SecurityDescriptor: sd}
	name, err := windows.UTF16PtrFromString(parent)
	if err != nil {
		t.Fatal(err)
	}
	if err := windows.CreateDirectory(name, sa); err != nil {
		t.Fatal(err)
	}
	before := acl(t, parent, true)
	if !strings.Contains(before, ";;;WD)") {
		t.Fatalf("test parent lacks permissive Everyone ACE: %s", before)
	}
	if err := CheckPrivate(parent); err == nil {
		t.Fatal("permissive parent accepted as private")
	}
	tmp, err := CreateTemp(parent, ".eject-*.tmp")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tmp.WriteString("sensitive metadata"); err != nil {
		t.Fatal(err)
	}
	if err := tmp.Close(); err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmp.Name())
	if got := acl(t, tmp.Name(), true); strings.Contains(got, ";;;WD)") {
		t.Fatalf("temporary file inherited Everyone: %s", got)
	}
	if err := CheckPrivate(tmp.Name()); err != nil {
		t.Fatal(err)
	}
	child := filepath.Join(parent, "project")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(child)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := ProtectDirectory(root, child); err != nil {
		t.Fatal(err)
	}
	if got := acl(t, child, true); strings.Contains(got, ";;;WD)") {
		t.Fatalf("project directory inherited Everyone: %s", got)
	}
	if err := CheckPrivate(child); err != nil {
		t.Fatal(err)
	}
	f, err := root.OpenFile("plan.json", os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("sensitive metadata"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if got := acl(t, filepath.Join(child, "plan.json"), false); strings.Contains(got, ";;;WD)") {
		t.Fatalf("generated file inherited Everyone: %s", got)
	}
	if err := CheckPrivate(filepath.Join(child, "plan.json")); err != nil {
		t.Fatal(err)
	}
	if after := acl(t, parent, true); after != before {
		t.Fatalf("parent ACL changed: before=%s after=%s", before, after)
	}
}
