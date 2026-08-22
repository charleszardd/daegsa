package cli

import (
	"fmt"
	"strings"
	"time"

	"github.com/charleszardd/daegsa/internal/clock"
	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/executor"
	"github.com/charleszardd/daegsa/internal/metrics"
	"github.com/charleszardd/daegsa/internal/plan"
	"github.com/charleszardd/daegsa/internal/report"
	"github.com/charleszardd/daegsa/internal/scheduler"
	"github.com/charleszardd/daegsa/internal/threshold"
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
				fmt.Fprint(cmd.OutOrStdout(), plan.FormatPlanSummary(p))
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

			evaluationAggregate := agg
			if agg != nil && agg.Measured != nil {
				evaluationAggregate = agg.Measured
			}

			// Threshold evaluation (§10)
			var thresholdResults []report.ThresholdResult
			allThresholdsPassed := true
			var evalResults []threshold.Result

			if p != nil && len(p.Thresholds) > 0 {
				var evalErr error
				evalResults, allThresholdsPassed, evalErr = threshold.EvaluateWithSteps(
					p.Thresholds,
					evaluationAggregate.ToThresholdSnapshot(),
					evaluationAggregate.ToStepThresholdSnapshots(),
					p.ToEvaluationContext(),
				)
				if evalErr != nil {
					return &CLIExitError{Code: core.ExitCodeRuntimeFailure, Err: fmt.Errorf("threshold evaluation failed: %w", evalErr)}
				}
				thresholdResults = threshold.ToReportResults(evalResults)
			}

			rep := report.BuildReport(p, agg, health, startTime, endTime, incomplete, thresholdResults)

			// Print terminal report (§13)
			fmt.Fprint(cmd.OutOrStdout(), report.FormatTerminalReport(rep, p))

			// Write JSON report if requested (§13)
			if flags.outputJSON != "" {
				if err := report.WriteJSONReport(flags.outputJSON, rep); err != nil {
					return &CLIExitError{Code: core.ExitCodeRuntimeFailure, Err: err}
				}
			}

			// Incomplete run / runtime failure handling (§10, §13)
			if runErr != nil {
				return &CLIExitError{Code: core.ExitCodeRuntimeFailure, Err: runErr}
			}
			if rep.Incomplete {
				return &CLIExitError{Code: core.ExitCodeRuntimeFailure, Err: fmt.Errorf("test execution incomplete (aborted)")}
			}

			// Threshold evaluation pass/fail handling (§10)
			if p != nil && len(p.Thresholds) > 0 {
				if !allThresholdsPassed {
					return &CLIExitError{
						Code: core.ExitCodeThresholdFailure,
						Err:  fmt.Errorf("%s", formatThresholdFailures(evalResults)),
					}
				}
				return nil
			}

			// Default pass/fail handling when no thresholds configured (§10)
			if evaluationAggregate != nil && evaluationAggregate.RequestCounts.Completed > 0 {
				successCount := evaluationAggregate.Outcomes[core.OutcomeSuccess]
				if p.Treat429AsExpected {
					successCount += evaluationAggregate.Outcomes[core.OutcomeRateLimited]
				}
				if successCount < evaluationAggregate.RequestCounts.Completed {
					return &CLIExitError{
						Code: core.ExitCodeThresholdFailure,
						Err:  fmt.Errorf("test completed with %d unexpected errors out of %d requests", evaluationAggregate.RequestCounts.Completed-successCount, evaluationAggregate.RequestCounts.Completed),
					}
				}
			}

			return nil
		},
	}

	addCommonFlags(cmd.Flags(), &flags)
	return cmd
}

func formatThresholdFailures(results []threshold.Result) string {
	var failures []string
	for _, r := range results {
		if !r.Passed {
			failures = append(failures, fmt.Sprintf("%s (%s) failed target %s", r.MetricName, r.ObservedFormatted, r.TargetFormatted))
		}
	}
	if len(failures) == 0 {
		return "one or more thresholds failed"
	}
	return strings.Join(failures, "; ")
}
