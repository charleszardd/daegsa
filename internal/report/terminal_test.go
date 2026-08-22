package report

import (
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/charleszardd/daegsa/internal/auth"
	"github.com/charleszardd/daegsa/internal/config"
	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/metrics"
	"github.com/charleszardd/daegsa/internal/plan"
)

func TestFormatTerminalReport_AllSections(t *testing.T) {
	parsedURL, _ := url.Parse("https://user:secret@api.example.com/items?token=secret123")
	p := &plan.Plan{
		Name:         "test-terminal-summary",
		TargetURL:    parsedURL,
		Method:       "GET",
		Model:        core.WorkloadModelClosed,
		Users:        10,
		ThinkTime:    50 * time.Millisecond,
		Duration:     10 * time.Second,
		GracefulStop: 5 * time.Second,
		Fingerprint:  "sha256:abcd1234efgh5678",
	}

	rep := &Report{
		ReportSchemaVersion: 1,
		DurationMS:          10000,
		WorkloadModel:       core.WorkloadModelClosed,
		ConfigFingerprint:   p.Fingerprint,
		RequestCounts: RequestCounts{
			Planned:   1000,
			Scheduled: 1000,
			Started:   1000,
			Completed: 980,
			Canceled:  20,
			Dropped:   0,
		},
		Outcomes: map[core.Outcome]int64{
			core.OutcomeSuccess:          950,
			core.OutcomeUnexpectedStatus: 30,
			core.OutcomeCanceled:         20,
		},
		StatusCodes: map[string]int64{
			"200": 950,
			"500": 30,
		},
		Latency: LatencySummary{
			AllCompleted: LatencyPercentiles{
				MinMS:  1.5,
				P50MS:  10.0,
				P90MS:  25.0,
				P95MS:  35.0,
				P99MS:  50.0,
				MaxMS:  120.0,
				MeanMS: 12.5,
			},
			ExpectedSuccess: LatencyPercentiles{
				MinMS:  1.5,
				P50MS:  9.5,
				P90MS:  22.0,
				P95MS:  30.0,
				P99MS:  45.0,
				MaxMS:  80.0,
				MeanMS: 11.2,
			},
		},
		RateLimits: RateLimitObservations{
			Observed429Count:  2,
			RetryAfterSamples: []string{"30s"},
			RateLimitHeaders: []RateLimitHeaderSample{
				{Limit: "100", Remaining: "0", Reset: "30s", Policy: "standard"},
			},
		},
		GeneratorHealth: GeneratorHealth{
			GoroutinesPeak:     42,
			CPUMaxPercent:      89.5,
			SchedulerLagMaxMS:  55.0,
			SaturationWarnings: []string{"client CPU saturation detected (> 85%)", "scheduler lag exceeded 50ms"},
		},
		Incomplete: false,
	}

	output := FormatTerminalReport(rep, p)

	// Verify Header sections
	if !strings.Contains(output, "DAEGSA Load Test Summary") {
		t.Errorf("missing header title in terminal report")
	}
	if !strings.Contains(output, "test-terminal-summary") {
		t.Errorf("missing test name in terminal report")
	}

	// Verify URL redaction
	if strings.Contains(output, "secret") || strings.Contains(output, "secret123") {
		t.Errorf("sensitive credentials leaked in terminal output: %s", output)
	}
	if !strings.Contains(output, "REDACTED") {
		t.Errorf("expected REDACTED placeholder in terminal report URL: %s", output)
	}

	// Verify Request counts
	if !strings.Contains(output, "Planned:           1000") {
		t.Errorf("missing planned count in output")
	}
	if !strings.Contains(output, "Completed:         980") {
		t.Errorf("missing completed count in output")
	}

	// Verify Outcomes
	if !strings.Contains(output, "success") || !strings.Contains(output, "unexpected_status") {
		t.Errorf("missing outcome rows in output")
	}

	// Verify Status Codes
	if !strings.Contains(output, "200") || !strings.Contains(output, "500") {
		t.Errorf("missing status codes in output")
	}

	// Verify Latency rows
	if !strings.Contains(output, "p50") || !strings.Contains(output, "p99") {
		t.Errorf("missing latency percentiles in output")
	}

	// Verify Rate Limiting
	if !strings.Contains(output, "Observed 429 Count: 2") {
		t.Errorf("missing 429 count in output")
	}

	// Verify Warnings
	if !strings.Contains(output, "client CPU saturation detected") {
		t.Errorf("missing saturation warning in output")
	}

	// Verify Result Banner (FAIL because 30 errors)
	if !strings.Contains(output, "TEST RESULT: FAIL") {
		t.Errorf("expected FAIL banner, got output: %s", output)
	}
}

