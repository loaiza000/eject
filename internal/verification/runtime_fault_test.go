package verification

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/loaiza000/eject/internal/generator"
)

func runtimeFixture() generator.Bundle {
	return generator.Bundle{Status: "PARTIAL", Files: map[string][]byte{"compose.yaml": []byte("services:\n  db:\n    image: postgres:16-alpine\n    healthcheck:\n      test: [CMD, true]\n")}}
}
func reportHas(report Report, code string, status Status) bool {
	for _, check := range report.Checks {
		if check.Code == code && check.Status == status {
			return true
		}
	}
	return false
}
func TestRuntimeFailurePathsAndIsolation(t *testing.T) {
	for _, tc := range []struct {
		name, fail, code string
		down             bool
	}{
		{"healthy", "", "POSTGRES_QUERY", true},
		{"daemon", "info", "DOCKER", false},
		{"unresolved-config", "config", "RUNTIME_CONFIG", false},
		{"startup", "up", "STARTUP", true},
		{"unhealthy", "unhealthy", "HEALTHCHECK", true},
		{"postgres-readiness", "pg_isready", "POSTGRES_CONNECTIVITY", true},
		{"postgres-query", "psql", "POSTGRES_QUERY", true},
		{"cleanup", "down", "CLEANUP", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls [][]string
			command := func(ctx context.Context, args ...string) ([]byte, error) {
				calls = append(calls, append([]string{}, args...))
				joined := strings.Join(args, " ")
				if strings.Contains(joined, " info ") || (len(args) > 0 && args[0] == "info") {
					if tc.fail == "info" {
						return nil, errors.New("daemon")
					}
					return []byte("27"), nil
				}
				if strings.HasSuffix(joined, " config --quiet") {
					if tc.fail == "config" {
						return nil, errors.New("required variable POSTGRES_PASSWORD is missing a value")
					}
					return nil, nil
				}
				if strings.Contains(joined, " up ") {
					if tc.fail == "up" {
						return nil, errors.New("startup")
					}
					return nil, nil
				}
				if strings.Contains(joined, " ps ") {
					health := "healthy"
					if tc.fail == "unhealthy" {
						health = "unhealthy"
					}
					return []byte(`{"Service":"db","State":"running","Health":"` + health + `"}`), nil
				}
				if strings.Contains(joined, " pg_isready ") {
					if tc.fail == "pg_isready" {
						return nil, errors.New("database")
					}
					return nil, nil
				}
				if strings.Contains(joined, " psql ") {
					if tc.fail == "psql" {
						return nil, errors.New("query")
					}
					return []byte("1\n"), nil
				}
				if strings.Contains(joined, " down ") {
					if tc.fail == "down" {
						return nil, errors.New("cleanup")
					}
					return nil, nil
				}
				return nil, errors.New("unexpected Docker command")
			}
			report := NewReport("runtime")
			runtimeWithCommand(context.Background(), t.TempDir(), runtimeFixture(), false, nil, command, &report)
			want := Passed
			if tc.fail != "" {
				want = Failed
			}
			if report.Outcome != want || !reportHas(report, tc.code, want) {
				t.Fatalf("report=%+v", report)
			}
			var project string
			downCount := 0
			for _, args := range calls {
				for i, arg := range args {
					if arg == "-p" && i+1 < len(args) {
						if project == "" {
							project = args[i+1]
						} else if project != args[i+1] {
							t.Fatal("Docker commands used different projects")
						}
					}
				}
				joined := strings.Join(args, " ")
				if strings.Contains(joined, " down ") {
					downCount++
					if strings.Contains(joined, " -v") {
						t.Fatal("persistent volume deletion requested")
					}
				}
				if tc.fail == "config" && strings.Contains(joined, " up ") {
					t.Fatal("containers started after an unresolved configuration")
				}
				if strings.Contains(joined, " up ") && !strings.Contains(joined, "--pull never") {
					t.Fatal("unexpected image pull authorization")
				}
			}
			if (downCount == 1) != tc.down {
				t.Fatalf("cleanup calls=%d", downCount)
			}
			if tc.down && !strings.HasPrefix(project, "eject-verify-") {
				t.Fatalf("unsafe project name %q", project)
			}
		})
	}
}
func TestRuntimeCancellationAndTimeout(t *testing.T) {
	for _, tc := range []struct {
		name    string
		context func() (context.Context, context.CancelFunc)
		code    string
	}{
		{"canceled", func() (context.Context, context.CancelFunc) {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			return ctx, cancel
		}, "RUNTIME_CANCELED"},
		{"deadline", func() (context.Context, context.CancelFunc) {
			return context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
		}, "RUNTIME_TIMEOUT"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx, cancel := tc.context()
			defer cancel()
			command := func(ctx context.Context, _ ...string) ([]byte, error) { return nil, ctx.Err() }
			report := NewReport("runtime")
			runtimeWithCommand(ctx, t.TempDir(), runtimeFixture(), false, nil, command, &report)
			if !reportHas(report, tc.code, Failed) {
				t.Fatalf("interruption not reported: %+v", report)
			}
		})
	}
}

func TestRuntimeHTTPProbeAndTemporaryOverrideCleanup(t *testing.T) {
	for _, status := range []int{http.StatusOK, http.StatusServiceUnavailable} {
		t.Run(http.StatusText(status), func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/health" {
					t.Errorf("unexpected HTTP path %q", r.URL.Path)
				}
				w.WriteHeader(status)
			}))
			defer server.Close()
			var override string
			var downCount int
			command := func(_ context.Context, args ...string) ([]byte, error) {
				joined := " " + strings.Join(args, " ") + " "
				switch {
				case args[0] == "info":
					return []byte("27"), nil
				case strings.HasSuffix(joined, " config --quiet "):
					return nil, nil
				case strings.Contains(joined, " up "):
					for i, arg := range args {
						if arg == "-f" && i+1 < len(args) && strings.HasPrefix(args[i+1], os.TempDir()) {
							override = args[i+1]
						}
					}
					return nil, nil
				case strings.Contains(joined, " ps "):
					return []byte(`{"Service":"api","State":"running","Health":"healthy"}`), nil
				case strings.Contains(joined, " port "):
					return []byte(strings.TrimPrefix(server.URL, "http://") + "\n"), nil
				case strings.Contains(joined, " down "):
					downCount++
					return nil, nil
				}
				return nil, errors.New("unexpected command")
			}
			bundle := generator.Bundle{Files: map[string][]byte{"compose.yaml": []byte("services:\n  api:\n    image: example:1\n    expose: [8080/tcp]\n    healthcheck:\n      test: [CMD, true]\n")}}
			report := NewReport("runtime")
			runtimeWithCommand(context.Background(), t.TempDir(), bundle, false, []string{"api:8080:/health"}, command, &report)
			want := Passed
			if status != http.StatusOK {
				want = Failed
			}
			if !reportHas(report, "HTTP", want) || !reportHas(report, "CLEANUP", Passed) || downCount != 1 {
				t.Fatalf("unexpected report or cleanup: %+v, down=%d", report, downCount)
			}
			if override == "" {
				t.Fatal("missing temporary HTTP override")
			}
			if _, err := os.Stat(override); !os.IsNotExist(err) {
				t.Fatalf("temporary HTTP override remains: %v", err)
			}
		})
	}
}
