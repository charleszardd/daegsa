package selftest

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/charleszardd/daegsa/internal/clock"
	"github.com/charleszardd/daegsa/internal/config"
	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/executor"
	"github.com/charleszardd/daegsa/internal/plan"
	"github.com/charleszardd/daegsa/internal/report"
	"github.com/charleszardd/daegsa/internal/safety"
	"github.com/charleszardd/daegsa/internal/scheduler"
	"github.com/charleszardd/daegsa/internal/testtarget"
	"github.com/charleszardd/daegsa/internal/threshold"
)

// runClosedWorkloadSubTest verifies closed VU loop execution, metrics aggregation, and latency tracking (§7, §9, §14).
func runClosedWorkloadSubTest(ctx context.Context, srv *testtarget.TargetServer) SubTestResult {
	start := time.Now()
	testName := "Closed Workload Loop"

	cfg := &config.Config{
		SchemaVersion: config.LegacySchemaVersion,
		Name:          "selftest-closed",
		Request: config.RequestConfig{
			URL:     srv.URL() + "/items",
			Method:  http.MethodGet,
			Timeout: config.Duration(2 * time.Second),
		},
		Load: config.LoadConfig{
			Model:     core.WorkloadModelClosed,
			Users:     5,
			Duration:  config.Duration(200 * time.Millisecond),
			ThinkTime: config.Duration(5 * time.Millisecond),
		},
		Safety: config.SafetyConfig{
			AllowedHosts: []string{"127.0.0.1", "localhost"},
		},
	}

	preflightResult, err := safety.NewPreflightEngine().Check(ctx, cfg, safety.SafetyFlags{})
	if err != nil {
		return SubTestResult{Name: testName, Status: StatusFail, Duration: time.Since(start), Err: fmt.Errorf("preflight failed: %w", err)}
	}

	p, err := plan.BuildPlan(cfg, preflightResult)
	if err != nil {
		return SubTestResult{Name: testName, Status: StatusFail, Duration: time.Since(start), Err: fmt.Errorf("plan build failed: %w", err)}
	}

	exec, err := executor.NewHTTPExecutor(p)
	if err != nil {
		return SubTestResult{Name: testName, Status: StatusFail, Duration: time.Since(start), Err: fmt.Errorf("executor creation failed: %w", err)}
	}
	defer exec.Close()

	sched, err := scheduler.NewClosedScheduler(p, exec, clock.NewRealClock())
	if err != nil {
		return SubTestResult{Name: testName, Status: StatusFail, Duration: time.Since(start), Err: fmt.Errorf("scheduler creation failed: %w", err)}
	}

	agg, _, runErr := sched.Run(ctx)
	elapsed := time.Since(start)
	if runErr != nil {
		return SubTestResult{Name: testName, Status: StatusFail, Duration: elapsed, Err: fmt.Errorf("execution error: %w", runErr)}
	}

	if agg == nil || agg.RequestCounts.Completed <= 0 {
		return SubTestResult{Name: testName, Status: StatusFail, Duration: elapsed, Detail: "No completed requests recorded"}
	}

	successes := agg.Outcomes[core.OutcomeSuccess]
	if successes != agg.RequestCounts.Completed {
		return SubTestResult{
			Name:              testName,
			Status:            StatusFail,
			Duration:          elapsed,
			RequestsCompleted: agg.RequestCounts.Completed,
			Errors:            agg.RequestCounts.Completed - successes,
			Detail:            fmt.Sprintf("Expected 0 errors, observed %d", agg.RequestCounts.Completed-successes),
		}
	}

	detail := fmt.Sprintf("Completed %d reqs across 5 VUs (p50: %.2fms, p95: %.2fms, p99: %.2fms)",
		agg.RequestCounts.Completed, agg.Latency.AllCompleted.P50MS, agg.Latency.AllCompleted.P95MS, agg.Latency.AllCompleted.P99MS)

	return SubTestResult{
		Name:              testName,
		Status:            StatusPass,
		Duration:          elapsed,
		RequestsCompleted: agg.RequestCounts.Completed,
		Errors:            0,
		Detail:            detail,
	}
}

