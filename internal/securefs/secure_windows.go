//go:build windows

package securefs

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
)

// CreateTemp gives each new file a protected DACL before any content is written.
// A subsequent rename or hard link retains this descriptor on Windows.
func CreateTemp(dir, pattern string) (*os.File, error) {
	if dir == "" {
		dir = os.TempDir()
	}
	prefix, suffix, ok := strings.Cut(pattern, "*")
	if !ok || strings.Contains(suffix, "*") {
		return nil, errors.New("temporary file pattern must contain one wildcard")
	}
	sa, err := privateAttributes(false)
	if err != nil {
		return nil, err
	}
	for range 10 {
		var random [16]byte
		if _, err := rand.Read(random[:]); err != nil {
			return nil, err
		}
		path := filepath.Join(dir, prefix+hex.EncodeToString(random[:])+suffix)
		name, err := windows.UTF16PtrFromString(path)
		if err != nil {
			return nil, err
		}
		handle, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, sa, windows.CREATE_NEW, windows.FILE_ATTRIBUTE_NORMAL, 0)
		if errors.Is(err, windows.ERROR_FILE_EXISTS) || errors.Is(err, windows.ERROR_ALREADY_EXISTS) {
			continue
		}
		if err != nil {
			return nil, err
		}
		return os.NewFile(uintptr(handle), path), nil
	}
	return nil, errors.New("cannot allocate a unique temporary file")
}

// ProtectDirectory is called on a newly created, still-empty output directory.
// It changes only that directory, never its parent or an existing project.
func ProtectDirectory(root *os.Root, path string) error {
	anchored, err := root.Open(".")
	if err != nil {
		return err
	}
	defer anchored.Close()
	anchoredInfo, err := anchored.Stat()
	if err != nil {
		return err
	}
	name, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return err
	}
	handle, err := windows.CreateFile(name, windows.WRITE_DAC|windows.READ_CONTROL|windows.FILE_READ_ATTRIBUTES,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE|windows.FILE_SHARE_DELETE, nil, windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS|windows.FILE_FLAG_OPEN_REPARSE_POINT, 0)
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(handle), path)
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.IsDir() || !os.SameFile(anchoredInfo, info) {
		return errors.New("output directory changed before ACL protection")
	}
	sa, err := privateAttributes(true)
	if err != nil {
		return err
	}
	dacl, _, err := sa.SecurityDescriptor.DACL()
	if err != nil || dacl == nil {
		return fmt.Errorf("private directory DACL unavailable: %w", err)
	}
	return windows.SetSecurityInfo(handle, windows.SE_FILE_OBJECT,
		windows.DACL_SECURITY_INFORMATION|windows.PROTECTED_DACL_SECURITY_INFORMATION,
		nil, nil, dacl, nil)
}

func privateAttributes(directory bool) (*windows.SecurityAttributes, error) {
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return nil, err
	}
	flags := ""
	if directory {
		flags = "OICI"
	}
	sddl := fmt.Sprintf("D:P(A;%s;FA;;;%s)(A;%s;FA;;;SY)(A;%s;FA;;;BA)", flags, user.User.Sid.String(), flags, flags)
	sd, err := windows.SecurityDescriptorFromString(sddl)
	if err != nil {
		return nil, err
	}
	return &windows.SecurityAttributes{
		Length:             uint32(unsafe.Sizeof(windows.SecurityAttributes{})),
		SecurityDescriptor: sd,
	}, nil
}

// CheckPrivate fails closed if a file or directory grants access to a trustee
// other than its current user, LocalSystem, or Administrators. It cannot prove
// secrecy against privileged processes or future ACL changes.
func CheckPrivate(path string) error {
	sd, err := windows.GetNamedSecurityInfo(path, windows.SE_FILE_OBJECT, windows.DACL_SECURITY_INFORMATION)
	if err != nil {
		return err
	}
	if sd == nil {
		return errors.New("missing security descriptor")
	}
	dacl, _, err := sd.DACL()
	if err != nil || dacl == nil {
		return errors.New("missing or null DACL")
	}
	user, err := windows.GetCurrentProcessToken().GetTokenUser()
	if err != nil {
		return err
	}
	allowed := map[string]bool{
		user.User.Sid.String(): true,
		"S-1-5-18":             true, // LocalSystem
		"S-1-5-32-544":         true, // Administrators
	}
	for i := uint32(0); i < uint32(dacl.AceCount); i++ {
		var ace *windows.ACCESS_ALLOWED_ACE
		if err := windows.GetAce(dacl, i, &ace); err != nil {
			return err
		}
		if ace.Header.AceType != windows.ACCESS_ALLOWED_ACE_TYPE ||
			!allowed[(*windows.SID)(unsafe.Pointer(&ace.SidStart)).String()] {
			return errors.New("ACL grants or describes access outside the private trustee set")
		}
	}
	return nil
}
