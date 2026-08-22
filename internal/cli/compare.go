package cli

import (
	"fmt"

	comparison "github.com/charleszardd/daegsa/internal/compare"
	"github.com/charleszardd/daegsa/internal/core"
	"github.com/spf13/cobra"
)

func newCompareCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "compare <baseline.json> <candidate.json>",
		Short: "Compare two complete DAEGSA reports for performance regressions",
		Long: `Compare two complete DAEGSA JSON reports (baseline vs candidate) to detect
latency regressions, throughput changes, error rate spikes, and threshold transitions in CI.

Analyzes p50/p90/p95/p99/max latency deltas, completed throughput differences, 429 response
counts, dropped requests, and threshold transition statuses (pass-to-fail, fail-to-pass).`,
		Example: `  # 1. Compare a baseline report with a new test run:
  daegsa compare baseline.json candidate.json

  # 2. Compare performance reports across staging and production:
  daegsa compare report-v1.0.json report-v1.1.json`,
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
