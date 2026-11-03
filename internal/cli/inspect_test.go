package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInspectOutputRefusesOverwrite(t *testing.T) {
	manifest := filepath.Join("..", "..", "examples", "aws-basic", "manifest.json")
	path := filepath.Join(t.TempDir(), "ir.json")
	if err := os.WriteFile(path, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := execute(t, "inspect", "--manifest", manifest, "--output", path); err == nil {
		t.Fatal("expected overwrite refusal")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "keep" {
		t.Fatalf("output changed: %q, %v", data, err)
	}
	if _, err := execute(t, "inspect", "--manifest", manifest, "--output", path, "--force"); err != nil {
		t.Fatal(err)
	}
	data, err = os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), `"schema_version": "1"`) {
		t.Fatalf("invalid output: %q, %v", data, err)
	}
	leftovers, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".eject-*.tmp"))
	if err != nil || len(leftovers) != 0 {
		t.Fatalf("temporary files remain: %v, %v", leftovers, err)
	}
}

func TestOutputRejectsSymlinkedPaths(t *testing.T) {
	base := t.TempDir()
	realDir := filepath.Join(base, "real")
	if err := os.Mkdir(realDir, 0700); err != nil {
		t.Fatal(err)
	}
	linkDir := filepath.Join(base, "link")
	if err := os.Symlink(realDir, linkDir); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	if err := writeOutput(filepath.Join(linkDir, "inspection.json"), []byte("sensitive"), false); err == nil {
		t.Fatal("output followed a symlinked parent")
	}
	if _, err := os.Stat(filepath.Join(realDir, "inspection.json")); !os.IsNotExist(err) {
		t.Fatalf("unexpected file in symlink target: %v", err)
	}
	if err := writeOutput(filepath.Join(linkDir, "new", "inspection.json"), []byte("sensitive"), false); err == nil {
		t.Fatal("output created a directory through a symlinked ancestor")
	}
	if _, err := os.Stat(filepath.Join(realDir, "new")); !os.IsNotExist(err) {
		t.Fatalf("unexpected directory in symlink target: %v", err)
	}
	target := filepath.Join(realDir, "existing.json")
	if err := os.WriteFile(target, []byte("keep"), 0600); err != nil {
		t.Fatal(err)
	}
	linkFile := filepath.Join(realDir, "linked.json")
	if err := os.Symlink(target, linkFile); err != nil {
		t.Fatal(err)
	}
	if err := writeOutput(linkFile, []byte("replace"), true); err == nil {
		t.Fatal("--force accepted a symlinked destination")
	}
	got, err := os.ReadFile(target)
	if err != nil || string(got) != "keep" {
		t.Fatalf("symlink target changed: %q, %v", got, err)
	}
}
