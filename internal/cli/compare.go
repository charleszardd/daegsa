package cli

import (
	"fmt"

	comparison "github.com/charleszardd/daegsa/internal/compare"
	"github.com/charleszardd/daegsa/internal/core"
	"github.com/spf13/cobra"
)

func newCompareCmd() *cobra.Command {
	return &cobra.Command{
		Use: "compare baseline.json candidate.json", Short: "Compare two complete DAEGSA reports",
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return &CLIExitError{Code: core.ExitCodeValidationFailure, Err: fmt.Errorf("compare requires baseline and candidate report paths")}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			baseline, err := comparison.LoadReport(args[0])
			if err != nil {
				return &CLIExitError{Code: core.ExitCodeValidationFailure, Err: err}
			}
			candidate, err := comparison.LoadReport(args[1])
			if err != nil {
				return &CLIExitError{Code: core.ExitCodeValidationFailure, Err: err}
			}
			result, err := comparison.Compare(baseline, candidate)
			if err != nil {
				return &CLIExitError{Code: core.ExitCodeValidationFailure, Err: err}
			}
			fmt.Fprint(cmd.OutOrStdout(), result.String())
			return nil
		},
	}
}
