package cli

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/loaiza000/eject/internal/discovery/aws"
	"github.com/spf13/cobra"
)

func newInspect() *cobra.Command {
	var manifest, output string
	var force bool
	cmd := &cobra.Command{
		Use:   "inspect",
		Short: "Inspect AWS CLI JSON snapshots offline",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if manifest == "" {
				return &ExitError{Code: 2, Err: errors.New("--manifest is required")}
			}
			ir, err := aws.Inspect(cmd.Context(), manifest)
			if err != nil {
				return &ExitError{Code: 2, Err: err}
			}
			data, err := json.MarshalIndent(ir, "", "  ")
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
	cmd.Flags().StringVar(&manifest, "manifest", "", "Path to a versioned JSON export manifest")
	cmd.Flags().StringVarP(&output, "output", "o", "", "Output JSON path (default: stdout)")
	cmd.Flags().BoolVar(&force, "force", false, "Replace an existing output file")
	return cmd
}

func writeOutput(path string, data []byte, force bool) error {
	if path == "" {
		return errors.New("empty output path")
	}
	if info, err := os.Lstat(path); err == nil {
		if info.Mode()&os.ModeSymlink != 0 || !info.Mode().IsRegular() {
			return errors.New("output is not a regular file")
		}
		if !force {
			return errors.New("output exists; use --force to replace it")
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, ".eject-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if force {
		// A same-directory rename replaces the file without a missing-file gap.
		// Go does not promise atomic rename visibility on non-Unix platforms.
		return os.Rename(tmp.Name(), path)
	}
	// Link fails if the destination appeared after the earlier existence check.
	// This prevents an accidental overwrite without --force.
	if err := os.Link(tmp.Name(), path); err != nil {
		return fmt.Errorf("create output without overwrite: %w", err)
	}
	return nil
}
