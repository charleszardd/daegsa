package report

import (
	"fmt"
	"sort"
	"strings"

	"github.com/charleszardd/daegsa/internal/config"
	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/plan"
)

const (
	dividerLine = "================================================================================"
	sectionLine = "--------------------------------------------------------------------------------"
)

// FormatTerminalReport generates a human-readable, ANSI-formatted console report summary (§13).
func FormatTerminalReport(rep *Report, p *plan.Plan) string {
	if rep == nil {
		return "No report generated\n"
	}

	var sb strings.Builder

	// 1. Header Banner
	sb.WriteString(dividerLine + "\n")
	sb.WriteString("  DAEGSA Load Test Summary\n")
	sb.WriteString(dividerLine + "\n")

	testName := "load-test"
	targetURL := ""
	method := "GET"
	if p != nil {
		if p.Name != "" {
			testName = p.Name
		}
		if p.TargetURL != nil {
			targetURL = config.RedactURL(p.TargetURL.String())
		}
		method = p.Method
	}

	sb.WriteString(fmt.Sprintf("  Test Name:         %s\n", testName))
	if targetURL != "" {
		sb.WriteString(fmt.Sprintf("  Target URL:        %s\n", targetURL))
	}
	sb.WriteString(fmt.Sprintf("  Method:            %s\n", method))

	if p != nil && p.Model == core.WorkloadModelClosed {
		sb.WriteString(fmt.Sprintf("  Workload Model:    closed (Users: %d, Think Time: %s)\n", p.Users, p.ThinkTime))
	} else if p != nil && p.Model == core.WorkloadModelOpen {
		sb.WriteString(fmt.Sprintf("  Workload Model:    open (Rate: %.1f/%s, Max In Flight: %d)\n", p.Rate, p.TimeUnit, p.MaxInFlight))
	} else if rep.WorkloadModel != "" {
		sb.WriteString(fmt.Sprintf("  Workload Model:    %s\n", rep.WorkloadModel))
	}

	if p != nil {
		sb.WriteString(fmt.Sprintf("  Duration:          Planned: %s, Elapsed: %dms\n", p.Duration, rep.DurationMS))
	} else {
		sb.WriteString(fmt.Sprintf("  Duration:          %dms\n", rep.DurationMS))
	}

	if rep.ConfigFingerprint != "" {
		sb.WriteString(fmt.Sprintf("  Fingerprint:       %s\n", rep.ConfigFingerprint))
	}

	// 2. Requests & Throughput
	sb.WriteString("\n" + sectionLine + "\n")
	sb.WriteString("  REQUESTS & THROUGHPUT\n")
	sb.WriteString(sectionLine + "\n")

	sb.WriteString(fmt.Sprintf("  Planned:           %d\n", rep.RequestCounts.Planned))
	sb.WriteString(fmt.Sprintf("  Scheduled:         %d\n", rep.RequestCounts.Scheduled))
	sb.WriteString(fmt.Sprintf("  Started:           %d\n", rep.RequestCounts.Started))
	sb.WriteString(fmt.Sprintf("  Completed:         %d\n", rep.RequestCounts.Completed))
	sb.WriteString(fmt.Sprintf("  Canceled:          %d\n", rep.RequestCounts.Canceled))
	sb.WriteString(fmt.Sprintf("  Dropped:           %d\n", rep.RequestCounts.Dropped))

	durSec := float64(rep.DurationMS) / 1000.0
	var startRPS, throughput float64
	if durSec > 0 {
		startRPS = float64(rep.RequestCounts.Started) / durSec
		throughput = float64(rep.RequestCounts.Completed) / durSec
	}
	sb.WriteString(fmt.Sprintf("  Achieved Start:    %.2f req/s\n", startRPS))
	sb.WriteString(fmt.Sprintf("  Completed Rate:    %.2f req/s\n", throughput))

	// 3. Outcomes
	sb.WriteString("\n" + sectionLine + "\n")
	sb.WriteString("  OUTCOMES\n")
	sb.WriteString(sectionLine + "\n")
	sb.WriteString(fmt.Sprintf("  %-30s %10s %12s\n", "Outcome", "Count", "Percent"))

	totalCompleted := rep.RequestCounts.Completed
	if totalCompleted == 0 {
		totalCompleted = rep.RequestCounts.Started
	}

	for _, o := range core.AllOutcomes {
		count := rep.Outcomes[o]
		var pct float64
		if totalCompleted > 0 {
			pct = (float64(count) / float64(totalCompleted)) * 100.0
		}
		if count > 0 || o == core.OutcomeSuccess {
			sb.WriteString(fmt.Sprintf("  %-30s %10d %11.2f%%\n", o, count, pct))
		}
	}

	// 4. Status Codes
	if len(rep.StatusCodes) > 0 {
		sb.WriteString("\n" + sectionLine + "\n")
		sb.WriteString("  HTTP STATUS CODES\n")
		sb.WriteString(sectionLine + "\n")
		sb.WriteString(fmt.Sprintf("  %-30s %10s %12s\n", "Status Code", "Count", "Percent"))

		// Sort status codes numerically
		keys := make([]string, 0, len(rep.StatusCodes))
		for k := range rep.StatusCodes {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		for _, codeStr := range keys {
			count := rep.StatusCodes[codeStr]
			var pct float64
			if totalCompleted > 0 {
				pct = (float64(count) / float64(totalCompleted)) * 100.0
			}
			sb.WriteString(fmt.Sprintf("  %-30s %10d %11.2f%%\n", codeStr, count, pct))
		}
	}

	// 5. Latency Distribution
	sb.WriteString("\n" + sectionLine + "\n")
	sb.WriteString("  LATENCY DISTRIBUTION (ms)\n")
	sb.WriteString(sectionLine + "\n")
	sb.WriteString(fmt.Sprintf("  %-16s %20s %22s\n", "Metric", "All Completed", "Expected Success"))
	sb.WriteString(fmt.Sprintf("  %-16s %17.2f ms %19.2f ms\n", "Min", rep.Latency.AllCompleted.MinMS, rep.Latency.ExpectedSuccess.MinMS))
	sb.WriteString(fmt.Sprintf("  %-16s %17.2f ms %19.2f ms\n", "p50", rep.Latency.AllCompleted.P50MS, rep.Latency.ExpectedSuccess.P50MS))
	sb.WriteString(fmt.Sprintf("  %-16s %17.2f ms %19.2f ms\n", "p90", rep.Latency.AllCompleted.P90MS, rep.Latency.ExpectedSuccess.P90MS))
	sb.WriteString(fmt.Sprintf("  %-16s %17.2f ms %19.2f ms\n", "p95", rep.Latency.AllCompleted.P95MS, rep.Latency.ExpectedSuccess.P95MS))
	sb.WriteString(fmt.Sprintf("  %-16s %17.2f ms %19.2f ms\n", "p99", rep.Latency.AllCompleted.P99MS, rep.Latency.ExpectedSuccess.P99MS))
	sb.WriteString(fmt.Sprintf("  %-16s %17.2f ms %19.2f ms\n", "Max", rep.Latency.AllCompleted.MaxMS, rep.Latency.ExpectedSuccess.MaxMS))
	sb.WriteString(fmt.Sprintf("  %-16s %17.2f ms %19.2f ms\n", "Mean", rep.Latency.AllCompleted.MeanMS, rep.Latency.ExpectedSuccess.MeanMS))

	// 6. Rate Limiting Observations
	if rep.RateLimits.Observed429Count > 0 || len(rep.RateLimits.RetryAfterSamples) > 0 || len(rep.RateLimits.RateLimitHeaders) > 0 {
		sb.WriteString("\n" + sectionLine + "\n")
		sb.WriteString("  RATE LIMITING OBSERVATIONS\n")
		sb.WriteString(sectionLine + "\n")
		sb.WriteString(fmt.Sprintf("  Observed 429 Count: %d\n", rep.RateLimits.Observed429Count))
		if len(rep.RateLimits.RetryAfterSamples) > 0 {
			sb.WriteString(fmt.Sprintf("  Retry-After Samples: %s\n", strings.Join(rep.RateLimits.RetryAfterSamples, ", ")))
		}
		for i, h := range rep.RateLimits.RateLimitHeaders {
			sb.WriteString(fmt.Sprintf("  Header Sample #%d:   Limit=%s, Remaining=%s, Reset=%s, Policy=%s\n",
				i+1, h.Limit, h.Remaining, h.Reset, h.Policy))
		}
	}

	// 7. Generator Health
	sb.WriteString("\n" + sectionLine + "\n")
	sb.WriteString("  GENERATOR HEALTH\n")
	sb.WriteString(sectionLine + "\n")
	sb.WriteString(fmt.Sprintf("  Peak Goroutines:   %d\n", rep.GeneratorHealth.GoroutinesPeak))
	sb.WriteString(fmt.Sprintf("  Max CPU:           %.1f%%\n", rep.GeneratorHealth.CPUMaxPercent))
	sb.WriteString(fmt.Sprintf("  Max Scheduler Lag: %.2f ms\n", rep.GeneratorHealth.SchedulerLagMaxMS))
	if len(rep.GeneratorHealth.SaturationWarnings) > 0 {
		sb.WriteString(fmt.Sprintf("  Warnings:          %s\n", strings.Join(rep.GeneratorHealth.SaturationWarnings, "; ")))
	}

	// 8. Test Result Banner
	sb.WriteString("\n" + dividerLine + "\n")
	resultBanner := "PASS"
	if rep.Incomplete {
		resultBanner = "INCOMPLETE (run aborted or timed out)"
	} else if rep.RequestCounts.Completed > 0 && rep.Outcomes[core.OutcomeSuccess] < rep.RequestCounts.Completed {
		resultBanner = "FAIL (unexpected status codes or errors detected)"
	}
	sb.WriteString(fmt.Sprintf("  TEST RESULT: %s\n", resultBanner))
	sb.WriteString(dividerLine + "\n")

	return sb.String()
}