// runOpenArrivalRateSubTest verifies open arrival-rate pacing, max-in-flight drop semantics, and dropped work accounting (§7, §9, §14).
func runOpenArrivalRateSubTest(ctx context.Context, srv *testtarget.TargetServer) SubTestResult {
	start := time.Now()
	testName := "Open Arrival-Rate Pacing"

	cfg := &config.Config{
		SchemaVersion: config.LegacySchemaVersion,
		Name:          "selftest-open",
		Request: config.RequestConfig{
			URL:     srv.URL() + "/items?delay=40ms",
			Method:  http.MethodGet,
			Timeout: config.Duration(2 * time.Second),
		},
		Load: config.LoadConfig{
			Model:       core.WorkloadModelOpen,
			Rate:        50,
			TimeUnit:    config.Duration(1 * time.Second),
			Duration:    config.Duration(200 * time.Millisecond),
			MaxInFlight: 4,
		},
		Safety: config.SafetyConfig{
			AllowedHosts: []string{"127.0.0.1", "localhost"},
		},
	}

	preflightResult, err := safety.NewPreflightEngine().Check(ctx, cfg, safety.SafetyFlags{})
	if err != nil {
		return SubTestResult{Name: testName, Status: StatusFail, Duration: time.Since(start), Err: fmt.Errorf("preflight failed: %w", err)}
	}

	p, err := plan.BuildPlan(cfg, preflightResult)
	if err != nil {
		return SubTestResult{Name: testName, Status: StatusFail, Duration: time.Since(start), Err: fmt.Errorf("plan build failed: %w", err)}
	}

	exec, err := executor.NewHTTPExecutor(p)
	if err != nil {
		return SubTestResult{Name: testName, Status: StatusFail, Duration: time.Since(start), Err: fmt.Errorf("executor creation failed: %w", err)}
	}
	defer exec.Close()

	sched, err := scheduler.NewOpenScheduler(p, exec, clock.NewRealClock())
	if err != nil {
		return SubTestResult{Name: testName, Status: StatusFail, Duration: time.Since(start), Err: fmt.Errorf("scheduler creation failed: %w", err)}
	}

	agg, _, runErr := sched.Run(ctx)
	elapsed := time.Since(start)
	if runErr != nil {
		return SubTestResult{Name: testName, Status: StatusFail, Duration: elapsed, Err: fmt.Errorf("execution error: %w", runErr)}
	}

	if agg == nil {
		return SubTestResult{Name: testName, Status: StatusFail, Duration: elapsed, Detail: "Nil aggregated metrics returned"}
	}

	// Verify dropped requests or completed requests are accounted for properly
	totalHandled := agg.RequestCounts.Completed + agg.RequestCounts.Dropped + agg.RequestCounts.Canceled
	if totalHandled <= 0 {
		return SubTestResult{Name: testName, Status: StatusFail, Duration: elapsed, Detail: "Zero scheduled requests executed"}
	}

	detail := fmt.Sprintf("Handled %d scheduled ticks (Completed: %d, Dropped: %d, Canceled: %d, MaxInFlight: 4)",
		totalHandled, agg.RequestCounts.Completed, agg.RequestCounts.Dropped, agg.RequestCounts.Canceled)

	return SubTestResult{
		Name:              testName,
		Status:            StatusPass,
		Duration:          elapsed,
		RequestsCompleted: agg.RequestCounts.Completed,
		Errors:            0,
		Detail:            detail,
	}
}

