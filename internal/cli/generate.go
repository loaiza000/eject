package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"time"

	"github.com/loaiza000/eject/internal/domain"
	"github.com/loaiza000/eject/internal/generator"
	"github.com/spf13/cobra"
)

const maxPlanSize = 32 << 20

func newGenerate() *cobra.Command {
	var planPath, output string
	cmd := &cobra.Command{Use: "generate", Short: "Generate a reviewed local Docker Compose proposal", Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if planPath == "" || output == "" {
				return &ExitError{Code: 2, Err: errors.New("--plan and --output are required")}
			}
			plan, err := loadPlan(planPath)
			if err != nil {
				return &ExitError{Code: 2, Err: fmt.Errorf("plan input: %w", err)}
			}
			bundle, err := generator.Build(cmd.Context(), plan)
			if err != nil {
				return &ExitError{Code: 2, Err: err}
			}
			if err := writeBundle(cmd.Context(), output, bundle); err != nil {
				return &ExitError{Code: 3, Err: err}
			}
			_, err = fmt.Fprintf(cmd.OutOrStdout(), "%s: %s\n", bundle.Status, output)
			return err
		}}
	cmd.Flags().StringVar(&planPath, "plan", "", "Version 2 JSON plan from eject plan")
	cmd.Flags().StringVarP(&output, "output", "o", "", "New output directory, or an identical existing generation")
	return cmd
}

func loadPlan(path string) (domain.Plan, error) {
	var plan domain.Plan
	abs, err := filepath.Abs(path)
	if err != nil {
		return plan, err
	}
	if err := rejectSymlinkPath(filepath.Dir(abs)); err != nil {
		return plan, err
	}
	root, err := os.OpenRoot(filepath.Dir(abs))
	if err != nil {
		return plan, err
	}
	defer root.Close()
	name := filepath.Base(abs)
	info, err := root.Lstat(name)
	if err != nil {
		return plan, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxPlanSize {
		return plan, errors.New("plan must be a regular file of at most 32 MiB")
	}
	f, err := root.Open(name)
	if err != nil {
		return plan, err
	}
	defer f.Close()
	info, err = f.Stat()
	if err != nil {
		return plan, err
	}
	if !info.Mode().IsRegular() || info.Size() > maxPlanSize {
		return plan, errors.New("plan must be a regular file of at most 32 MiB")
	}
	data, err := io.ReadAll(io.LimitReader(f, maxPlanSize+1))
	if err != nil {
		return plan, err
	}
	if len(data) > maxPlanSize {
		return plan, errors.New("plan exceeds 32 MiB")
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return plan, errors.New("JSON root must be an object")
	}
	if err := checkPlanInputDepth(data); err != nil {
		return plan, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&plan); err != nil {
		return plan, err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return plan, errors.New("trailing JSON content")
	}
	return plan, nil
}

func writeBundle(ctx context.Context, path string, bundle generator.Bundle) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	parent := filepath.Dir(abs)
	if err := rejectSymlinkPath(parent); err != nil {
		return err
	}
	info, err := os.Lstat(parent)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return errors.New("output parent is not a directory")
	}
	if info, err := os.Lstat(abs); err == nil {
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("output exists and is not a regular directory")
		}
		return compareBundle(abs, bundle.Files)
	} else if !os.IsNotExist(err) {
		return err
	}
	keys := make([]string, 0, len(bundle.Files))
	for key := range bundle.Files {
		if key != "compose.yaml" && key != ".env.example" && key != "README.md" && key != "limitations.json" && key != "plan.json" && key != "generation.json" {
			return errors.New("unsafe generated filename")
		}
		keys = append(keys, key)
	}
	sort.Strings(keys)
	parentRoot, err := os.OpenRoot(parent)
	if err != nil {
		return err
	}
	defer parentRoot.Close()
	if err := parentRoot.Mkdir(filepath.Base(abs), 0700); err != nil {
		return fmt.Errorf("create output without overwrite: %w", err)
	}
	projectRoot, err := parentRoot.OpenRoot(filepath.Base(abs))
	if err != nil {
		return err
	}
	defer projectRoot.Close()
	for _, key := range keys {
		if key == "generation.json" {
			continue
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := writeGeneratedFile(projectRoot, key, bundle.Files[key]); err != nil {
			return err
		}
	}
	if _, ok := bundle.Files["compose.yaml"]; ok {
		if err := validateComposeWithDocker(ctx, filepath.Join(abs, "compose.yaml")); err != nil {
			return err
		}
	}
	if marker, ok := bundle.Files["generation.json"]; ok {
		if err := ctx.Err(); err != nil {
			return err
		}
		if err := writeGeneratedFile(projectRoot, "generation.json", marker); err != nil {
			return err
		}
	}
	return nil
}

func writeGeneratedFile(root *os.Root, name string, data []byte) error {
	f, err := root.OpenFile(name, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return err
	}
	return f.Close()
}

func compareBundle(path string, files map[string][]byte) error {
	parent, err := os.OpenRoot(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer parent.Close()
	root, err := parent.OpenRoot(filepath.Base(path))
	if err != nil {
		return err
	}
	defer root.Close()
	dir, err := root.Open(".")
	if err != nil {
		return err
	}
	entries, err := dir.ReadDir(-1)
	closeErr := dir.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if len(entries) != len(files) {
		return errors.New("output directory differs from generated files; refusing overwrite")
	}
	for _, entry := range entries {
		want, ok := files[entry.Name()]
		if !ok || !entry.Type().IsRegular() {
			return errors.New("output directory contains an unexpected or unsafe file")
		}
		f, err := root.Open(entry.Name())
		if err != nil {
			return err
		}
		got, readErr := io.ReadAll(io.LimitReader(f, int64(len(want))+1))
		closeErr := f.Close()
		if readErr != nil {
			return readErr
		}
		if closeErr != nil {
			return closeErr
		}
		if !bytes.Equal(got, want) {
			return errors.New("output directory differs from generated files; refusing overwrite")
		}
	}
	return nil
}

func rejectSymlinkPath(path string) error {
	abs, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	for current := abs; ; current = filepath.Dir(current) {
		info, err := os.Lstat(current)
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlink in output path: %s", current)
		}
		if next := filepath.Dir(current); next == current {
			break
		}
	}
	return nil
}

func validateComposeWithDocker(ctx context.Context, path string) error {
	docker, err := exec.LookPath("docker")
	if errors.Is(err, exec.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, docker, "compose", "-f", path, "config", "--no-interpolate", "--quiet")
	for _, key := range []string{"PATH", "APPDATA", "LOCALAPPDATA", "USERPROFILE", "PROGRAMDATA", "HOME", "DOCKER_CONFIG", "DOCKER_CLI_PLUGIN_EXTRA_DIRS"} {
		if value, ok := os.LookupEnv(key); ok {
			cmd.Env = append(cmd.Env, key+"="+value)
		}
	}
	probe := exec.CommandContext(ctx, docker, "compose", "version", "--short")
	probe.Env = cmd.Env
	if _, err := probe.Output(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return nil
	}
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("Docker Compose validation failed: %w: %s", err, bytes.TrimSpace(output))
	}
	return nil
}
