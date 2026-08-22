package cli

import (
	"fmt"
	"time"

	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/doctor"
	"github.com/spf13/cobra"
)

type doctorFlagValues struct {
	jsonOutput bool
	verbose    bool
	timeout    time.Duration
}

func newDoctorCmd() *cobra.Command {
	var flags doctorFlagValues

	cmd := &cobra.Command{
		Use:   "doctor",
		Short: "Run host environment diagnostics and health checks",
		Long: `Run comprehensive diagnostic checks against the host environment:
  - Timer and monotonic clock precision
  - Loopback and local DNS resolution
  - TLS handshake and system root CA certificates
  - TCP socket allocation and ephemeral port capacity
  - Host CPU cores, GOMAXPROCS, and memory headroom

Returns exit code 0 if all checks pass or warn, and exit code 3 (RUNTIME_FAILURE)
if any critical system check fails.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := doctor.Options{
				Verbose: flags.verbose,
				Timeout: flags.timeout,
			}

			rep := doctor.RunDiagnostics(cmd.Context(), opts)

			if flags.jsonOutput {
				jsonBytes, err := rep.JSON()
				if err != nil {
					return &CLIExitError{Code: core.ExitCodeRuntimeFailure, Err: fmt.Errorf("failed to serialize diagnostic JSON: %w", err)}
				}
				fmt.Fprintln(cmd.OutOrStdout(), string(jsonBytes))
			} else {
				fmt.Fprint(cmd.OutOrStdout(), doctor.FormatTerminalReport(rep, flags.verbose))
			}

			if rep.OverallStatus == doctor.StatusFail {
				var failedChecks []string
				for _, c := range rep.Checks {
					if c.Status == doctor.StatusFail {
						failedChecks = append(failedChecks, c.Name)
					}
				}
				return &CLIExitError{
					Code: core.ExitCodeRuntimeFailure,
					Err:  fmt.Errorf("system diagnostics failed for: %s", fmt.Sprintf("%v", failedChecks)),
				}
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&flags.jsonOutput, "json", false, "Output diagnostics in JSON format")
	cmd.Flags().BoolVarP(&flags.verbose, "verbose", "v", false, "Show detailed diagnostic measurements and suggestions")
	cmd.Flags().DurationVar(&flags.timeout, "timeout", 10*time.Second, "Diagnostic timeout duration")

	return cmd
}
