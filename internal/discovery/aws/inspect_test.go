package aws

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExample(t *testing.T) {
	path := filepath.Join("..", "..", "..", "examples", "aws-basic", "manifest.json")
	ir, err := Inspect(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if len(ir.Resources) != 5 || len(ir.Dependencies) != 2 {
		t.Fatalf("unexpected IR: %+v", ir)
	}
	for _, r := range ir.Resources {
		for _, v := range r.Properties {
			if strings.Contains(v, "synthetic-secret-reference") {
				t.Fatal("secret reference leaked")
			}
		}
	}
	again, err := Inspect(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	firstJSON, err := json.Marshal(ir)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(firstJSON), "synthetic-secret-reference") || strings.Contains(string(firstJSON), "synthetic.example.invalid") {
		t.Fatal("ECS environment value or secret reference leaked")
	}
	secondJSON, err := json.Marshal(again)
	if err != nil || string(firstJSON) != string(secondJSON) {
		t.Fatal("inspection output is not deterministic")
	}
}

func TestManifestValidation(t *testing.T) {
	for _, tc := range []struct{ name, body, want string }{
		{"malformed", "{", "manifest"},
		{"unknown version", `{"schema_version":"2","exports":[{}]}`, "unsupported manifest"},
		{"unknown field", `{"schema_version":"1","exports":[],"extra":true}`, "unknown field"},
		{"null root", `null`, "JSON root must be an object"},
		{"traversal", `{"schema_version":"1","exports":[{"operation":"s3.get-bucket-versioning","file":"../outside.json","resource":"bucket","region":"us-east-1"}]}`, "escapes manifest directory"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			path := filepath.Join(dir, "manifest.json")
			if err := os.WriteFile(path, []byte(tc.body), 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := Inspect(context.Background(), path); err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestOversizedManifest(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(path, []byte(strings.Repeat("x", maxFileSize+1)), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Inspect(context.Background(), path); err == nil {
		t.Fatal("expected size error")
	}
}

func TestJSONLimits(t *testing.T) {
	for _, tc := range []struct {
		name      string
		data      []byte
		wantError bool
	}{
		{"size-at-limit", []byte("{}" + strings.Repeat(" ", maxFileSize-2)), false},
		{"size-over-limit", []byte("{}" + strings.Repeat(" ", maxFileSize-1)), true},
		{"depth-at-limit", []byte(`{"v":` + strings.Repeat("[", maxJSONDepth-1) + "0" + strings.Repeat("]", maxJSONDepth-1) + "}"), false},
		{"depth-over-limit", []byte(`{"v":` + strings.Repeat("[", maxJSONDepth) + "0" + strings.Repeat("]", maxJSONDepth) + "}"), true},
		{"brackets-in-string", []byte(`{"value":"[[[[[\\\"{{{{{"}`), false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "input.json")
			if err := os.WriteFile(path, tc.data, 0600); err != nil {
				t.Fatal(err)
			}
			file, err := os.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer file.Close()
			var result any
			err = readJSON(file, &result, false)
			if (err != nil) != tc.wantError {
				t.Fatalf("readJSON error = %v", err)
			}
		})
	}
}

func TestOperationValidation(t *testing.T) {
	const taskARN = "arn:aws:ecs:us-east-1:123456789012:task-definition/demo:1"
	const serviceARN = "arn:aws:ecs:us-east-1:123456789012:service/demo/app"
	const dbARN = "arn:aws:rds:us-east-1:123456789012:db:demo-db"
	for _, tc := range []struct{ name, operation, resource, response, want string }{
		{"corrupt-task", "ecs.describe-task-definition", taskARN, `{"taskDefinition":`, "unexpected EOF"},
		{"incomplete-task", "ecs.describe-task-definition", taskARN, `{"taskDefinition":{"taskDefinitionArn":"` + taskARN + `","family":"demo"}}`, "containerDefinitions"},
		{"incomplete-service", "ecs.describe-services", serviceARN, `{"services":[{"serviceArn":"` + serviceARN + `","serviceName":"app"}]}`, "taskDefinition"},
		{"duplicate-service", "ecs.describe-services", serviceARN, `{"services":[{"serviceArn":"` + serviceARN + `","serviceName":"app","taskDefinition":"` + taskARN + `"},{"serviceArn":"` + serviceARN + `","serviceName":"app","taskDefinition":"` + taskARN + `"}]}`, "duplicate resource ID"},
		{"incomplete-rds", "rds.describe-db-instances", dbARN, `{"DBInstances":[{"DBInstanceArn":"` + dbARN + `","Engine":"postgres"}]}`, "DBInstanceIdentifier"},
		{"unexpected-versioning", "s3.get-bucket-versioning", "bucket", `{"Status":"Broken"}`, "unexpected versioning status"},
		{"incomplete-encryption", "s3.get-bucket-encryption", "bucket", `{"ServerSideEncryptionConfiguration":{"Rules":[]}}`, "encryption rules"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			if err := os.WriteFile(filepath.Join(dir, "response.json"), []byte(tc.response), 0600); err != nil {
				t.Fatal(err)
			}
			manifest := fmt.Sprintf(`{"schema_version":"1","exports":[{"operation":%q,"file":"response.json","resource":%q,"region":"us-east-1"}]}`, tc.operation, tc.resource)
			path := filepath.Join(dir, "manifest.json")
			if err := os.WriteFile(path, []byte(manifest), 0600); err != nil {
				t.Fatal(err)
			}
			_, err := Inspect(context.Background(), path)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error = %v, want %q", err, tc.want)
			}
		})
	}
}

func TestMissingTaskReference(t *testing.T) {
	const serviceARN = "arn:aws:ecs:us-east-1:123456789012:service/demo/app"
	dir := t.TempDir()
	response := `{"services":[{"serviceArn":"` + serviceARN + `","serviceName":"app","taskDefinition":"missing-task"}]}`
	if err := os.WriteFile(filepath.Join(dir, "service.json"), []byte(response), 0600); err != nil {
		t.Fatal(err)
	}
	manifest := `{"schema_version":"1","exports":[{"operation":"ecs.describe-services","file":"service.json","resource":"` + serviceARN + `","region":"us-east-1"}]}`
	path := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(path, []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	ir, err := Inspect(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	if len(ir.Findings) != 1 || ir.Findings[0].Code != "MISSING_REFERENCE" {
		t.Fatalf("findings = %+v", ir.Findings)
	}
}

func TestDuplicateManifestEntry(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "v.json"), []byte(`{"Status":"Enabled"}`), 0600); err != nil {
		t.Fatal(err)
	}
	entry := `{"operation":"s3.get-bucket-versioning","file":"v.json","resource":"bucket","region":"us-east-1"}`
	manifest := `{"schema_version":"1","exports":[` + entry + `,` + entry + `]}`
	path := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(path, []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	_, err := Inspect(context.Background(), path)
	if err == nil || !strings.Contains(err.Error(), "duplicate operation/resource/region") {
		t.Fatalf("error = %v", err)
	}
}

func TestSymlinkRejected(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "response.json")
	if err := os.WriteFile(target, []byte(`{"Status":"Enabled"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, "link.json")); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	manifest := `{"schema_version":"1","exports":[{"operation":"s3.get-bucket-versioning","file":"link.json","resource":"bucket","region":"us-east-1"}]}`
	path := filepath.Join(dir, "manifest.json")
	if err := os.WriteFile(path, []byte(manifest), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Inspect(context.Background(), path); err == nil {
		t.Fatal("expected symlink rejection")
	}
}

func FuzzManifest(f *testing.F) {
	f.Add([]byte(`{"schema_version":"1","exports":[]}`))
	f.Add([]byte("{"))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > maxFileSize {
			return
		}
		dir := t.TempDir()
		path := filepath.Join(dir, "manifest.json")
		if err := os.WriteFile(path, data, 0600); err != nil {
			t.Fatal(err)
		}
		_, _ = Inspect(context.Background(), path)
	})
}

func FuzzTaskResponse(f *testing.F) {
	seed, err := os.ReadFile(filepath.Join("..", "..", "..", "examples", "aws-basic", "task-definition.json"))
	if err != nil {
		f.Fatal(err)
	}
	f.Add(seed)
	f.Add([]byte(`{"taskDefinition":{}}`))
	f.Fuzz(func(t *testing.T, data []byte) {
		if len(data) > maxFileSize {
			return
		}
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "task.json"), data, 0600); err != nil {
			t.Fatal(err)
		}
		manifest := `{"schema_version":"1","exports":[{"operation":"ecs.describe-task-definition","file":"task.json","resource":"arn:aws:ecs:us-east-1:123456789012:task-definition/demo:1","region":"us-east-1"}]}`
		path := filepath.Join(dir, "manifest.json")
		if err := os.WriteFile(path, []byte(manifest), 0600); err != nil {
			t.Fatal(err)
		}
		_, _ = Inspect(context.Background(), path)
	})
}
