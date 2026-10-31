// Package cli defines EJECT's command-line interface.
package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// ExitError associates an operational error with a documented exit status.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string { return e.Err.Error() }
func (e *ExitError) Unwrap() error { return e.Err }

// New constructs the CLI without process-wide mutable state.
func New(version string) *cobra.Command {
	root := &cobra.Command{
		Use:               "eject",
		Short:             "Inspect cloud infrastructure for portable deployment",
		Long:              "EJECT is a local, offline-first cloud portability CLI. It inspects selected AWS CLI JSON snapshots, assesses Compose compatibility, and generates a local configuration proposal.",
		SilenceUsage:      true,
		SilenceErrors:     true,
		Version:           version,
		CompletionOptions: cobra.CompletionOptions{DisableDefaultCmd: true},
	}
	root.SetVersionTemplate(fmt.Sprintf("eject %s\n", version))
	root.AddCommand(newVersion(version), newDoctor(version), newInspect(), newPlan(), newGenerate())
	return root
}
