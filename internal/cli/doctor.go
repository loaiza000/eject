package cli

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"

	"github.com/spf13/cobra"
)

type doctorReport struct {
	SchemaVersion string `json:"schema_version"`
	EjectVersion  string `json:"eject_version"`
	GoVersion     string `json:"go_version"`
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	DockerFound   bool   `json:"docker_found"`
}

func newDoctor(version string) *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Report local environment capabilities",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, dockerErr := exec.LookPath("docker")
			report := doctorReport{"1", version, runtime.Version(), runtime.GOOS, runtime.GOARCH, dockerErr == nil}
			if jsonOutput {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(report)
			}
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "EJECT: %s\nGo: %s\nPlatform: %s/%s\nDocker executable found: %t\n", report.EjectVersion, report.GoVersion, report.OS, report.Arch, report.DockerFound)
			return err
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}
