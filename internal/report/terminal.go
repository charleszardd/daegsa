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
		var targetRPS float64
		if p.TimeUnit > 0 {
			targetRPS = p.Rate / p.TimeUnit.Seconds()
		}
		sb.WriteString(fmt.Sprintf("  Workload Model:    open (Target Rate: %.2f req/s, Rate: %.1f/%s, Max In-Flight: %d)\n", targetRPS, p.Rate, p.TimeUnit, p.MaxInFlight))
	} else if rep.WorkloadModel != "" {
		sb.WriteString(fmt.Sprintf("  Workload Model:    %s\n", rep.WorkloadModel))
	}

	if p != nil && p.Authenticator != nil && p.Authenticator.AuthMode() != "none" {
		cookieNote := ""
		if p.CookieJarEnabled {
			cookieNote = ", cookie jar enabled"
		}
		sb.WriteString(fmt.Sprintf("  Auth:              %s (%d token(s)%s)\n", p.Authenticator.AuthMode(), p.Authenticator.TokenCount(), cookieNote))
	} else if p != nil && p.CookieJarEnabled {
		sb.WriteString("  Auth:              none (cookie jar enabled)\n")
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

	// 5b. Scenario Steps
	if rep.Scenario != nil && len(rep.Scenario.Steps) > 0 {
		sb.WriteString("\n" + sectionLine + "\n")
		sb.WriteString(fmt.Sprintf("  SCENARIO: %s (Planned: %d, Completed: %d, Failed: %d)\n",
			rep.Scenario.Name, rep.Scenario.Iterations.Planned, rep.Scenario.Iterations.Completed, rep.Scenario.Iterations.Failed))
		sb.WriteString(sectionLine + "\n")
		sb.WriteString(fmt.Sprintf("  %-16s %-6s %10s %10s %10s %10s %10s %10s\n",
			"Step", "Method", "Completed", "Err Rate", "p50 (ms)", "p95 (ms)", "p99 (ms)", "Throughput"))

		for _, step := range rep.Scenario.Steps {
			sb.WriteString(fmt.Sprintf("  %-16s %-6s %10d %9.2f%% %10.2f %10.2f %10.2f %9.2f/s\n",
				step.Name,
				step.Method,
				step.RequestCounts.Completed,
				step.ErrorRate,
				step.Latency.AllCompleted.P50MS,
				step.Latency.AllCompleted.P95MS,
				step.Latency.AllCompleted.P99MS,
				step.CompletedThroughput,
			))
		}
	}

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

	if rep.ReportSchemaVersion >= ProfileReportSchemaVersion {
		sb.WriteString("\n" + sectionLine + "\n  PROFILE SEGMENTS\n" + sectionLine + "\n")
		for _, segment := range rep.Segments {
			sb.WriteString(fmt.Sprintf("  %s [%s]: target %.2f req/s, started %d, completed %d, 429 %d, reliable=%v\n", segment.Segment.Name, segment.Segment.Stage, segment.Segment.TargetRPS, segment.Metrics.RequestCounts.Started, segment.Metrics.RequestCounts.Completed, segment.Metrics.RateLimits.Observed429Count, segment.Calibration.Reliable))
		}
		if rep.MeasuredSummary != nil {
			sb.WriteString(fmt.Sprintf("  Measured Only: started %d, completed %d, p95 %.2f ms\n", rep.MeasuredSummary.RequestCounts.Started, rep.MeasuredSummary.RequestCounts.Completed, rep.MeasuredSummary.Latency.AllCompleted.P95MS))
		}
		if rep.RateLimits.Observed429Count == 0 {
			sb.WriteString("  No throttling observed at tested rates; this is not a guaranteed safe production limit.\n")
		}
	}

	// 7. Generator Health
	sb.WriteString("\n" + sectionLine + "\n")
	sb.WriteString("  GENERATOR HEALTH\n")
	sb.WriteString(sectionLine + "\n")
	sb.WriteString(fmt.Sprintf("  Peak Goroutines:   %d\n", rep.GeneratorHealth.GoroutinesPeak))
	if rep.ReportSchemaVersion >= ProfileReportSchemaVersion && !rep.GeneratorHealth.CPUAvailable {
		sb.WriteString("  Max CPU:           unavailable\n")
	} else {
		sb.WriteString(fmt.Sprintf("  Max CPU:           %.1f%%\n", rep.GeneratorHealth.CPUMaxPercent))
	}
	sb.WriteString(fmt.Sprintf("  Max Scheduler Lag: %.2f ms\n", rep.GeneratorHealth.SchedulerLagMaxMS))
	if len(rep.GeneratorHealth.SaturationWarnings) > 0 {
		sb.WriteString(fmt.Sprintf("  Warnings:          %s\n", strings.Join(rep.GeneratorHealth.SaturationWarnings, "; ")))
	}

	// 8. Threshold Evaluation
	if len(rep.Thresholds) > 0 {
		sb.WriteString("\n" + sectionLine + "\n")
		sb.WriteString("  THRESHOLD EVALUATION\n")
		sb.WriteString(sectionLine + "\n")
		sb.WriteString(fmt.Sprintf("  %-30s %-16s %-16s %-8s\n", "Metric", "Target", "Observed", "Status"))
		for _, tr := range rep.Thresholds {
			status := "PASS"
			if !tr.Passed {
				status = "FAIL"
			}
			metricName := tr.Expression
			if idx := strings.Index(tr.Expression, " "); idx != -1 {
				metricName = tr.Expression[:idx]
			}
			sb.WriteString(fmt.Sprintf("  %-30s %-16s %-16s %-8s\n", metricName, tr.Target, tr.Observed, status))
		}
	}

	// 9. Test Result Banner
	sb.WriteString("\n" + dividerLine + "\n")
	resultBanner := "PASS"
	if rep.Incomplete {
		resultBanner = "INCOMPLETE (run aborted or timed out)"
	} else if len(rep.Thresholds) > 0 {
		anyFailed := false
		for _, tr := range rep.Thresholds {
			if !tr.Passed {
				anyFailed = true
				break
			}
		}
		if anyFailed {
			resultBanner = "FAIL (thresholds failed)"
		} else {
			resultBanner = "PASS"
		}
	} else {
		evaluationCounts, evaluationOutcomes := rep.RequestCounts, rep.Outcomes
		if rep.MeasuredSummary != nil {
			evaluationCounts, evaluationOutcomes = rep.MeasuredSummary.RequestCounts, rep.MeasuredSummary.Outcomes
		}
		if evaluationCounts.Completed > 0 {
			successCount := evaluationOutcomes[core.OutcomeSuccess]
			if p != nil && p.Treat429AsExpected {
				successCount += evaluationOutcomes[core.OutcomeRateLimited]
			}
			if successCount < evaluationCounts.Completed {
				resultBanner = "FAIL (unexpected status codes or errors detected)"
			}
		}
	}
	sb.WriteString(fmt.Sprintf("  TEST RESULT: %s\n", resultBanner))
	sb.WriteString(dividerLine + "\n")

	return sb.String()
}
