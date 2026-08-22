package report

import (
	"encoding/json"
	"time"

	"github.com/charleszardd/daegsa/internal/core"
)

// ExpectedReportSchemaVersion is the canonical report schema version (§13).
const ExpectedReportSchemaVersion = 1

// Report represents the canonical root JSON report document (§13).
type Report struct {
	ReportSchemaVersion int                        `json:"report_schema_version"`
	DaegsaVersion       string                     `json:"daegsa_version"`
	Commit              string                     `json:"commit"`
	BuildDate           string                     `json:"build_date"`
	OS                  string                     `json:"os"`
	Arch                string                     `json:"arch"`
	ConfigFingerprint   string                     `json:"config_fingerprint"`
	StartTimeUTC        time.Time                  `json:"start_time_utc"`
	EndTimeUTC          time.Time                  `json:"end_time_utc"`
	DurationMS          int64                      `json:"duration_ms"`
	WorkloadModel       core.WorkloadModel         `json:"workload_model"`
	RequestCounts       RequestCounts              `json:"request_counts"`
	Outcomes            map[core.Outcome]int64     `json:"outcomes"`
	StatusCodes         map[string]int64           `json:"status_codes"`
	Latency             LatencySummary             `json:"latency"`
	RateLimits          RateLimitObservations      `json:"rate_limits"`
	GeneratorHealth     GeneratorHealth            `json:"generator_health"`
	Thresholds          []ThresholdResult          `json:"thresholds"`
	Incomplete          bool                       `json:"incomplete"`
}

// RequestCounts aggregates request lifecycle counters (§9, §13).
type RequestCounts struct {
	Planned   int64 `json:"planned"`
	Scheduled int64 `json:"scheduled"`
	Started   int64 `json:"started"`
	Completed int64 `json:"completed"`
	Canceled  int64 `json:"canceled"`
	Dropped   int64 `json:"dropped"`
}

// LatencyPercentiles holds summary distribution metrics in milliseconds.
type LatencyPercentiles struct {
	MinMS  float64 `json:"min_ms"`
	MaxMS  float64 `json:"max_ms"`
	MeanMS float64 `json:"mean_ms"`
	P50MS  float64 `json:"p50_ms"`
	P90MS  float64 `json:"p90_ms"`
	P95MS  float64 `json:"p95_ms"`
	P99MS  float64 `json:"p99_ms"`
}

// LatencySummary separates metrics for all completed responses and expected-success responses.
type LatencySummary struct {
	AllCompleted    LatencyPercentiles `json:"all_completed"`
	ExpectedSuccess LatencyPercentiles `json:"expected_success"`
}

// RateLimitObservations captures 429 throttling and rate-limit header observations.
type RateLimitObservations struct {
	Observed429Count  int64                  `json:"observed_429_count"`
	RetryAfterSamples []string               `json:"retry_after_samples,omitempty"`
	RateLimitHeaders  []RateLimitHeaderSample `json:"rate_limit_headers,omitempty"`
}

// RateLimitHeaderSample stores observed rate-limiting headers.
type RateLimitHeaderSample struct {
	Limit     string `json:"limit,omitempty"`
	Remaining string `json:"remaining,omitempty"`
	Reset     string `json:"reset,omitempty"`
	Policy    string `json:"policy,omitempty"`
}

// GeneratorHealth records load generator resource metrics to distinguish client saturation (§13, §14).
type GeneratorHealth struct {
	CPUMaxPercent      float64  `json:"cpu_max_percent"`
	GoroutinesPeak     int64    `json:"goroutines_peak"`
	SchedulerLagMaxMS  float64  `json:"scheduler_lag_max_ms"`
	SaturationWarnings []string `json:"saturation_warnings,omitempty"`
}

// ThresholdResult records the pass/fail evaluation of a single threshold expression (§10, §13).
type ThresholdResult struct {
	Expression string `json:"expression"`
	Target     string `json:"target"`
	Observed   string `json:"observed"`
	Passed     bool   `json:"passed"`
}

// ToJSON marshals the report to formatted JSON bytes.
func (r *Report) ToJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}
