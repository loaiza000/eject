package verification

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/loaiza000/eject/internal/generator"
	"gopkg.in/yaml.v3"
)

type runtimeCompose struct {
	Services map[string]struct {
		Image       string   `yaml:"image"`
		Expose      []string `yaml:"expose"`
		Healthcheck any      `yaml:"healthcheck"`
	} `yaml:"services"`
}
type containerState struct {
	Service string `json:"Service"`
	State   string `json:"State"`
	Health  string `json:"Health"`
}
type httpCheck struct {
	service string
	port    int
	path    string
}

var httpCheckPattern = regexp.MustCompile(`^([a-z0-9][a-z0-9-]*):([0-9]{1,5}):(/[A-Za-z0-9/_?=&.-]*)$`)

// Runtime starts only a unique Compose project after explicit caller authorization.
// It uses fixed Docker subcommands, never plan-supplied command text.
func Runtime(ctx context.Context, project string, bundle generator.Bundle, allowPull bool, checks []string, env []string, report *Report) {
	docker, err := exec.LookPath("docker")
	if err != nil {
		report.Add("DOCKER", Skipped, "Docker executable unavailable.")
		report.Outcome = Skipped
		return
	}
	runtimeWithCommand(ctx, project, bundle, allowPull, checks, func(ctx context.Context, args ...string) ([]byte, error) {
		return outputDocker(ctx, docker, env, args...)
	}, report)
}

// dockerCommand is injected in tests to exercise failures without starting containers.
type dockerCommand func(context.Context, ...string) ([]byte, error)

func runtimeWithCommand(ctx context.Context, project string, bundle generator.Bundle, allowPull bool, checks []string, command dockerCommand, report *Report) {
	var config runtimeCompose
	if err := yaml.Unmarshal(bundle.Files["compose.yaml"], &config); err != nil {
		report.Add("RUNTIME_CONFIG", Failed, "Generated Compose could not be parsed.")
		return
	}
	parsed, err := parseHTTPChecks(checks, config)
	if err != nil {
		report.Add("HTTP_CONFIGURATION", Failed, "HTTP check must name an exposed service port and safe path.")
		return
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	defer func() {
		if err := ctx.Err(); err != nil {
			code := "RUNTIME_CANCELED"
			if errors.Is(err, context.DeadlineExceeded) {
				code = "RUNTIME_TIMEOUT"
			}
			report.Add(code, Failed, "Runtime verification was interrupted; cleanup was attempted.")
		}
	}()
	if _, err := command(ctx, "info", "--format", "{{.ServerVersion}}"); err != nil {
		report.Add("DOCKER", Failed, "Docker daemon is unavailable.")
		return
	}
	report.Add("DOCKER", Passed, "Local Docker daemon responded.")
	random := make([]byte, 8)
	if _, err := rand.Read(random); err != nil {
		report.Add("PROJECT_ISOLATION", Failed, "Cannot create project identifier.")
		return
	}
	name := "eject-verify-" + hex.EncodeToString(random)
	report.Add("PROJECT_ISOLATION", Passed, "Using a unique Docker Compose project: "+name)
	composeFile := filepath.Join(project, "compose.yaml")
	base := []string{"compose", "-f", composeFile, "-p", name}
	if len(parsed) > 0 {
		override, err := writeHTTPOverride(parsed)
		if err != nil {
			report.Add("HTTP_CONFIGURATION", Failed, "Cannot prepare local HTTP port mapping.")
			return
		}
		defer func() {
			if err := os.Remove(override); err != nil && !os.IsNotExist(err) {
				report.Add("TEMP_CLEANUP", Failed, "Could not remove the temporary HTTP override file.")
			}
		}()
		base = []string{"compose", "-f", composeFile, "-f", override, "-p", name}
	}
	if _, err := os.Stat(filepath.Join(project, ".env")); err == nil {
		base = append(base, "--env-file", filepath.Join(project, ".env"))
	}
	pull := "never"
	if allowPull {
		pull = "missing"
	}
	args := append(append([]string{}, base...), "up", "-d", "--no-build", "--pull", pull, "--wait", "--wait-timeout", "90")
	defer func() {
		downCtx, stop := context.WithTimeout(context.Background(), 30*time.Second)
		defer stop()
		if _, err := command(downCtx, append(append([]string{}, base...), "down", "--remove-orphans")...); err != nil {
			report.Add("CLEANUP", Failed, "Could not stop this verification project; inspect it manually by project name "+name+".")
		} else {
			report.Add("CLEANUP", Passed, "Stopped isolated project "+name+"; persistent volumes were retained.")
		}
	}()
	if _, err := command(ctx, args...); err != nil {
		report.Add("STARTUP", Failed, "Compose could not start and reach its health checks. No image pull was allowed unless --allow-pull was set.")
		return
	}
	report.Add("STARTUP", Passed, "Isolated Compose project started and passed Compose wait checks.")
	output, err := command(ctx, append(append([]string{}, base...), "ps", "--all", "--format", "json")...)
	if err != nil {
		report.Add("CONTAINER_STATE", Failed, "Cannot read container states.")
		return
	}
	states, err := parseStates(output)
	if err != nil {
		report.Add("CONTAINER_STATE", Failed, "Docker returned an unreadable container state report.")
		return
	}
	serviceNames := make([]string, 0, len(config.Services))
	for service := range config.Services {
		serviceNames = append(serviceNames, service)
	}
	sort.Strings(serviceNames)
	for _, service := range serviceNames {
		spec := config.Services[service]
		state, ok := states[service]
		if !ok || state.State != "running" {
			report.Add("CONTAINER_STATE", Failed, "Service is not running: "+service)
			continue
		}
		report.Add("CONTAINER_STATE", Passed, "Service is running: "+service)
		if state.Health == "healthy" {
			report.Add("HEALTHCHECK", Passed, "Service health check passed: "+service)
		} else if state.Health == "" && spec.Healthcheck == nil {
			report.Add("HEALTHCHECK", NotVerified, "No health check configured for service: "+service)
		} else {
			report.Add("HEALTHCHECK", Failed, "Service health check is not healthy: "+service)
		}
		if strings.HasPrefix(spec.Image, "postgres:16") {
			if _, err := command(ctx, append(append([]string{}, base...), "exec", "-T", service, "pg_isready", "-U", "eject", "-d", "eject")...); err != nil {
				report.Add("POSTGRES_CONNECTIVITY", Failed, "PostgreSQL did not accept a local readiness probe: "+service)
			} else {
				report.Add("POSTGRES_CONNECTIVITY", Passed, "PostgreSQL accepted a local readiness probe: "+service)
			}
			query, err := command(ctx, append(append([]string{}, base...), "exec", "-T", service, "psql", "-U", "eject", "-d", "eject", "-tAc", "SELECT 1")...)
			if err != nil || strings.TrimSpace(string(query)) != "1" {
				report.Add("POSTGRES_QUERY", Failed, "PostgreSQL did not return SELECT 1: "+service)
			} else {
				report.Add("POSTGRES_QUERY", Passed, "PostgreSQL returned SELECT 1: "+service)
			}
		}
	}
	if len(parsed) == 0 {
		report.Add("HTTP", Skipped, "No HTTP checks were configured.")
	}
	client := &http.Client{Timeout: 5 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	for _, check := range parsed {
		portText := strconv.Itoa(check.port)
		output, err := command(ctx, append(append([]string{}, base...), "port", check.service, portText)...)
		if err != nil {
			report.Add("HTTP", Failed, "Cannot resolve local HTTP port for "+check.service)
			continue
		}
		address := strings.TrimSpace(string(output))
		host, hostPort, found := strings.Cut(address, ":")
		if !found || host != "127.0.0.1" || hostPort == "" {
			report.Add("HTTP", Failed, "Docker returned no local HTTP port for "+check.service)
			continue
		}
		url := "http://127.0.0.1:" + hostPort + check.path
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			report.Add("HTTP", Failed, "Invalid local HTTP URL.")
			continue
		}
		resp, err := client.Do(req)
		if err != nil {
			report.Add("HTTP", Failed, "Local HTTP probe failed for "+check.service)
			continue
		}
		resp.Body.Close()
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			report.Add("HTTP", Failed, "Local HTTP endpoint returned an error for "+check.service)
		} else {
			report.Add("HTTP", Passed, "Local HTTP endpoint responded for "+check.service)
		}
	}
	report.Add("DATA_EQUIVALENCE", NotVerified, "No data migration or application equivalence was tested.")
}

