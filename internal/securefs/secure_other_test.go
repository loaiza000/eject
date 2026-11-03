//go:build !windows

package securefs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestUnixPrivateModes(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		t.Fatal(err)
	}
	defer root.Close()
	if err := ProtectDirectory(root, dir); err != nil {
		t.Fatal(err)
	}
	file, err := CreateTemp(dir, "private-*.json")
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}
	if err := CheckPrivate(file.Name()); err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(file.Name(), 0644); err != nil {
		t.Fatal(err)
	}
	if err := CheckPrivate(file.Name()); err == nil {
		t.Fatal("world-readable generated file accepted")
	}
	if err := os.Chmod(dir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := CheckPrivate(dir); err == nil {
		t.Fatal("world-readable output directory accepted")
	}
	if err := ProtectDirectory(root, dir); err == nil {
		t.Fatal("permissive directory accepted before generation")
	}
	link := filepath.Join(dir, "linked.json")
	if err := os.Symlink(file.Name(), link); err != nil {
		t.Fatal(err)
	}
	if err := CheckPrivate(link); err == nil {
		t.Fatal("symlink accepted as a private artifact")
	}
}
