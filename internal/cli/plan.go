package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/loaiza000/eject/internal/compatibility"
	"github.com/loaiza000/eject/internal/domain"
	"github.com/loaiza000/eject/internal/planner"
	"github.com/spf13/cobra"
)

const maxIRSize = 8 << 20

func newPlan() *cobra.Command {
	var input, target, reportKind, output string
	var force bool
	cmd := &cobra.Command{
		Use:   "plan",
		Short: "Assess compatibility and propose a Compose migration plan",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if input == "" {
				return &ExitError{Code: 2, Err: errors.New("--input is required")}
			}
			if reportKind != "plan" && reportKind != "compatibility" {
				return &ExitError{Code: 2, Err: fmt.Errorf("unsupported --report %q", reportKind)}
			}
			infra, err := loadInfrastructure(input)
			if err != nil {
				return &ExitError{Code: 2, Err: fmt.Errorf("infrastructure input: %w", err)}
			}
			assessment, err := compatibility.Evaluate(cmd.Context(), infra, target)
			if err != nil {
				return &ExitError{Code: 2, Err: err}
			}
			var value any = assessment
			if reportKind == "plan" {
				if err := compatibility.ValidateSource(infra); err != nil {
					return &ExitError{Code: 2, Err: err}
				}
				plan, err := planner.Build(cmd.Context(), infra, assessment)
				if err != nil {
					return &ExitError{Code: 2, Err: err}
				}
				value = plan
			}
			data, err := json.MarshalIndent(value, "", "  ")
			if err != nil {
				return err
			}
			data = append(data, '\n')
			if output == "" {
				_, err = cmd.OutOrStdout().Write(data)
				return err
			}
			if err := writeOutput(output, data, force); err != nil {
				return &ExitError{Code: 3, Err: err}
			}
			_, err = fmt.Fprintln(cmd.ErrOrStderr(), "wrote", output)
			return err
		},
	}
	cmd.Flags().StringVar(&input, "input", "", "Versioned infrastructure JSON from eject inspect")
	cmd.Flags().StringVar(&target, "target", "compose", "Target to assess (compose)")
	cmd.Flags().StringVar(&reportKind, "report", "plan", "Output report: plan or compatibility")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output JSON path (default: stdout)")
	cmd.Flags().BoolVar(&force, "force", false, "Replace an existing output file")
	return cmd
}

func loadInfrastructure(path string) (domain.Infrastructure, error) {
	var infra domain.Infrastructure
	absPath, err := filepath.Abs(path)
	if err != nil {
		return infra, err
	}
	root, err := os.OpenRoot(filepath.Dir(absPath))
	if err != nil {
		return infra, err
	}
	defer root.Close()
	name := filepath.Base(absPath)
	info, err := root.Lstat(name)
	if err != nil {
		return infra, err
	}
	if !info.Mode().IsRegular() {
		return infra, errors.New("input must be a regular file, not a symlink")
	}
	if info.Size() > maxIRSize {
		return infra, fmt.Errorf("input exceeds %d bytes", maxIRSize)
	}
	file, err := root.Open(name)
	if err != nil {
		return infra, err
	}
	defer file.Close()
	info, err = file.Stat()
	if err != nil {
		return infra, err
	}
	if !info.Mode().IsRegular() {
		return infra, errors.New("input must be a regular file")
	}
	if info.Size() > maxIRSize {
		return infra, fmt.Errorf("input exceeds %d bytes", maxIRSize)
	}
	data, err := io.ReadAll(io.LimitReader(file, maxIRSize+1))
	if err != nil {
		return infra, err
	}
	if len(data) > maxIRSize {
		return infra, fmt.Errorf("input exceeds %d bytes", maxIRSize)
	}
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 || trimmed[0] != '{' {
		return infra, errors.New("JSON root must be an object")
	}
	if err := checkPlanInputDepth(data); err != nil {
		return infra, err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&infra); err != nil {
		return infra, err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return infra, errors.New("trailing JSON content")
	}
	if len(infra.Resources) > 10000 || len(infra.Dependencies) > 20000 || len(infra.Findings) > 20000 {
		return infra, errors.New("infrastructure entry count exceeds limit")
	}
	return infra, nil
}

func checkPlanInputDepth(data []byte) error {
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
			if depth > 64 {
				return errors.New("JSON nesting exceeds 64 levels")
			}
		case '}', ']':
			depth--
		}
	}
	return nil
}
