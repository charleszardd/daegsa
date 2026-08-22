package cli

import (
	"fmt"
	"time"

	"github.com/charleszardd/daegsa/internal/clock"
	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/executor"
	"github.com/charleszardd/daegsa/internal/metrics"
	"github.com/charleszardd/daegsa/internal/plan"
	"github.com/charleszardd/daegsa/internal/report"
	"github.com/charleszardd/daegsa/internal/scheduler"
	"github.com/spf13/cobra"
)

func newRunCmd() *cobra.Command {
	var flags flagValues

	cmd := &cobra.Command{
		Use:   "run",
		Short: "Execute load, rate-limit, or capacity test",
		Long:  "Execute load, rate-limit, or capacity test against target HTTP endpoint.",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, _, p, err := loadAndPreflightConfig(cmd.Context(), &flags)
			if err != nil {
				return err
			}

			// Check --dry-run
			if flags.dryRun {
				fmt.Print(plan.FormatPlanSummary(p))
				return nil
			}

			exec, err := executor.NewHTTPExecutor(p)
			if err != nil {
				return &CLIExitError{Code: core.ExitCodeRuntimeFailure, Err: err}
			}
			defer exec.Close()

			var agg *metrics.AggregatedMetrics
			var health *metrics.GeneratorHealth
			var runErr error

			startTime := time.Now().UTC()

			if p.Model == core.WorkloadModelClosed {
				sched, err := scheduler.NewClosedScheduler(p, exec, clock.NewRealClock())
				if err != nil {
					return &CLIExitError{Code: core.ExitCodeRuntimeFailure, Err: err}
				}
				agg, health, runErr = sched.Run(cmd.Context())
			} else if p.Model == core.WorkloadModelOpen {
				sched, err := scheduler.NewOpenScheduler(p, exec, clock.NewRealClock())
				if err != nil {
					return &CLIExitError{Code: core.ExitCodeRuntimeFailure, Err: err}
				}
				agg, health, runErr = sched.Run(cmd.Context())
			} else {
				return &CLIExitError{Code: core.ExitCodeRuntimeFailure, Err: fmt.Errorf("unsupported workload model: %s", p.Model)}
			}

			endTime := time.Now().UTC()
			incomplete := (runErr != nil) || (cmd.Context().Err() != nil)

			rep := report.BuildReport(p, agg, health, startTime, endTime, incomplete)

			// Print terminal report
			fmt.Print(report.FormatTerminalReport(rep, p))

			// Write JSON report if requested
			if flags.outputJSON != "" {
				if err := report.WriteJSONReport(flags.outputJSON, rep); err != nil {
					return &CLIExitError{Code: core.ExitCodeRuntimeFailure, Err: err}
				}
			}

			if runErr != nil {
				return &CLIExitError{Code: core.ExitCodeRuntimeFailure, Err: runErr}
			}

			if rep.Incomplete {
				return &CLIExitError{Code: core.ExitCodeRuntimeFailure, Err: fmt.Errorf("test execution incomplete")}
			}

			// Determine test pass/fail
			if rep.RequestCounts.Completed > 0 {
				successCount := rep.Outcomes[core.OutcomeSuccess]
				if p.Treat429AsExpected {
					successCount += rep.Outcomes[core.OutcomeRateLimited]
				}
				if successCount < rep.RequestCounts.Completed {
					return &CLIExitError{
						Code: core.ExitCodeThresholdFailure,
						Err:  fmt.Errorf("test completed with %d failures out of %d requests", rep.RequestCounts.Completed-successCount, rep.RequestCounts.Completed),
					}
				}
			}

			return nil
		},
	}

	addCommonFlags(cmd.Flags(), &flags)
	return cmd
}
