package verification

import (
	"os"
	"strings"
	"testing"
)

func TestHTTPCheckValidation(t *testing.T) {
	config := runtimeCompose{Services: map[string]struct {
		Image       string   `yaml:"image"`
		Expose      []string `yaml:"expose"`
		Healthcheck any      `yaml:"healthcheck"`
	}{"api": {Expose: []string{"8080/tcp"}}}}
	for _, value := range []string{"api:0:/health", "api:8080:http://evil", "other:8080:/health", "api:9999:/health", "api:8080:/bad path"} {
		if _, err := parseHTTPChecks([]string{value}, config); err == nil {
			t.Fatalf("accepted %q", value)
		}
	}
	checks, err := parseHTTPChecks([]string{"api:8080:/health"}, config)
	if err != nil || len(checks) != 1 {
		t.Fatalf("check=%+v error=%v", checks, err)
	}
	path, err := writeHTTPOverride(checks)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Remove(path) })
	data, err := os.ReadFile(path)
	if err != nil || !strings.Contains(string(data), "127.0.0.1::8080") {
		t.Fatalf("override=%q error=%v", data, err)
	}
}
func TestParseStates(t *testing.T) {
	states, err := parseStates([]byte("{\"Service\":\"api\",\"State\":\"running\",\"Health\":\"healthy\"}\n"))
	if err != nil || states["api"].Health != "healthy" {
		t.Fatalf("states=%+v error=%v", states, err)
	}
	if _, err := parseStates([]byte("[]")); err == nil {
		t.Fatal("invalid state accepted")
	}
	if _, err := parseStates([]byte("{\"Service\":\"api\",\"State\":\"running\"}\n{\"Service\":\"api\",\"State\":\"exited\"}")); err == nil {
		t.Fatal("duplicate service state accepted")
	}
}

func FuzzParseStates(f *testing.F) {
	f.Add([]byte(`{"Service":"api","State":"running","Health":"healthy"}`))
	f.Add([]byte(`[]`))
	f.Fuzz(func(t *testing.T, data []byte) { _, _ = parseStates(data) })
}

func FuzzHTTPCheck(f *testing.F) {
	f.Add("api:8080:/health")
	f.Add("api:8080://unexpected")
	config := runtimeCompose{Services: map[string]struct {
		Image       string   `yaml:"image"`
		Expose      []string `yaml:"expose"`
		Healthcheck any      `yaml:"healthcheck"`
	}{"api": {Expose: []string{"8080/tcp"}}}}
	f.Fuzz(func(t *testing.T, value string) { _, _ = parseHTTPChecks([]string{value}, config) })
}