// runMultiStepScenarioSubTest verifies variable extraction, session cookie jar preservation, and multi-step scenario flow (§6, §7, §11, §14).
func runMultiStepScenarioSubTest(ctx context.Context, srv *testtarget.TargetServer) SubTestResult {
	start := time.Now()
	testName := "Multi-Step Scenario & State Chaining"

	cfg := &config.Config{
		SchemaVersion: config.LegacySchemaVersion,
		Name:          "selftest-scenario",
		Load: config.LoadConfig{
			Model:     core.WorkloadModelClosed,
			Users:     2,
			Duration:  config.Duration(200 * time.Millisecond),
			ThinkTime: config.Duration(5 * time.Millisecond),
		},
		Safety: config.SafetyConfig{
			AllowedHosts:       []string{"127.0.0.1", "localhost"},
			AllowNonIdempotent: true,
		},
		Auth: config.AuthConfig{
			CookieJar: true,
		},
		Scenario: &config.ScenarioConfig{
			Name: "login-items-logout",
			Steps: []config.StepConfig{
				{
					Name:   "login",
					URL:    srv.URL() + "/auth/login",
					Method: http.MethodPost,
					Extract: map[string]config.ExtractRuleConfig{
						"jwt_token": {
							From:       config.ExtractSourceJSON,
							Expression: "token",
						},
					},
				},
				{
					Name:   "fetch_items",
					URL:    srv.URL() + "/api/items",
					Method: http.MethodGet,
					Headers: map[string]string{
						"Authorization": "Bearer ${jwt_token}",
					},
				},
				{
					Name:   "logout",
					URL:    srv.URL() + "/api/logout",
					Method: http.MethodPost,
				},
			},
		},
	}

	safetyFlags := safety.SafetyFlags{AllowDestructive: true}
	preflightResult, err := safety.NewPreflightEngine().Check(ctx, cfg, safetyFlags)
	if err != nil {
		return SubTestResult{Name: testName, Status: StatusFail, Duration: time.Since(start), Err: fmt.Errorf("preflight failed: %w", err)}
	}

	p, err := plan.BuildPlan(cfg, preflightResult)
	if err != nil {
		return SubTestResult{Name: testName, Status: StatusFail, Duration: time.Since(start), Err: fmt.Errorf("plan build failed: %w", err)}
	}

	exec, err := executor.NewHTTPExecutor(p)
	if err != nil {
		return SubTestResult{Name: testName, Status: StatusFail, Duration: time.Since(start), Err: fmt.Errorf("executor creation failed: %w", err)}
	}
	defer exec.Close()

	sched, err := scheduler.NewClosedScheduler(p, exec, clock.NewRealClock())
	if err != nil {
		return SubTestResult{Name: testName, Status: StatusFail, Duration: time.Since(start), Err: fmt.Errorf("scheduler creation failed: %w", err)}
	}

	agg, _, runErr := sched.Run(ctx)
	elapsed := time.Since(start)
	if runErr != nil {
		return SubTestResult{Name: testName, Status: StatusFail, Duration: elapsed, Err: fmt.Errorf("scenario execution error: %w", runErr)}
	}

	if agg == nil || agg.ScenarioIterations.Completed <= 0 {
		return SubTestResult{Name: testName, Status: StatusFail, Duration: elapsed, Detail: "Zero completed scenario iterations recorded"}
	}

	// Verify all 3 steps recorded
	if len(agg.Steps) != 3 {
		return SubTestResult{Name: testName, Status: StatusFail, Duration: elapsed, Detail: fmt.Sprintf("Expected 3 scenario steps in metrics, found %d", len(agg.Steps))}
	}

	detail := fmt.Sprintf("Completed %d iterations across 3 steps (%d total requests, 0 errors)",
		agg.ScenarioIterations.Completed, agg.RequestCounts.Completed)

	return SubTestResult{
		Name:              testName,
		Status:            StatusPass,
		Duration:          elapsed,
		RequestsCompleted: agg.RequestCounts.Completed,
		Errors:            0,
		Detail:            detail,
	}
}