func TestFormatTerminalReport_PassBanner(t *testing.T) {
	p := &plan.Plan{
		Name:     "successful-run",
		Model:    core.WorkloadModelClosed,
		Users:    2,
		Duration: 1 * time.Second,
	}

	rep := &Report{
		ReportSchemaVersion: 1,
		DurationMS:          1000,
		RequestCounts: RequestCounts{
			Planned:   100,
			Started:   100,
			Completed: 100,
		},
		Outcomes: map[core.Outcome]int64{
			core.OutcomeSuccess: 100,
		},
		StatusCodes: map[string]int64{
			"200": 100,
		},
		Incomplete: false,
	}

	output := FormatTerminalReport(rep, p)

	if !strings.Contains(output, "TEST RESULT: PASS") {
		t.Errorf("expected PASS banner, got output: %s", output)
	}
}

func TestFormatTerminalReport_IncompleteBanner(t *testing.T) {
	p := &plan.Plan{
		Name:     "aborted-run",
		Model:    core.WorkloadModelClosed,
		Users:    2,
		Duration: 1 * time.Second,
	}

	rep := &Report{
		ReportSchemaVersion: 1,
		Incomplete:          true,
	}

	output := FormatTerminalReport(rep, p)

	if !strings.Contains(output, "TEST RESULT: INCOMPLETE") {
		t.Errorf("expected INCOMPLETE banner, got output: %s", output)
	}
}

func TestFormatTerminalReport_OpenModel(t *testing.T) {
	p := &plan.Plan{
		Name:        "open-model-test",
		Model:       core.WorkloadModelOpen,
		Rate:        100,
		TimeUnit:    time.Second,
		MaxInFlight: 50,
		Duration:    10 * time.Second,
	}

	rep := &Report{
		ReportSchemaVersion: 1,
		DurationMS:          10000,
		WorkloadModel:       core.WorkloadModelOpen,
		RequestCounts: RequestCounts{
			Planned:   1000,
			Scheduled: 950,
			Started:   950,
			Completed: 940,
			Canceled:  10,
			Dropped:   50,
		},
		Outcomes: map[core.Outcome]int64{
			core.OutcomeSuccess: 940,
			core.OutcomeDropped: 50,
		},
		GeneratorHealth: GeneratorHealth{
			GoroutinesPeak:     40,
			CPUMaxPercent:      55.0,
			SchedulerLagMaxMS:  5.2,
			SaturationWarnings: []string{"target degradation or low max_in_flight caused dropped requests"},
		},
		Incomplete: false,
	}

	output := FormatTerminalReport(rep, p)

	if !strings.Contains(output, "Workload Model:    open (Target Rate: 100.00 req/s, Rate: 100.0/1s, Max In-Flight: 50)") {
		t.Errorf("missing or malformed open workload model line in output: %s", output)
	}
	if !strings.Contains(output, "Planned:           1000") {
		t.Errorf("missing planned count in output")
	}
	if !strings.Contains(output, "Dropped:           50") {
		t.Errorf("missing dropped count in output")
	}
	if !strings.Contains(output, "Achieved Start:    95.00 req/s") {
		t.Errorf("missing achieved start rate in output")
	}
	if !strings.Contains(output, "Completed Rate:    94.00 req/s") {
		t.Errorf("missing completed rate in output")
	}
	if !strings.Contains(output, "target degradation or low max_in_flight caused dropped requests") {
		t.Errorf("missing saturation warning in output")
	}
}

func TestFormatTerminalReport_ThresholdEvaluation(t *testing.T) {
	p := &plan.Plan{
		Name:     "threshold-eval-test",
		Model:    core.WorkloadModelClosed,
		Users:    5,
		Duration: 5 * time.Second,
	}

	rep := &Report{
		ReportSchemaVersion: 1,
		DurationMS:          5000,
		RequestCounts: RequestCounts{
			Planned:   500,
			Started:   500,
			Completed: 500,
		},
		Outcomes: map[core.Outcome]int64{
			core.OutcomeSuccess: 500,
		},
		Thresholds: []ThresholdResult{
			{
				Expression: "http_error_rate <= 1%",
				Target:     "<= 1%",
				Observed:   "0.00%",
				Passed:     true,
			},
			{
				Expression: "p95 <= 500ms",
				Target:     "<= 500ms",
				Observed:   "650.00ms",
				Passed:     false,
			},
			{
				Expression: "completed_rps >= 90",
				Target:     ">= 90",
				Observed:   "100.00 req/s",
				Passed:     true,
			},
		},
		Incomplete: false,
	}

	output := FormatTerminalReport(rep, p)

	// Verify Threshold Evaluation section
	if !strings.Contains(output, "THRESHOLD EVALUATION") {
		t.Errorf("missing THRESHOLD EVALUATION section in output:\n%s", output)
	}
	if !strings.Contains(output, "http_error_rate") || !strings.Contains(output, "0.00%") || !strings.Contains(output, "PASS") {
		t.Errorf("missing passing threshold row in output:\n%s", output)
	}
	if !strings.Contains(output, "p95") || !strings.Contains(output, "650.00ms") || !strings.Contains(output, "FAIL") {
		t.Errorf("missing failing threshold row in output:\n%s", output)
	}
	if !strings.Contains(output, "TEST RESULT: FAIL (thresholds failed)") {
		t.Errorf("expected FAIL (thresholds failed) banner, got:\n%s", output)
	}
}

