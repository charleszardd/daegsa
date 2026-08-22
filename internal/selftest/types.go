package selftest

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// TestStatus represents the pass/fail state of a self-test subtest (§14, §15).
type TestStatus string

const (
	StatusPass TestStatus = "PASS"
	StatusFail TestStatus = "FAIL"
)

// SubTestResult represents the result of an individual in-process self-test (§14).
type SubTestResult struct {
	Name              string        `json:"name"`
	Status            TestStatus    `json:"status"`
	Duration          time.Duration `json:"duration_ms"`
	RequestsCompleted int64         `json:"requests_completed"`
	Errors            int64         `json:"errors"`
	Detail            string        `json:"detail,omitempty"`
	Err               error         `json:"-"`
}

// MarshalJSON serializes SubTestResult with duration in milliseconds for JSON reporting.
func (r SubTestResult) MarshalJSON() ([]byte, error) {
	type Alias SubTestResult
	var errMsg string
	if r.Err != nil {
		errMsg = r.Err.Error()
	}
	return json.Marshal(&struct {
		Alias
		DurationMS float64 `json:"duration_ms"`
		Error      string  `json:"error,omitempty"`
	}{
		Alias:      Alias(r),
		DurationMS: float64(r.Duration.Microseconds()) / 1000.0,
		Error:      errMsg,
	})
}

// SelfTestReport aggregates the results of all in-process self-tests (§14).
type SelfTestReport struct {
	Timestamp     time.Time       `json:"timestamp_utc"`
	Passed        bool            `json:"passed"`
	Tests         []SubTestResult `json:"tests"`
	TotalDuration time.Duration   `json:"total_duration_ms"`
}

// MarshalJSON serializes SelfTestReport with duration in milliseconds.
func (r SelfTestReport) MarshalJSON() ([]byte, error) {
	type Alias SelfTestReport
	return json.Marshal(&struct {
		Alias
		TotalDurationMS float64 `json:"total_duration_ms"`
	}{
		Alias:           Alias(r),
		TotalDurationMS: float64(r.TotalDuration.Microseconds()) / 1000.0,
	})
}

// JSON serializes the self-test report into indented JSON (§14).
func (r *SelfTestReport) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// Options configures self-test execution.
type Options struct {
	Verbose bool
	Timeout time.Duration
}

// FormatTerminalReport renders human-readable self-test results for terminal display (§14).
func FormatTerminalReport(r *SelfTestReport, verbose bool) string {
	var sb strings.Builder

	sb.WriteString("================================================================================\n")
	sb.WriteString("                           DAEGSA IN-PROCESS SELF-TESTS                         \n")
	sb.WriteString("================================================================================\n\n")

	sb.WriteString(fmt.Sprintf("  Report Timestamp : %s\n", r.Timestamp.Format("2006-01-02 15:04:05 UTC")))
	sb.WriteString(fmt.Sprintf("  Total Duration   : %v\n\n", r.TotalDuration))

	sb.WriteString("--------------------------------------------------------------------------------\n")
	sb.WriteString(fmt.Sprintf("  %-6s  %-35s  %-12s  %s\n", "STATUS", "SUB-TEST", "REQUESTS", "DURATION"))
	sb.WriteString("--------------------------------------------------------------------------------\n")

	passedCount := 0
	failedCount := 0

	for _, t := range r.Tests {
		badge := fmt.Sprintf("[%s]", t.Status)
		if t.Status == StatusPass {
			passedCount++
		} else {
			failedCount++
		}

		reqSummary := fmt.Sprintf("%d reqs", t.RequestsCompleted)
		if t.Errors > 0 {
			reqSummary = fmt.Sprintf("%d reqs (%d errs)", t.RequestsCompleted, t.Errors)
		}

		sb.WriteString(fmt.Sprintf("  %-6s  %-35s  %-12s  %v\n", badge, t.Name, reqSummary, t.Duration))

		if verbose || t.Status == StatusFail {
			if t.Detail != "" {
				sb.WriteString(fmt.Sprintf("          Detail : %s\n", t.Detail))
			}
			if t.Err != nil {
				sb.WriteString(fmt.Sprintf("          Error  : %v\n", t.Err))
			}
		}
	}

	sb.WriteString("--------------------------------------------------------------------------------\n\n")

	if r.Passed {
		sb.WriteString(fmt.Sprintf("ALL SELF-TESTS PASSED (%d/%d tests passed in %v)\n",
			passedCount, len(r.Tests), r.TotalDuration))
	} else {
		sb.WriteString(fmt.Sprintf("SELF-TESTS FAILED (%d passed, %d failed out of %d tests in %v)\n",
			passedCount, failedCount, len(r.Tests), r.TotalDuration))
	}

	return sb.String()
}
