//go:build !windows

package securefs

import (
	"errors"
	"fmt"
	"os"
)

// CreateTemp creates a private temporary file before callers write data.
func CreateTemp(dir, pattern string) (*os.File, error) {
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return nil, err
	}
	if err := f.Chmod(0600); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return nil, err
	}
	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return nil, err
	}
	if info.Mode().Perm()&0077 != 0 {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return nil, errors.New("filesystem did not enforce private temporary file permissions")
	}
	return f, nil
}

// ProtectDirectory verifies that the newly created directory is private before
// generated files are written. Some mounted filesystems ignore requested modes.
func ProtectDirectory(root *os.Root, _ string) error {
	f, err := root.Open(".")
	if err != nil {
		return err
	}
	info, statErr := f.Stat()
	closeErr := f.Close()
	if statErr != nil {
		return statErr
	}
	if closeErr != nil {
		return closeErr
	}
	if !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("filesystem did not enforce private output directory permissions: mode %04o", info.Mode().Perm())
	}
	return nil
}

// CheckPrivate rejects existing projects whose Unix mode grants group or other
// access; it also rejects symlinks and non-file entries.
func CheckPrivate(path string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() && !info.IsDir() {
		return errors.New("artifact is not a regular file or directory")
	}
	if info.Mode().Perm()&0077 != 0 {
		return errors.New("artifact grants group or other access")
	}
	return nil
}
