// Package generator renders a reviewed plan into a local Compose proposal.
package generator

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/loaiza000/eject/internal/compatibility"
	"github.com/loaiza000/eject/internal/domain"
	"github.com/loaiza000/eject/internal/planner"
	"gopkg.in/yaml.v3"
)

var (
	versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+(?:\.[0-9]+)?$`)
)

// Bundle contains only local files. Building it has no external side effects.
type Bundle struct {
	Status string
	Files  map[string][]byte
}

type completion struct {
	SchemaVersion string            `json:"schema_version"`
	Status        string            `json:"status"`
	Files         map[string]string `json:"files"`
}

type limitations struct {
	SchemaVersion string             `json:"schema_version"`
	Status        string             `json:"status"`
	Included      []string           `json:"included"`
	Excluded      []string           `json:"excluded"`
	Blockers      []domain.PlanIssue `json:"blockers"`
	ManualActions []domain.PlanIssue `json:"manual_actions"`
	Limitations   []string           `json:"limitations"`
}

type compose struct {
	Name     string             `yaml:"name"`
	Services map[string]service `yaml:"services"`
	Networks map[string]network `yaml:"networks,omitempty"`
	Volumes  map[string]volume  `yaml:"volumes,omitempty"`
}
type service struct {
	Image       string            `yaml:"image"`
	Environment map[string]string `yaml:"environment,omitempty"`
	Expose      []string          `yaml:"expose,omitempty"`
	Networks    []string          `yaml:"networks,omitempty"`
	Volumes     []string          `yaml:"volumes,omitempty"`
	Healthcheck *healthcheck      `yaml:"healthcheck,omitempty"`
}
type healthcheck struct {
	Test     []string `yaml:"test"`
	Interval string   `yaml:"interval"`
	Timeout  string   `yaml:"timeout"`
	Retries  int      `yaml:"retries"`
}
type network struct {
	Driver string `yaml:"driver"`
}
type volume struct{}

// Build verifies the entire plan against its source before rendering any files.
func Build(ctx context.Context, plan domain.Plan) (Bundle, error) {
	var out Bundle
	if plan.SchemaVersion != "2" || plan.Target != "compose" || plan.SourceSchemaVersion != "1" || plan.Source == nil {
		return out, errors.New("generate requires a version 2 Compose plan with embedded source; regenerate older plans with eject plan")
	}
	if err := compatibility.ValidateSource(*plan.Source); err != nil {
		return out, err
	}
	report, err := compatibility.Evaluate(ctx, *plan.Source, "compose")
	if err != nil {
		return out, err
	}
	expected, err := planner.Build(ctx, *plan.Source, report)
	if err != nil {
		return out, err
	}
	a, err := json.Marshal(plan)
	if err != nil {
		return out, err
	}
	b, err := json.Marshal(expected)
	if err != nil {
		return out, err
	}
	if !bytes.Equal(a, b) {
		return out, errors.New("plan differs from recalculated compatibility and steps")
	}

	c := compose{Name: "eject", Services: map[string]service{}, Networks: map[string]network{}, Volumes: map[string]volume{}}
	lim := limitations{SchemaVersion: "1", Included: []string{}, Excluded: []string{}, Blockers: plan.Blockers, ManualActions: plan.ManualActions, Limitations: append([]string{}, plan.Limitations...)}
	for i := 0; i < len(lim.Limitations); i++ {
		if lim.Limitations[i] == "This plan is a proposal only; no files are generated and no infrastructure is changed." {
			lim.Limitations = append(lim.Limitations[:i], lim.Limitations[i+1:]...)
			break
		}
	}
	variables := map[string]bool{}
	for _, r := range plan.Source.Resources {
		if err := ctx.Err(); err != nil {
			return out, err
		}
		if r.Provider != "aws" {
			lim.Excluded = append(lim.Excluded, r.ID)
			continue
		}
		switch r.Kind {
		case "compute.container":
			if hasBlocker(plan.Blockers, r.ID) {
				lim.Excluded = append(lim.Excluded, r.ID)
				continue
			}
			name, ok := stepName(plan.Steps, r.ID)
			if !ok {
				lim.Excluded = append(lim.Excluded, r.ID)
				continue
			}
			s := service{Image: r.Properties["image"], Environment: map[string]string{}, Expose: []string{}, Networks: []string{"eject"}}
			keys := make([]string, 0, len(r.Properties))
			for key := range r.Properties {
				keys = append(keys, key)
			}
			sort.Strings(keys)
			for _, key := range keys {
				value := r.Properties[key]
				switch {
				case strings.HasPrefix(key, "port_"):
					s.Expose = append(s.Expose, value)
				case strings.HasPrefix(key, "env_name_"):
					s.Environment[value] = "${" + value + ":?set in local environment}"
					variables[value] = true
				}
			}
			c.Services[name] = s
			lim.Included = append(lim.Included, r.ID)
		case "database":
			if r.Properties["engine"] != "postgres" {
				lim.Excluded = append(lim.Excluded, r.ID)
				continue
			}
			name, ok := stepName(plan.Steps, r.ID)
			if !ok {
				lim.Excluded = append(lim.Excluded, r.ID)
				continue
			}
			version := r.Properties["engine_version"]
			if !versionPattern.MatchString(version) || !strings.HasPrefix(version, "16.") {
				lim.Excluded = append(lim.Excluded, r.ID)
				continue
			}
			major, _, _ := strings.Cut(version, ".")
			volumeName := name + "-data"
			c.Services[name] = service{Image: "postgres:" + major + "-alpine", Environment: map[string]string{"POSTGRES_USER": "eject", "POSTGRES_DB": "eject", "POSTGRES_PASSWORD": "${POSTGRES_PASSWORD:?set locally}"}, Networks: []string{"eject"}, Volumes: []string{volumeName + ":/var/lib/postgresql/data"}, Healthcheck: &healthcheck{Test: []string{"CMD-SHELL", "pg_isready -U eject -d eject"}, Interval: "10s", Timeout: "5s", Retries: 5}}
			c.Volumes[volumeName] = volume{}
			variables["POSTGRES_PASSWORD"] = true
			lim.Included = append(lim.Included, r.ID)
			lim.Limitations = append(lim.Limitations, "PostgreSQL image uses the source major version only; verify patch version, extensions and settings before use.")
		default:
			lim.Excluded = append(lim.Excluded, r.ID)
		}
	}
	if len(c.Services) > 0 {
		c.Networks["eject"] = network{Driver: "bridge"}
	}
	lim.Limitations = append(lim.Limitations, "Compose bridge networking does not reproduce ECS awsvpc, IAM, scaling or load balancing.", "No host ports are published; container ports are only exposed within the Compose network.", "No data or secrets are transferred. Generated files must be reviewed before running Docker Compose.")
	if len(c.Services) == 0 {
		lim.Status = "BLOCKED"
	} else if len(lim.Blockers) > 0 || len(lim.ManualActions) > 0 || len(lim.Excluded) > 0 || hasNonSupported(report) {
		lim.Status = "PARTIAL"
	} else {
		lim.Status = "COMPLETE"
	}
	sort.Strings(lim.Included)
	sort.Strings(lim.Excluded)
	sort.Strings(lim.Limitations)
	data, err := json.MarshalIndent(lim, "", "  ")
	if err != nil {
		return out, err
	}
	files := map[string][]byte{"limitations.json": append(data, '\n'), "README.md": []byte(generatedREADME)}
	if len(c.Services) > 0 {
		yamlData, err := yaml.Marshal(c)
		if err != nil {
			return out, err
		}
		var parsed compose
		decoder := yaml.NewDecoder(bytes.NewReader(yamlData))
		decoder.KnownFields(true)
		if err := decoder.Decode(&parsed); err != nil {
			return out, fmt.Errorf("generated YAML: %w", err)
		}
		files["compose.yaml"] = yamlData
		keys := make([]string, 0, len(variables))
		for key := range variables {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		var env strings.Builder
		for _, key := range keys {
			env.WriteString(key + "=\n")
		}
		files[".env.example"] = []byte(env.String())
	}
	planData, err := json.MarshalIndent(plan, "", "  ")
	if err != nil {
		return out, err
	}
	files["plan.json"] = append(planData, '\n')
	marker := completion{SchemaVersion: "1", Status: lim.Status, Files: map[string]string{}}
	for name, content := range files {
		sum := sha256.Sum256(content)
		marker.Files[name] = hex.EncodeToString(sum[:])
	}
	markerData, err := json.MarshalIndent(marker, "", "  ")
	if err != nil {
		return out, err
	}
	files["generation.json"] = append(markerData, '\n')
	return Bundle{Status: lim.Status, Files: files}, nil
}

func hasNonSupported(report domain.CompatibilityReport) bool {
	for _, result := range report.Results {
		if result.Status != domain.Supported {
			return true
		}
	}
	return false
}
func hasBlocker(blockers []domain.PlanIssue, id string) bool {
	for _, blocker := range blockers {
		if blocker.Resource == id {
			return true
		}
	}
	return false
}
func stepName(steps []domain.PlanStep, id string) (string, bool) {
	for _, step := range steps {
		if len(step.SourceResources) == 1 && step.SourceResources[0] == id && step.ExpectedStatus == "CONFIGURATION_PROPOSED" && strings.HasPrefix(step.TargetResource, "compose:service:") {
			return strings.TrimPrefix(step.TargetResource, "compose:service:"), true
		}
	}
	return "", false
}

const generatedREADME = "# Generated Compose proposal\n\nThis directory is a local configuration proposal from EJECT. Read `limitations.json` before running anything. A PARTIAL or BLOCKED status means source behavior is not fully represented.\n\nCopy `.env.example` to a private `.env` file and supply required values. Do not commit `.env` or put credentials in `compose.yaml`. EJECT has not copied data, images or secrets, and has not changed AWS resources.\n\n`compose.yaml` contains only services with supported configuration evidence. Ports are internal to the local bridge network. The PostgreSQL service, when present, starts with an empty named volume and a local health check; it does not contain RDS data. Verify image access, architecture, application behavior, networking and version compatibility independently.\n"
