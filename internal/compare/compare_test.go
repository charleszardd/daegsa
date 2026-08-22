package compare

import (
	"testing"

	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/report"
)

func TestCompareZeroBaselineAndWarnings(t *testing.T) {
	baseline := &report.Report{ReportSchemaVersion: 1, WorkloadModel: core.WorkloadModelOpen, ConfigFingerprint: "a"}
	candidate := &report.Report{ReportSchemaVersion: 2, WorkloadModel: core.WorkloadModelClosed, ConfigFingerprint: "b"}
	result, err := Compare(baseline, candidate)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Warnings) < 2 {
		t.Fatalf("warnings = %v", result.Warnings)
	}
	if result.Deltas[0].PercentageAvailable {
		t.Fatal("zero baseline percentage must be unavailable")
	}
}

func TestCompareDetailedDeltasAndThresholdTransitions(t *testing.T) {
	baseline := &report.Report{
		ReportSchemaVersion: 2,
		WorkloadModel:       core.WorkloadModelOpen,
		ConfigFingerprint:   "fp123",
		DurationMS:          10000,
		RequestCounts: report.RequestCounts{
			Planned: 1000, Scheduled: 1000, Started: 1000, Completed: 1000,
		},
		Outcomes: map[core.Outcome]int64{core.OutcomeSuccess: 980, core.OutcomeUnexpectedStatus: 20},
		Latency: report.LatencySummary{
			AllCompleted: report.LatencyPercentiles{P50MS: 20.0, P90MS: 40.0, P95MS: 50.0, P99MS: 100.0, MaxMS: 200.0},
		},
		RateLimits: report.RateLimitObservations{Observed429Count: 5},
		Thresholds: []report.ThresholdResult{
			{Expression: "p95_ms <= 100ms", Target: "<= 100ms", Observed: "50ms", Passed: true},
			{Expression: "error_rate <= 1%", Target: "<= 1%", Observed: "2%", Passed: false},
			{Expression: "removed_check <= 10", Target: "<= 10", Observed: "5", Passed: true},
		},
	}

	candidate := &report.Report{
		ReportSchemaVersion: 2,
		WorkloadModel:       core.WorkloadModelOpen,
		ConfigFingerprint:   "fp123",
		DurationMS:          10000,
		RequestCounts: report.RequestCounts{
			Planned: 1000, Scheduled: 1000, Started: 1000, Completed: 1000,
		},
		Outcomes: map[core.Outcome]int64{core.OutcomeSuccess: 995, core.OutcomeUnexpectedStatus: 5},
		Latency: report.LatencySummary{
			AllCompleted: report.LatencyPercentiles{P50MS: 25.0, P90MS: 45.0, P95MS: 55.0, P99MS: 110.0, MaxMS: 220.0},
		},
		RateLimits: report.RateLimitObservations{Observed429Count: 0},
		Thresholds: []report.ThresholdResult{
			{Expression: "p95_ms <= 100ms", Target: "<= 100ms", Observed: "55ms", Passed: true},
			{Expression: "error_rate <= 1%", Target: "<= 1%", Observed: "0.5%", Passed: true},
			{Expression: "new_check <= 5", Target: "<= 5", Observed: "2", Passed: true},
		},
	}

	result, err := Compare(baseline, candidate)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Warnings) != 0 {
		t.Errorf("expected no warnings for matching configurations, got %v", result.Warnings)
	}

	// Verify p95 delta: 50 -> 55 (+5, +10%)
	var foundP95 bool
	for _, d := range result.Deltas {
		if d.Name == "p95_ms" {
			foundP95 = true
			if d.Baseline != 50.0 || d.Candidate != 55.0 || d.Absolute != 5.0 || !d.PercentageAvailable || d.Percentage != 10.0 {
				t.Errorf("unexpected p95 delta: %+v", d)
			}
		}
	}
	if !foundP95 {
		t.Error("p95_ms delta not found")
	}

	// Verify threshold transitions:
	// "error_rate <= 1%" -> fail-to-pass
	// "removed_check <= 10" -> removed
	// "new_check <= 5" -> added
	transitions := make(map[string]string)
	for _, tc := range result.ThresholdChanges {
		transitions[tc.Expression] = tc.Change
	}

	if transitions["error_rate <= 1%"] != "fail-to-pass" {
		t.Errorf("expected fail-to-pass, got %q", transitions["error_rate <= 1%"])
	}
	if transitions["removed_check <= 10"] != "removed" {
		t.Errorf("expected removed, got %q", transitions["removed_check <= 10"])
	}
	if transitions["new_check <= 5"] != "added" {
		t.Errorf("expected added, got %q", transitions["new_check <= 5"])
	}

	// Verify output string formatting
	formatted := result.String()
	if formatted == "" {
		t.Error("expected non-empty formatted result string")
	}
}
