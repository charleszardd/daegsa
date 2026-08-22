package cli

import (
	"fmt"
	"time"

	"github.com/charleszardd/daegsa/internal/config"
	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/executor"
	"github.com/charleszardd/daegsa/internal/plan"
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

			// Phase 1 execution: single-request test and classification
			exec, err := executor.NewHTTPExecutor(p)
			if err != nil {
				return &CLIExitError{Code: core.ExitCodeRuntimeFailure, Err: err}
			}
			defer exec.Close()

			res, err := exec.ExecuteRequest(cmd.Context())
			if err != nil {
				return &CLIExitError{Code: core.ExitCodeRuntimeFailure, Err: err}
			}

			printExecutionResult(p, res)

			// Determine test pass/fail
			if res.Outcome == core.OutcomeSuccess || (res.Outcome == core.OutcomeRateLimited && p.Treat429AsExpected) {
				return nil
			}

			return &CLIExitError{
				Code: core.ExitCodeThresholdFailure,
				Err:  fmt.Errorf("request finished with outcome: %s (status: %d)", res.Outcome, res.StatusCode),
			}
		},
	}

	addCommonFlags(cmd.Flags(), &flags)
	return cmd
}

func printExecutionResult(p *plan.Plan, res *executor.Result) {
	fmt.Printf("\n--- DAEGSA Request Execution Result ---\n")
	fmt.Printf("Target:     %s %s\n", p.Method, config.RedactURL(p.TargetURL.String()))
	if res.StatusCode > 0 {
		fmt.Printf("Status:     %d (%s)\n", res.StatusCode, res.Protocol)
	} else {
		fmt.Printf("Status:     <none>\n")
	}
	fmt.Printf("Outcome:    %s\n", res.Outcome)
	fmt.Printf("Latency:    %v (TTFB: %v)\n", res.Latency, res.TTFB)
	fmt.Printf("Bytes:      Sent %d bytes, Received %d bytes\n", res.BytesSent, res.BytesReceived)

	if res.RateLimitInfo != nil {
		if res.RateLimitInfo.RetryAfterSeconds != nil {
			fmt.Printf("RateLimit:  Retry-After = %d seconds\n", *res.RateLimitInfo.RetryAfterSeconds)
		} else if res.RateLimitInfo.RetryAfterDate != nil {
			fmt.Printf("RateLimit:  Retry-After = %s\n", res.RateLimitInfo.RetryAfterDate.Format(time.RFC1123))
		}
		if res.RateLimitInfo.Limit != nil {
			fmt.Printf("RateLimit:  Limit = %d\n", *res.RateLimitInfo.Limit)
		}
		if res.RateLimitInfo.Remaining != nil {
			fmt.Printf("RateLimit:  Remaining = %d\n", *res.RateLimitInfo.Remaining)
		}
		if res.RateLimitInfo.ResetSeconds != nil {
			fmt.Printf("RateLimit:  Reset = %d seconds\n", *res.RateLimitInfo.ResetSeconds)
		}
	}

	if res.Err != nil {
		fmt.Printf("Error:      %v\n", res.Err)
	}
	fmt.Printf("---------------------------------------\n")
}
