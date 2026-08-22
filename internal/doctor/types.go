package doctor

import (
	"encoding/json"
	"time"
)

// CheckStatus represents the diagnostic outcome of a single check or overall report (§14, §15).
type CheckStatus string

const (
	StatusPass CheckStatus = "PASS"
	StatusWarn CheckStatus = "WARN"
	StatusFail CheckStatus = "FAIL"
)

// Category groups diagnostic checks by domain.
type Category string

const (
	CategoryClock     Category = "clock"
	CategoryDNS       Category = "dns"
	CategoryTLS       Category = "tls"
	CategorySocket    Category = "socket"
	CategoryResources Category = "resources"
)

// CheckResult represents the outcome of an individual diagnostic check (§14).
type CheckResult struct {
	Name       string        `json:"name"`
	Category   Category      `json:"category"`
	Status     CheckStatus   `json:"status"`
	Summary    string        `json:"summary"`
	Detail     string        `json:"detail,omitempty"`
	Suggestion string        `json:"suggestion,omitempty"`
	Duration   time.Duration `json:"duration_ms"`
}

// MarshalJSON provides custom duration serialization in milliseconds for JSON reporting.
func (c CheckResult) MarshalJSON() ([]byte, error) {
	type Alias CheckResult
	return json.Marshal(&struct {
		Alias
		DurationMS float64 `json:"duration_ms"`
	}{
		Alias:      Alias(c),
		DurationMS: float64(c.Duration.Microseconds()) / 1000.0,
	})
}

// SystemDiagnostics captures environment metadata evaluated during diagnostic runs.
type SystemDiagnostics struct {
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	NumCPU      int    `json:"num_cpu"`
	GOMAXPROCS  int    `json:"gomaxprocs"`
	GoVersion   string `json:"go_version"`
	MemoryAlloc uint64 `json:"memory_alloc_bytes"`
	MemorySys   uint64 `json:"memory_sys_bytes"`
}

// DiagnosticReport aggregates the results of all doctor diagnostic checks (§14).
type DiagnosticReport struct {
	Timestamp     time.Time         `json:"timestamp_utc"`
	OverallStatus CheckStatus       `json:"overall_status"`
	Checks        []CheckResult     `json:"checks"`
	System        SystemDiagnostics `json:"system"`
	TotalDuration time.Duration     `json:"total_duration_ms"`
}

// Options configures diagnostic check execution.
type Options struct {
	Verbose bool
	Timeout time.Duration
}
