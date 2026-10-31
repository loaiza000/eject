//go:build !windows

package securefs

import "os"

// CreateTemp creates a private temporary file before callers write data.
func CreateTemp(dir, pattern string) (*os.File, error) {
	f, err := os.CreateTemp(dir, pattern)
	if err != nil {
		return nil, err
	}
	if err := f.Chmod(0600); err != nil {
		f.Close()
		os.Remove(f.Name())
		return nil, err
	}
	return f, nil
}

// ProtectDirectory is unnecessary on Unix, where Root.Mkdir receives mode 0700.
func ProtectDirectory(_ *os.Root, _ string) error { return nil }

// CheckPrivate is handled by Unix mode checks at the relevant call sites.
func CheckPrivate(_ string) error { return nil }
