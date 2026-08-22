package report

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/metrics"
	"github.com/charleszardd/daegsa/internal/plan"
)

func TestBuildReport_And_WriteJSONReport(t *testing.T) {
	p := &plan.Plan{
		Name:        "json-report-test",
		Fingerprint: "sha256:112233445566",
		Model:       core.WorkloadModelClosed,
	}

	agg := &metrics.AggregatedMetrics{
		RequestCounts: RequestCounts{
			Planned:   50,
			Started:   50,
			Completed: 50,
		},
		Outcomes: map[core.Outcome]int64{
			core.OutcomeSuccess: 50,
		},
		StatusCodes: map[string]int64{
			"200": 50,
		},
		Latency: LatencySummary{
			AllCompleted: LatencyPercentiles{
				MinMS: 5.0,
				P50MS: 10.0,
				MaxMS: 20.0,
			},
			ExpectedSuccess: LatencyPercentiles{
				MinMS: 5.0,
				P50MS: 10.0,
				MaxMS: 20.0,
			},
		},
	}

	health := &GeneratorHealth{
		GoroutinesPeak:    12,
		CPUMaxPercent:     15.0,
		SchedulerLagMaxMS: 0.5,
	}

	startTime := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	endTime := startTime.Add(5 * time.Second)

	rep := BuildReport(p, agg, health, startTime, endTime, false)

	if rep.ReportSchemaVersion != ExpectedReportSchemaVersion {
		t.Errorf("expected schema version %d, got %d", ExpectedReportSchemaVersion, rep.ReportSchemaVersion)
	}
	if rep.ConfigFingerprint != p.Fingerprint {
		t.Errorf("fingerprint mismatch: %s vs %s", rep.ConfigFingerprint, p.Fingerprint)
	}
	if rep.DurationMS != 5000 {
		t.Errorf("expected duration 5000ms, got %d", rep.DurationMS)
	}
	if rep.WorkloadModel != core.WorkloadModelClosed {
		t.Errorf("workload model mismatch: %s", rep.WorkloadModel)
	}
	if rep.RequestCounts.Completed != 50 {
		t.Errorf("expected 50 completed, got %d", rep.RequestCounts.Completed)
	}

	// Test writing to disk
	tempDir := t.TempDir()
	outPath := filepath.Join(tempDir, "sub", "report.json")

	if err := WriteJSONReport(outPath, rep); err != nil {
		t.Fatalf("WriteJSONReport failed: %v", err)
	}

	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read written report file: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("unmarshal generated report JSON failed: %v", err)
	}

	if int(parsed["report_schema_version"].(float64)) != 1 {
		t.Errorf("schema version in JSON = %v, want 1", parsed["report_schema_version"])
	}
	if parsed["incomplete"] != false {
		t.Errorf("incomplete in JSON = %v, want false", parsed["incomplete"])
	}
}

func TestWriteJSONReport_Errors(t *testing.T) {
	if err := WriteJSONReport("", &Report{}); err == nil {
		t.Errorf("expected error for empty file path")
	}
	if err := WriteJSONReport("some/path.json", nil); err == nil {
		t.Errorf("expected error for nil report")
	}
}