func TestFormatTerminalReport_AllThresholdsPassBanner(t *testing.T) {
	p := &plan.Plan{
		Name:     "threshold-pass-test",
		Model:    core.WorkloadModelClosed,
		Users:    5,
		Duration: 5 * time.Second,
	}

	rep := &Report{
		ReportSchemaVersion: 1,
		DurationMS:          5000,
		RequestCounts: RequestCounts{
			Planned:   500,
			Started:   500,
			Completed: 500,
		},
		Outcomes: map[core.Outcome]int64{
			core.OutcomeSuccess: 500,
		},
		Thresholds: []ThresholdResult{
			{
				Expression: "http_error_rate <= 1%",
				Target:     "<= 1%",
				Observed:   "0.00%",
				Passed:     true,
			},
			{
				Expression: "p95 <= 500ms",
				Target:     "<= 500ms",
				Observed:   "35.00ms",
				Passed:     true,
			},
		},
		Incomplete: false,
	}

	output := FormatTerminalReport(rep, p)

	if !strings.Contains(output, "TEST RESULT: PASS") {
		t.Errorf("expected PASS banner when all thresholds pass, got:\n%s", output)
	}
}

func TestFormatTerminalReport_AuthSummary(t *testing.T) {
	parsedURL, _ := url.Parse("https://api.example.com/items")
	tokens := []string{"secret-tok-1", "secret-tok-2", "secret-tok-3"}
	authn, _ := auth.NewAuthenticator(&config.AuthConfig{
		Type:      config.AuthTypeTokenPool,
		TokenPool: tokens,
	})

	p := &plan.Plan{
		Name:             "auth-terminal-test",
		TargetURL:        parsedURL,
		Method:           "GET",
		Model:            core.WorkloadModelClosed,
		Users:            3,
		Duration:         5 * time.Second,
		Authenticator:    authn,
		CookieJarEnabled: true,
		KnownSecrets:     tokens,
	}

	rep := &Report{
		ReportSchemaVersion: 1,
		DurationMS:          5000,
		RequestCounts: RequestCounts{
			Planned:   100,
			Started:   100,
			Completed: 100,
		},
		Outcomes: map[core.Outcome]int64{
			core.OutcomeSuccess: 100,
		},
		Auth: &AuthReportSummary{
			AuthMode:         "token_pool",
			TokenCount:       3,
			CookieJarEnabled: true,
		},
		Incomplete: false,
	}

	output := FormatTerminalReport(rep, p)

	// Assert no secret tokens appear
	for _, tok := range tokens {
		if strings.Contains(output, tok) {
			t.Errorf("CRITICAL SECURITY VIOLATION: secret %q leaked in terminal report: %s", tok, output)
		}
	}

	// Assert auth banner line exists
	if !strings.Contains(output, "Auth:              token_pool (3 token(s), cookie jar enabled)") {
		t.Errorf("missing or incorrect Auth line in terminal report: %s", output)
	}
}

func TestFormatTerminalReport_ScenarioSteps(t *testing.T) {
	p := &plan.Plan{
		Name:   "scenario-terminal-test",
		Method: "SCENARIO",
		Model:  core.WorkloadModelClosed,
		Users:  5,
	}

	rep := &Report{
		ReportSchemaVersion: 1,
		DurationMS:          10000,
		RequestCounts: RequestCounts{
			Planned:   200,
			Started:   200,
			Completed: 200,
		},
		Outcomes: map[core.Outcome]int64{
			core.OutcomeSuccess: 200,
		},
		Scenario: &ScenarioReport{
			Name: "order_workflow",
			Iterations: metrics.ScenarioIterationCounts{
				Planned:   100,
				Started:   100,
				Completed: 100,
				Failed:    0,
			},
			Steps: []StepReport{
				{
					Name:   "login",
					Method: "POST",
					URL:    "https://api.example.com/login",
					RequestCounts: RequestCounts{
						Completed: 100,
					},
					Latency: LatencySummary{
						AllCompleted: LatencyPercentiles{
							P50MS: 15.0,
							P95MS: 30.0,
							P99MS: 45.0,
						},
					},
					CompletedThroughput: 10.0,
					ErrorRate:           0.0,
				},
				{
					Name:   "create_order",
					Method: "POST",
					URL:    "https://api.example.com/order",
					RequestCounts: RequestCounts{
						Completed: 100,
					},
					Latency: LatencySummary{
						AllCompleted: LatencyPercentiles{
							P50MS: 40.0,
							P95MS: 75.0,
							P99MS: 90.0,
						},
					},
					CompletedThroughput: 10.0,
					ErrorRate:           0.0,
				},
			},
		},
	}

	output := FormatTerminalReport(rep, p)

	if !strings.Contains(output, "SCENARIO: order_workflow (Planned: 100, Completed: 100, Failed: 0)") {
		t.Errorf("missing scenario header in terminal report: %s", output)
	}
	if !strings.Contains(output, "login") || !strings.Contains(output, "create_order") {
		t.Errorf("missing scenario steps in terminal report: %s", output)
	}
}
