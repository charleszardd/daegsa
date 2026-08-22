package doctor

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestCheckClockPrecision(t *testing.T) {
	ctx := context.Background()
	res := CheckClockPrecision(ctx)

	if res.Name == "" {
		t.Errorf("expected non-empty check name")
	}
	if res.Category != CategoryClock {
		t.Errorf("expected CategoryClock, got %s", res.Category)
	}
	if res.Status != StatusPass && res.Status != StatusWarn {
		t.Errorf("unexpected status: %s (detail: %s)", res.Status, res.Detail)
	}
	if res.Duration <= 0 {
		t.Errorf("expected positive duration, got %v", res.Duration)
	}
}

func TestCheckClockPrecision_Canceled(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	res := CheckClockPrecision(ctx)
	if res.Status != StatusFail {
		t.Errorf("expected StatusFail on canceled context, got %s", res.Status)
	}
}

func TestCheckDNSResolution(t *testing.T) {
	ctx := context.Background()
	res := CheckDNSResolution(ctx)

	if res.Category != CategoryDNS {
		t.Errorf("expected CategoryDNS, got %s", res.Category)
	}
	if res.Status == StatusFail {
		t.Errorf("DNS resolution for localhost failed: %s (%s)", res.Summary, res.Detail)
	}
}

func TestCheckTLSConfiguration(t *testing.T) {
	ctx := context.Background()
	res := CheckTLSConfiguration(ctx)

	if res.Category != CategoryTLS {
		t.Errorf("expected CategoryTLS, got %s", res.Category)
	}
	if res.Status == StatusFail {
		t.Errorf("TLS configuration check failed: %s (%s)", res.Summary, res.Detail)
	}
}

func TestCheckSocketLimits(t *testing.T) {
	ctx := context.Background()
	res := CheckSocketLimits(ctx)

	if res.Category != CategorySocket {
		t.Errorf("expected CategorySocket, got %s", res.Category)
	}
	if res.Status == StatusFail {
		t.Errorf("Socket check failed: %s (%s)", res.Summary, res.Detail)
	}
}

func TestCheckSystemResources(t *testing.T) {
	ctx := context.Background()
	res := CheckSystemResources(ctx)

	if res.Category != CategoryResources {
		t.Errorf("expected CategoryResources, got %s", res.Category)
	}
	if res.Status == StatusFail {
		t.Errorf("Resource check failed: %s", res.Detail)
	}
}

func TestRunDiagnostics(t *testing.T) {
	ctx := context.Background()
	opts := Options{
		Verbose: true,
		Timeout: 10 * time.Second,
	}

	report := RunDiagnostics(ctx, opts)
	if report == nil {
		t.Fatalf("expected non-nil DiagnosticReport")
	}

	if len(report.Checks) != 5 {
		t.Errorf("expected 5 checks, got %d", len(report.Checks))
	}
	if report.OverallStatus == "" {
		t.Errorf("expected non-empty OverallStatus")
	}
	if report.System.OS == "" || report.System.Arch == "" {
		t.Errorf("expected OS and Arch in system diagnostics, got %s/%s", report.System.OS, report.System.Arch)
	}

	// Test Terminal Report formatting
	output := FormatTerminalReport(report, false)
	if !strings.Contains(output, "DAEGSA SYSTEM DIAGNOSTICS") {
		t.Errorf("terminal report missing title header")
	}
	if !strings.Contains(output, "OVERALL STATUS:") {
		t.Errorf("terminal report missing overall status line")
	}

	verboseOutput := FormatTerminalReport(report, true)
	if !strings.Contains(verboseOutput, "Duration") {
		t.Errorf("verbose terminal report should contain Duration")
	}

	// Test JSON serialization
	jsonBytes, err := report.JSON()
	if err != nil {
		t.Fatalf("failed to serialize report to JSON: %v", err)
	}

	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonBytes, &parsed); err != nil {
		t.Fatalf("failed to unmarshal generated JSON: %v", err)
	}
	if _, ok := parsed["overall_status"]; !ok {
		t.Errorf("expected 'overall_status' in JSON output")
	}
}

func TestFormatTerminalReport_WarnAndFail(t *testing.T) {
	rep := &DiagnosticReport{
		Timestamp:     time.Now().UTC(),
		OverallStatus: StatusFail,
		TotalDuration: 15 * time.Millisecond,
		System: SystemDiagnostics{
			OS:         "linux",
			Arch:       "amd64",
			NumCPU:     1,
			GOMAXPROCS: 1,
			GoVersion:  "go1.23.0",
		},
		Checks: []CheckResult{
			{
				Name:     "Timer & Clock Precision",
				Category: CategoryClock,
				Status:   StatusPass,
				Summary:  "Accurate",
				Duration: 1 * time.Millisecond,
			},
			{
				Name:       "System Resources",
				Category:   CategoryResources,
				Status:     StatusWarn,
				Summary:    "Single core",
				Detail:     "1 CPU",
				Suggestion: "Use 2+ cores",
				Duration:   1 * time.Millisecond,
			},
			{
				Name:       "DNS Resolution",
				Category:   CategoryDNS,
				Status:     StatusFail,
				Summary:    "Failed to resolve localhost",
				Detail:     "Timeout",
				Suggestion: "Fix /etc/hosts",
				Duration:   10 * time.Millisecond,
			},
		},
	}

	output := FormatTerminalReport(rep, false)
	if !strings.Contains(output, "[FAIL]") {
		t.Errorf("expected [FAIL] in formatted report")
	}
	if !strings.Contains(output, "[WARN]") {
		t.Errorf("expected [WARN] in formatted report")
	}
	if !strings.Contains(output, "Remediation required") {
		t.Errorf("expected remediation note in failed report")
	}

	rep.OverallStatus = StatusWarn
	rep.Checks = rep.Checks[:2]
	warnOutput := FormatTerminalReport(rep, false)
	if !strings.Contains(warnOutput, "Advisory:") {
		t.Errorf("expected advisory note in warn report")
	}
}
