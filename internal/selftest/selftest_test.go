package selftest

import (
	"context"
	"encoding/json"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestRunSelfTests(t *testing.T) {
	ctx := context.Background()
	opts := Options{
		Verbose: true,
		Timeout: 10 * time.Second,
	}

	var progressCount int32
	report := RunSelfTests(ctx, opts, func(res SubTestResult) {
		atomic.AddInt32(&progressCount, 1)
		t.Logf("Self-test progress: [%s] %s (%v)", res.Status, res.Name, res.Duration)
	})

	if report == nil {
		t.Fatalf("expected non-nil SelfTestReport")
	}

	if atomic.LoadInt32(&progressCount) != 5 {
		t.Errorf("expected 5 progress events, got %d", progressCount)
	}

	if len(report.Tests) != 5 {
		t.Fatalf("expected 5 sub-tests, got %d", len(report.Tests))
	}

	if !report.Passed {
		for _, sub := range report.Tests {
			if sub.Status != StatusPass {
				t.Errorf("Subtest %q failed: detail=%s, err=%v", sub.Name, sub.Detail, sub.Err)
			}
		}
	}

	// Test Terminal Formatting
	termOut := FormatTerminalReport(report, false)
	if !strings.Contains(termOut, "DAEGSA IN-PROCESS SELF-TESTS") {
		t.Errorf("expected title in terminal report")
	}
	if !strings.Contains(termOut, "ALL SELF-TESTS PASSED") {
		t.Errorf("expected success banner in terminal report")
	}

	verboseTermOut := FormatTerminalReport(report, true)
	if !strings.Contains(verboseTermOut, "Detail :") {
		t.Errorf("expected detail in verbose terminal report")
	}

	// Test JSON Serialization
	jsonBytes, err := report.JSON()
	if err != nil {
		t.Fatalf("failed to serialize self-test report: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("failed to unmarshal self-test JSON: %v", err)
	}

	if parsed["passed"] != true {
		t.Errorf("expected passed=true in JSON")
	}
}

func TestRunSelfTests_ContextCanceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	opts := Options{
		Timeout: 5 * time.Second,
	}

	report := RunSelfTests(ctx, opts, nil)
	if report == nil {
		t.Fatalf("expected non-nil SelfTestReport")
	}

	if report.Passed {
		t.Errorf("expected overall Passed=false when context is already canceled")
	}
}

func TestFormatTerminalReport_Failed(t *testing.T) {
	report := &SelfTestReport{
		Timestamp:     time.Now().UTC(),
		Passed:        false,
		TotalDuration: 100 * time.Millisecond,
		Tests: []SubTestResult{
			{
				Name:              "Closed Workload Loop",
				Status:            StatusPass,
				Duration:          20 * time.Millisecond,
				RequestsCompleted: 50,
			},
			{
				Name:              "Open Arrival-Rate Pacing",
				Status:            StatusFail,
				Duration:          80 * time.Millisecond,
				RequestsCompleted: 10,
				Errors:            5,
				Detail:            "Rate limit saturated",
			},
		},
	}

	out := FormatTerminalReport(report, false)
	if !strings.Contains(out, "SELF-TESTS FAILED") {
		t.Errorf("expected failure banner in terminal report")
	}
	if !strings.Contains(out, "[FAIL]") {
		t.Errorf("expected [FAIL] badge in terminal report")
	}
	if !strings.Contains(out, "Rate limit saturated") {
		t.Errorf("expected detail on failed subtest in terminal report")
	}
}
