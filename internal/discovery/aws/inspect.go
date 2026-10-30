// Package aws reads selected AWS CLI JSON responses from a versioned manifest.
package aws

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/loaiza000/eject/internal/domain"
)

const (
	maxFileSize  = 1 << 20
	maxJSONDepth = 64
)

type manifest struct {
	SchemaVersion string  `json:"schema_version"`
	Exports       []entry `json:"exports"`
}

type entry struct {
	Operation string `json:"operation"`
	File      string `json:"file"`
	Resource  string `json:"resource"`
	Region    string `json:"region"`
}

// Inspect parses AWS CLI responses without contacting AWS or executing commands.
func Inspect(ctx context.Context, manifestPath string) (domain.Infrastructure, error) {
	result := domain.Infrastructure{SchemaVersion: "1", Resources: []domain.Resource{}, Dependencies: []domain.Dependency{}, Findings: []domain.Finding{}}
	absManifest, err := filepath.Abs(manifestPath)
	if err != nil {
		return result, fmt.Errorf("manifest path: %w", err)
	}
	root := filepath.Dir(absManifest)
	rootHandle, err := os.OpenRoot(root)
	if err != nil {
		return result, fmt.Errorf("manifest directory: %w", err)
	}
	defer rootHandle.Close()
	manifestFile, err := secureOpen(rootHandle, root, filepath.Base(absManifest))
	if err != nil {
		return result, fmt.Errorf("manifest path: %w", err)
	}
	defer manifestFile.Close()
	var m manifest
	if err := readJSON(manifestFile, &m, true); err != nil {
		return result, fmt.Errorf("manifest: %w", err)
	}
	if m.SchemaVersion != "1" {
		return result, fmt.Errorf("unsupported manifest schema_version %q", m.SchemaVersion)
	}
	if len(m.Exports) == 0 || len(m.Exports) > 1000 {
		return result, errors.New("manifest exports must contain 1 to 1000 entries")
	}
	seen := map[string]bool{}
	for n, e := range m.Exports {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		if e.Operation == "" || e.File == "" || e.Resource == "" || e.Region == "" {
			return result, fmt.Errorf("export %d: operation, file, resource and region are required", n)
		}
		key := e.Operation + "\x00" + e.Resource + "\x00" + e.Region
		if seen[key] {
			return result, fmt.Errorf("export %d: duplicate operation/resource/region", n)
		}
		seen[key] = true
		file, err := secureOpen(rootHandle, root, e.File)
		if err != nil {
			return result, fmt.Errorf("export %d file: %w", n, err)
		}
		parseErr := parse(file, e, &result)
		closeErr := file.Close()
		if parseErr != nil {
			return result, fmt.Errorf("export %d (%s): %w", n, e.Operation, parseErr)
		}
		if closeErr != nil {
			return result, fmt.Errorf("export %d file close: %w", n, closeErr)
		}
	}
	ids := map[string]bool{}
	for _, r := range result.Resources {
		if ids[r.ID] {
			return result, fmt.Errorf("duplicate resource ID %q", r.ID)
		}
		ids[r.ID] = true
	}
	for _, d := range result.Dependencies {
		if !ids[d.To] {
			result.Findings = append(result.Findings, domain.Finding{Code: "MISSING_REFERENCE", Resource: d.From, Message: "Referenced resource was not supplied: " + d.To})
		}
	}
	result.Sort()
	return result, nil
}

func secureOpen(rootHandle *os.Root, root, relative string) (*os.File, error) {
	if relative == "" || filepath.IsAbs(relative) {
		return nil, errors.New("path must be relative")
	}
	joined := filepath.Join(root, relative)
	rel, err := filepath.Rel(root, joined)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return nil, errors.New("path escapes manifest directory")
	}
	current := ""
	for _, part := range strings.Split(rel, string(filepath.Separator)) {
		current = filepath.Join(current, part)
		info, err := rootHandle.Lstat(current)
		if err != nil {
			return nil, err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("symlink in input path")
		}
	}
	file, err := rootHandle.Open(rel)
	if err != nil {
		return nil, err
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, err
	}
	if !info.Mode().IsRegular() {
		file.Close()
		return nil, errors.New("path is not a regular file")
	}
	return file, nil
}

func readJSON(file *os.File, target any, strict bool) error {
	info, err := file.Stat()
	if err != nil {
		return err
	}
	if info.Size() > maxFileSize {
		return fmt.Errorf("file exceeds %d bytes", maxFileSize)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxFileSize+1))
	if err != nil {
		return err
	}
	if len(data) > maxFileSize {
		return fmt.Errorf("file exceeds %d bytes", maxFileSize)
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return errors.New("JSON root must be an object")
	}
	if err := checkJSONDepth(data); err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	if strict {
		dec.DisallowUnknownFields()
	}
	if err := dec.Decode(target); err != nil {
		return err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return errors.New("trailing JSON content")
	}
	return nil
}

// checkJSONDepth bounds nested input before encoding/json allocates nested values.
// The decoder still performs full syntax validation.
func checkJSONDepth(data []byte) error {
	depth := 0
	inString, escaped := false, false
	for _, b := range data {
		if inString {
			switch {
			case escaped:
				escaped = false
			case b == '\\':
				escaped = true
			case b == '"':
				inString = false
			}
			continue
		}
		switch b {
		case '"':
			inString = true
		case '{', '[':
			depth++
			if depth > maxJSONDepth {
				return fmt.Errorf("JSON nesting exceeds %d levels", maxJSONDepth)
			}
		case '}', ']':
			depth--
		}
	}
	return nil
}
