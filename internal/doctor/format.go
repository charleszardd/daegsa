package doctor

import (
	"encoding/json"
	"fmt"
	"strings"
)

// JSON serializes the diagnostic report into formatted JSON (§14).
func (r *DiagnosticReport) JSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}

// FormatTerminalReport renders a human-readable diagnostic report for terminal display (§14).
func FormatTerminalReport(r *DiagnosticReport, verbose bool) string {
	var sb strings.Builder

	sb.WriteString("================================================================================\n")
	sb.WriteString("                           DAEGSA SYSTEM DIAGNOSTICS                            \n")
	sb.WriteString("================================================================================\n\n")

	sb.WriteString(fmt.Sprintf("  Host Environment : %s/%s (%d CPUs, GOMAXPROCS=%d)\n",
		r.System.OS, r.System.Arch, r.System.NumCPU, r.System.GOMAXPROCS))
	sb.WriteString(fmt.Sprintf("  Go Runtime       : %s\n", r.System.GoVersion))
	sb.WriteString(fmt.Sprintf("  Report Timestamp : %s\n\n", r.Timestamp.Format("2006-01-02 15:04:05 UTC")))

	sb.WriteString("--------------------------------------------------------------------------------\n")
	sb.WriteString(fmt.Sprintf("  %-6s  %-35s  %-10s  %s\n", "STATUS", "CHECK", "CATEGORY", "SUMMARY"))
	sb.WriteString("--------------------------------------------------------------------------------\n")

	passCount := 0
	warnCount := 0
	failCount := 0

	for _, check := range r.Checks {
		badge := fmt.Sprintf("[%s]", check.Status)
		switch check.Status {
		case StatusPass:
			passCount++
		case StatusWarn:
			warnCount++
		case StatusFail:
			failCount++
		}

		sb.WriteString(fmt.Sprintf("  %-6s  %-35s  %-10s  %s\n", badge, check.Name, check.Category, check.Summary))

		if verbose || check.Status != StatusPass {
			if check.Detail != "" {
				sb.WriteString(fmt.Sprintf("          Detail     : %s\n", check.Detail))
			}
			if check.Suggestion != "" {
				sb.WriteString(fmt.Sprintf("          Suggestion : %s\n", check.Suggestion))
			}
			sb.WriteString(fmt.Sprintf("          Duration   : %v\n", check.Duration))
		}
	}

	sb.WriteString("--------------------------------------------------------------------------------\n\n")

	sb.WriteString(fmt.Sprintf("OVERALL STATUS: %s (%d passed, %d warnings, %d failures in %v)\n\n",
		r.OverallStatus, passCount, warnCount, failCount, r.TotalDuration))

	if r.OverallStatus == StatusFail {
		sb.WriteString("Remediation required: One or more critical system diagnostics failed.\n")
		sb.WriteString("Review the suggestions above before running high-volume load tests.\n")
	} else if r.OverallStatus == StatusWarn {
		sb.WriteString("Advisory: All critical checks passed, but warnings were detected.\n")
		sb.WriteString("High-concurrency load testing may experience minor timing or resource constraints.\n")
	} else {
		sb.WriteString("System is healthy and fully ready for high-precision load testing.\n")
	}

	return sb.String()
}
