package report_test

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/report"
)

func TestReport_Serialization(t *testing.T) {
	startTime := time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)
	endTime := startTime.Add(30 * time.Second)

	rep := report.Report{
		ReportSchemaVersion: 1,
		DaegsaVersion:       "0.1.0",
		Commit:              "abcd123",
		BuildDate:           "2026-08-22",
		OS:                  "windows",
		Arch:                "amd64",
		ConfigFingerprint:   "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855",
		StartTimeUTC:        startTime,
		EndTimeUTC:          endTime,
		DurationMS:          30000,
		WorkloadModel:       core.WorkloadModelOpen,
		RequestCounts: report.RequestCounts{
			Planned:   3000,
			Scheduled: 3000,
			Started:   3000,
			Completed: 2990,
			Canceled:  10,
			Dropped:   0,
		},
		Outcomes: map[core.Outcome]int64{
			core.OutcomeSuccess:          2950,
			core.OutcomeUnexpectedStatus: 40,
			core.OutcomeCanceled:         10,
		},
		StatusCodes: map[string]int64{
			"200": 2950,
			"500": 40,
		},
		Latency: report.LatencySummary{
			AllCompleted: report.LatencyPercentiles{
				MinMS:  1.2,
				MaxMS:  125.4,
				MeanMS: 15.3,
				P50MS:  12.0,
				P90MS:  25.0,
				P95MS:  35.5,
				P99MS:  80.0,
			},
			ExpectedSuccess: report.LatencyPercentiles{
				MinMS:  1.2,
				MaxMS:  95.0,
				MeanMS: 14.1,
				P50MS:  11.5,
				P90MS:  22.0,
				P95MS:  30.0,
				P99MS:  65.0,
			},
		},
		RateLimits: report.RateLimitObservations{
			Observed429Count: 0,
		},
		GeneratorHealth: report.GeneratorHealth{
			CPUMaxPercent:     18.5,
			GoroutinesPeak:    35,
			SchedulerLagMaxMS: 0.8,
		},
		Thresholds: []report.ThresholdResult{
			{
				Expression: "<= 1%",
				Target:     "http_error_rate",
				Observed:   "1.33%",
				Passed:     false,
			},
			{
				Expression: "<= 500ms",
				Target:     "p95",
				Observed:   "35.5ms",
				Passed:     true,
			},
		},
		Incomplete: false,
	}

	data, err := rep.ToJSON()
	if err != nil {
		t.Fatalf("Report.ToJSON() error: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal generated JSON: %v", err)
	}

	// Verify required fields are present and not nil
	requiredFields := []string{
		"report_schema_version",
		"daegsa_version",
		"commit",
		"build_date",
		"os",
		"arch",
		"config_fingerprint",
		"start_time_utc",
		"end_time_utc",
		"duration_ms",
		"workload_model",
		"request_counts",
		"outcomes",
		"status_codes",
		"latency",
		"rate_limits",
		"generator_health",
		"thresholds",
		"incomplete",
	}

	for _, field := range requiredFields {
		if _, exists := parsed[field]; !exists {
			t.Errorf("expected field %q in serialized JSON report", field)
		}
	}

	if int(parsed["report_schema_version"].(float64)) != 1 {
		t.Errorf("report_schema_version = %v, want 1", parsed["report_schema_version"])
	}
}

func TestReport_IncompleteOnCancellation(t *testing.T) {
	rep := report.Report{
		ReportSchemaVersion: 1,
		DaegsaVersion:       "0.1.0",
		Incomplete:          true,
		RequestCounts: report.RequestCounts{
			Planned:  1000,
			Canceled: 500,
		},
	}

	data, err := rep.ToJSON()
	if err != nil {
		t.Fatalf("Report.ToJSON() error: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("failed to unmarshal JSON: %v", err)
	}

	if parsed["incomplete"] != true {
		t.Errorf("incomplete = %v, want true", parsed["incomplete"])
	}
}
