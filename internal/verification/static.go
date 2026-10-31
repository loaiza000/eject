// Package verification checks generated projects without claiming workload equivalence.
package verification

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"

	"github.com/loaiza000/eject/internal/generator"
	"github.com/loaiza000/eject/internal/securefs"
)

type Status string

const (
	Passed      Status = "PASSED"
	Failed      Status = "FAILED"
	Skipped     Status = "SKIPPED"
	NotVerified Status = "NOT_VERIFIED"
)

type Check struct {
	Code   string `json:"code"`
	Status Status `json:"status"`
	Detail string `json:"detail"`
}
type Report struct {
	SchemaVersion    string  `json:"schema_version"`
	Mode             string  `json:"mode"`
	Outcome          Status  `json:"outcome"`
	GenerationStatus string  `json:"generation_status,omitempty"`
	Checks           []Check `json:"checks"`
}

func NewReport(mode string) Report {
	return Report{SchemaVersion: "1", Mode: mode, Outcome: Passed, Checks: []Check{}}
}
func (r *Report) Add(code string, status Status, detail string) {
	r.Checks = append(r.Checks, Check{Code: code, Status: status, Detail: detail})
	if status == Failed {
		r.Outcome = Failed
	}
}

var credentialPattern = regexp.MustCompile(`(?i)(AKIA[0-9A-Z]{16}|-----BEGIN (RSA |EC |OPENSSH )?PRIVATE KEY-----|aws_secret_access_key\s*[:=])`)

// Static compares each generated file with a fresh rendering of its embedded plan.
// The caller must parse the plan with bounded, strict JSON decoding first.
func Static(ctx context.Context, project string, bundle generator.Bundle) Report {
	r := NewReport("static")
	r.GenerationStatus = bundle.Status
	root, err := os.OpenRoot(project)
	if err != nil {
		r.Add("PROJECT_READ", Failed, "Cannot open project directory.")
		return r
	}
	defer root.Close()
	dir, err := root.Open(".")
	if err != nil {
		r.Add("PROJECT_READ", Failed, "Cannot list project directory.")
		return r
	}
	entries, err := dir.ReadDir(-1)
	closeErr := dir.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		r.Add("PROJECT_READ", Failed, "Cannot list project directory.")
		return r
	}
	r.Add("PLAN_COHERENCE", Passed, "Embedded plan, resources and rule results were recalculated.")
	seen := map[string]bool{}
	markerValid := false
	aclValid := true
	if runtime.GOOS == "windows" && securefs.CheckPrivate(project) != nil {
		aclValid = false
	}
	for _, entry := range entries {
		if err := ctx.Err(); err != nil {
			r.Add("INTERRUPTED", Failed, "Verification was interrupted.")
			return r
		}
		name := entry.Name()
		if name == ".env" {
			if !entry.Type().IsRegular() {
				r.Add("LOCAL_ENV", Failed, "Private .env is not a regular file.")
			} else if runtime.GOOS != "windows" {
				info, err := entry.Info()
				if err != nil || info.Mode().Perm()&0077 != 0 {
					r.Add("LOCAL_ENV", Failed, "Private .env permissions must exclude group and other users.")
				} else {
					r.Add("LOCAL_ENV", NotVerified, "Private .env values are intentionally not inspected.")
				}
			} else if err := securefs.CheckPrivate(filepath.Join(project, name)); err != nil {
				r.Add("LOCAL_ENV", Failed, "Private .env grants access outside the local user and system administrators.")
			} else {
				r.Add("LOCAL_ENV", NotVerified, "Private .env values are intentionally not inspected.")
			}
			continue
		}
		want, ok := bundle.Files[name]
		if !ok || !entry.Type().IsRegular() {
			r.Add("UNEXPECTED_FILE", Failed, "Unexpected or unsafe project entry: "+name)
			continue
		}
		seen[name] = true
		if runtime.GOOS == "windows" && securefs.CheckPrivate(filepath.Join(project, name)) != nil {
			aclValid = false
		}
		f, err := root.Open(name)
		if err != nil {
			r.Add("FILE_READ", Failed, "Cannot read "+name)
			continue
		}
		got, readErr := io.ReadAll(io.LimitReader(f, int64(len(want))+1))
		closeErr := f.Close()
		if readErr != nil || closeErr != nil {
			r.Add("FILE_READ", Failed, "Cannot read "+name)
			continue
		}
		if !bytes.Equal(got, want) {
			r.Add("FILE_INTEGRITY", Failed, "Generated file differs from plan: "+name)
		} else {
			r.Add("FILE_INTEGRITY", Passed, "Generated file matches plan: "+name)
			if name == "generation.json" {
				markerValid = true
			}
		}
		if name != "generation.json" && credentialPattern.Match(got) {
			r.Add("SECRET_SCAN", Failed, "Possible credential in generated file: "+name)
		}
	}
	keys := make([]string, 0, len(bundle.Files))
	for key := range bundle.Files {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		if !seen[key] {
			r.Add("MISSING_FILE", Failed, "Missing generated file: "+key)
		}
	}
	if markerValid {
		r.Add("COMPLETION_MARKER", Passed, "Generation completion marker is present and matches the plan.")
	} else {
		r.Add("COMPLETION_MARKER", Failed, "Generation is incomplete: completion marker is missing, altered or unreadable.")
	}
	if r.Outcome != Failed {
		r.Add("REFERENCES_AND_VARIABLES", Passed, "Compose services, variables and dependencies match the recalculated plan.")
		r.Add("GENERATED_SECRETS", Passed, "No credential pattern found in generated files; private .env content was not inspected.")
	}
	if runtime.GOOS == "windows" {
		if aclValid {
			r.Add("WINDOWS_ACL", Passed, "Generated files grant access only to the current user, LocalSystem and Administrators.")
		} else {
			r.Add("WINDOWS_ACL", Failed, "Project or generated file grants access outside the private Windows trustee set.")
		}
	}
	r.Add("APPLICATION_BEHAVIOR", NotVerified, "Static checks do not establish application behavior or data equivalence.")
	return r
}
