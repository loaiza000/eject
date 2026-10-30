package cli

import (
	"encoding/json"
	"fmt"

	"github.com/spf13/cobra"
)

func newVersion(version string) *cobra.Command {
	var jsonOutput bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the EJECT version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if jsonOutput {
				return json.NewEncoder(cmd.OutOrStdout()).Encode(struct {
					Version string `json:"version"`
				}{version})
			}
			_, err := fmt.Fprintf(cmd.OutOrStdout(), "eject %s\n", version)
			return err
		},
	}
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Print machine-readable JSON")
	return cmd
}