func parseHTTPChecks(values []string, config runtimeCompose) ([]httpCheck, error) {
	var result []httpCheck
	for _, value := range values {
		parts := httpCheckPattern.FindStringSubmatch(value)
		if parts == nil {
			return nil, errors.New("invalid HTTP check")
		}
		port, _ := strconv.Atoi(parts[2])
		if port < 1 || port > 65535 {
			return nil, errors.New("invalid HTTP port")
		}
		spec, ok := config.Services[parts[1]]
		if !ok {
			return nil, errors.New("unknown HTTP service")
		}
		found := false
		for _, exposed := range spec.Expose {
			if exposed == fmt.Sprintf("%d/tcp", port) {
				found = true
			}
		}
		if !found {
			return nil, errors.New("HTTP port is not exposed")
		}
		result = append(result, httpCheck{service: parts[1], port: port, path: parts[3]})
	}
	return result, nil
}
func writeHTTPOverride(checks []httpCheck) (string, error) {
	services := map[string]map[string][]string{}
	for _, check := range checks {
		if services[check.service] == nil {
			services[check.service] = map[string][]string{"ports": {}}
		}
		port := fmt.Sprintf("127.0.0.1::%d", check.port)
		found := false
		for _, existing := range services[check.service]["ports"] {
			if existing == port {
				found = true
			}
		}
		if !found {
			services[check.service]["ports"] = append(services[check.service]["ports"], port)
		}
	}
	data, err := yaml.Marshal(map[string]any{"services": services})
	if err != nil {
		return "", err
	}
	f, err := os.CreateTemp("", "eject-verify-*.yaml")
	if err != nil {
		return "", err
	}
	if err := f.Chmod(0600); err != nil {
        _ = f.Close()
        _ = os.Remove(f.Name())
        return "", err
    }
    if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(f.Name())
		return "", err
	}
	return f.Name(), nil
}
func parseStates(data []byte) (map[string]containerState, error) {
	states := map[string]containerState{}
	for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		var state containerState
		if err := json.Unmarshal([]byte(line), &state); err != nil {
			return nil, err
		}
		if state.Service == "" {
			return nil, errors.New("missing service")
		}
		if _, duplicate := states[state.Service]; duplicate {
			return nil, errors.New("duplicate service state")
		}
		states[state.Service] = state
	}
	return states, nil
}
func outputDocker(ctx context.Context, executable string, env []string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, executable, args...)
	cmd.Env = env
	return cmd.Output()
}
