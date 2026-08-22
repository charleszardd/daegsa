package report

import (
	"encoding/json"
	"time"

	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/metrics"
	"github.com/charleszardd/daegsa/internal/threshold"
)

// ExpectedReportSchemaVersion is the canonical report schema version (§13).
const ExpectedReportSchemaVersion = 1

// Type aliases to internal/metrics types for seamless JSON reporting (§13).
type RequestCounts = metrics.RequestCounts
type LatencyPercentiles = metrics.LatencyPercentiles
type LatencySummary = metrics.LatencySummary
type RateLimitObservations = metrics.RateLimitObservations
type RateLimitHeaderSample = metrics.RateLimitHeaderSample
type GeneratorHealth = metrics.GeneratorHealth

// Report represents the canonical root JSON report document (§13).
type Report struct {
	ReportSchemaVersion int                    `json:"report_schema_version"`
	DaegsaVersion       string                 `json:"daegsa_version"`
	Commit              string                 `json:"commit"`
	BuildDate           string                 `json:"build_date"`
	OS                  string                 `json:"os"`
	Arch                string                 `json:"arch"`
	ConfigFingerprint   string                 `json:"config_fingerprint"`
	StartTimeUTC        time.Time              `json:"start_time_utc"`
	EndTimeUTC          time.Time              `json:"end_time_utc"`
	DurationMS          int64                  `json:"duration_ms"`
	WorkloadModel       core.WorkloadModel     `json:"workload_model"`
	RequestCounts       RequestCounts          `json:"request_counts"`
	Outcomes            map[core.Outcome]int64 `json:"outcomes"`
	StatusCodes         map[string]int64       `json:"status_codes"`
	Latency             LatencySummary         `json:"latency"`
	RateLimits          RateLimitObservations  `json:"rate_limits"`
	GeneratorHealth     GeneratorHealth        `json:"generator_health"`
	Thresholds          []ThresholdResult      `json:"thresholds"`
	Incomplete          bool                   `json:"incomplete"`
}

// ThresholdResult records the pass/fail evaluation of a single threshold expression (§10, §13).
type ThresholdResult = threshold.ReportResult

// ToJSON marshals the report to formatted JSON bytes.
func (r *Report) ToJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}
