package cli

import (
	"fmt"
	"time"

	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/selftest"
	"github.com/spf13/cobra"
)

type selfTestFlagValues struct {
	jsonOutput bool
	verbose    bool
	timeout    time.Duration
}

func newSelfTestCmd() *cobra.Command {
	var flags selfTestFlagValues

	cmd := &cobra.Command{
		Use:     "self-test [flags]",
		Aliases: []string{"selftest"},
		Short:   "Run in-process end-to-end self-tests against an embedded target",
		Long: `Run comprehensive in-process verification across DAEGSA core engines:
  - Closed workload VU loop & latency quantile calculation
  - Open arrival-rate pacing & max-in-flight drop semantics
  - Multi-step scenario variable extraction & cookie jar chaining
  - Threshold rule evaluation (passing and failing constraints)
  - Terminal reporting and JSON schema serialization

Uses an embedded in-memory HTTP server on loopback without sending any external traffic.
Returns exit code 0 if all self-tests pass, and exit code 3 (RUNTIME_FAILURE)
if any subtest encounters an assertion or execution failure.`,
		Example: `  # 1. Run automated in-process self-tests:
  daegsa self-test

  # 2. Run self-tests with verbose per-test progress and details:
  daegsa self-test --verbose

  # 3. Export self-test results in JSON format:
  daegsa self-test --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := selftest.Options{
				Verbose: flags.verbose,
				Timeout: flags.timeout,
			}

			stepIndex := 0
			var progressCallback func(selftest.SubTestResult)
			if !flags.jsonOutput {
				fmt.Fprintln(cmd.OutOrStdout(), "Running DAEGSA in-process self-tests...")
				progressCallback = func(res selftest.SubTestResult) {
					stepIndex++
					statusBadge := fmt.Sprintf("[%s]", res.Status)
					fmt.Fprintf(cmd.OutOrStdout(), "  [%d/5] %-38s %-6s (%v)\n",
						stepIndex, res.Name+"...", statusBadge, res.Duration.Truncate(time.Millisecond))
					if flags.verbose && res.Detail != "" {
						fmt.Fprintf(cmd.OutOrStdout(), "        -> %s\n", res.Detail)
					}
				}
			}

			rep := selftest.RunSelfTests(cmd.Context(), opts, progressCallback)

			if flags.jsonOutput {
				jsonBytes, err := rep.JSON()
				if err != nil {
					return &CLIExitError{Code: core.ExitCodeRuntimeFailure, Err: fmt.Errorf("failed to serialize self-test JSON: %w", err)}
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(jsonBytes))
			} else {
				fmt.Fprintln(cmd.OutOrStdout())
				if rep.Passed {
					fmt.Fprintf(cmd.OutOrStdout(), "ALL SELF-TESTS PASSED (5/5 tests passed in %v).\n", rep.TotalDuration.Truncate(time.Millisecond))
				} else {
					fmt.Fprintf(cmd.OutOrStdout(), "SELF-TESTS FAILED (completed in %v).\n", rep.TotalDuration.Truncate(time.Millisecond))
				}
			}

			if !rep.Passed {
				var failedNames []string
				for _, t := range rep.Tests {
					if t.Status != selftest.StatusPass {
						failedNames = append(failedNames, t.Name)
					}
				}
				return &CLIExitError{
					Code: core.ExitCodeRuntimeFailure,
					Err:  fmt.Errorf("self-test failures detected in: %v", failedNames),
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&flags.jsonOutput, "json", false, "Output self-test results in JSON format")
	cmd.Flags().BoolVarP(&flags.verbose, "verbose", "v", false, "Show detailed per-test metrics and step details")
	cmd.Flags().DurationVar(&flags.timeout, "timeout", 30*time.Second, "Total self-test timeout duration")

	return cmd
}