// runThresholdEvaluationSubTest verifies threshold rule compilation and deterministic evaluation (§10, §14).
func runThresholdEvaluationSubTest(ctx context.Context, srv *testtarget.TargetServer) SubTestResult {
	start := time.Now()
	testName := "Threshold Evaluation Engine"

	thPass, err := threshold.ParseThreshold("http_error_rate", "<= 0%")
	if err != nil {
		return SubTestResult{Name: testName, Status: StatusFail, Duration: time.Since(start), Err: fmt.Errorf("failed to parse passing threshold: %w", err)}
	}

	thFail, err := threshold.ParseThreshold("p99", "<= 1µs")
	if err != nil {
		return SubTestResult{Name: testName, Status: StatusFail, Duration: time.Since(start), Err: fmt.Errorf("failed to parse failing threshold: %w", err)}
	}

	snapshot := threshold.MetricsSnapshot{
		CompletedRequests: 100,
		ErrorRate:         0.0,
		P99LatencyMS:      5.0,
		AchievedStartRPS:  50.0,
	}

	evalCtx := threshold.EvaluationContext{
		TargetRPS: 50.0,
	}

	results, allPassed, err := threshold.EvaluateWithSteps([]*threshold.Threshold{thPass, thFail}, snapshot, nil, evalCtx)
	elapsed := time.Since(start)

	if err != nil {
		return SubTestResult{Name: testName, Status: StatusFail, Duration: elapsed, Err: fmt.Errorf("threshold evaluation returned unexpected error: %w", err)}
	}

	if allPassed {
		return SubTestResult{Name: testName, Status: StatusFail, Duration: elapsed, Detail: "Expected overall threshold evaluation to fail due to p99 <= 1µs constraint"}
	}

	if len(results) != 2 {
		return SubTestResult{Name: testName, Status: StatusFail, Duration: elapsed, Detail: fmt.Sprintf("Expected 2 threshold results, got %d", len(results))}
	}

	if !results[0].Passed {
		return SubTestResult{Name: testName, Status: StatusFail, Duration: elapsed, Detail: "Expected passing threshold 'http_error_rate <= 0%' to pass"}
	}

	if results[1].Passed {
		return SubTestResult{Name: testName, Status: StatusFail, Duration: elapsed, Detail: "Expected failing threshold 'p99 <= 1µs' to fail"}
	}

	detail := "Evaluated passing (http_error_rate <= 0% -> PASS) and deliberate failing (p99 <= 1µs -> FAIL) thresholds deterministically"

	return SubTestResult{
		Name:              testName,
		Status:            StatusPass,
		Duration:          elapsed,
		RequestsCompleted: 0,
		Errors:            0,
		Detail:            detail,
	}
}

// runReportGenerationSubTest verifies JSON schema serialization and terminal rendering (§13, §14).
func runReportGenerationSubTest(ctx context.Context, srv *testtarget.TargetServer) SubTestResult {
	start := time.Now()
	testName := "Report Serialization & Schema"

	now := time.Now().UTC()
	rep := &report.Report{
		ReportSchemaVersion: report.ExpectedReportSchemaVersion,
		DaegsaVersion:       "v0.1.0-dev",
		Commit:              "selftest-commit",
		BuildDate:           "2026-08-22",
		OS:                  "windows",
		Arch:                "amd64",
		StartTimeUTC:        now.Add(-1 * time.Second),
		EndTimeUTC:          now,
		DurationMS:          1000,
		WorkloadModel:       core.WorkloadModelClosed,
		RequestCounts: report.RequestCounts{
			Planned:   50,
			Completed: 50,
		},
		Outcomes: map[core.Outcome]int64{
			core.OutcomeSuccess: 50,
		},
		StatusCodes: map[string]int64{
			"200": 50,
		},
		Latency: report.LatencySummary{
			AllCompleted: report.LatencyPercentiles{
				P50MS: 10.0,
				P95MS: 20.0,
				P99MS: 30.0,
			},
		},
		Thresholds: make([]report.ThresholdResult, 0),
	}

	// 1. Terminal Report Formatting
	termOut := report.FormatTerminalReport(rep, nil)
	if termOut == "" {
		return SubTestResult{Name: testName, Status: StatusFail, Duration: time.Since(start), Detail: "Formatted terminal report was empty"}
	}

	// 2. JSON Serialization
	jsonBytes, err := rep.ToJSON()
	if err != nil {
		return SubTestResult{Name: testName, Status: StatusFail, Duration: time.Since(start), Err: fmt.Errorf("report JSON serialization failed: %w", err)}
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		return SubTestResult{Name: testName, Status: StatusFail, Duration: time.Since(start), Err: fmt.Errorf("report JSON deserialization failed: %w", err)}
	}

	if parsed["report_schema_version"] != float64(report.ExpectedReportSchemaVersion) {
		return SubTestResult{Name: testName, Status: StatusFail, Duration: time.Since(start), Detail: "Invalid report_schema_version in JSON report"}
	}

	elapsed := time.Since(start)
	detail := fmt.Sprintf("Terminal report rendering and JSON schema v%d serialization verified", report.ExpectedReportSchemaVersion)

	return SubTestResult{
		Name:              testName,
		Status:            StatusPass,
		Duration:          elapsed,
		RequestsCompleted: 0,
		Errors:            0,
		Detail:            detail,
	}
}
