package report

import (
	"encoding/json"
	"time"

	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/metrics"
	"github.com/charleszardd/daegsa/internal/profile"
	"github.com/charleszardd/daegsa/internal/threshold"
)

// ExpectedReportSchemaVersion is the canonical report schema version (§13).
const (
	ExpectedReportSchemaVersion = 1
	ProfileReportSchemaVersion  = 2
)

// Type aliases to internal/metrics types for seamless JSON reporting (§13).
type RequestCounts = metrics.RequestCounts
type LatencyPercentiles = metrics.LatencyPercentiles
type LatencySummary = metrics.LatencySummary
type RateLimitObservations = metrics.RateLimitObservations
type RateLimitHeaderSample = metrics.RateLimitHeaderSample
type GeneratorHealth = metrics.GeneratorHealth

// StepReport records execution metrics for an individual scenario step (§6, §13).
type StepReport struct {
	Name                string                 `json:"name"`
	URL                 string                 `json:"url"`
	Method              string                 `json:"method"`
	RequestCounts       RequestCounts          `json:"request_counts"`
	Outcomes            map[core.Outcome]int64 `json:"outcomes"`
	StatusCodes         map[string]int64       `json:"status_codes"`
	Latency             LatencySummary         `json:"latency"`
	AchievedStartRPS    float64                `json:"achieved_start_rate"`
	CompletedThroughput float64                `json:"completed_throughput"`
	ErrorRate           float64                `json:"error_rate"`
}

// ScenarioReport records multi-step scenario execution metrics (§6, §13).
type ScenarioReport struct {
	Name       string                          `json:"name"`
	Iterations metrics.ScenarioIterationCounts `json:"iterations"`
	Steps      []StepReport                    `json:"steps,omitempty"`
}

// Report represents the canonical root JSON report document (§13).
type Report struct {
	ReportSchemaVersion int                       `json:"report_schema_version"`
	DaegsaVersion       string                    `json:"daegsa_version"`
	Commit              string                    `json:"commit"`
	BuildDate           string                    `json:"build_date"`
	OS                  string                    `json:"os"`
	Arch                string                    `json:"arch"`
	ConfigFingerprint   string                    `json:"config_fingerprint"`
	StartTimeUTC        time.Time                 `json:"start_time_utc"`
	EndTimeUTC          time.Time                 `json:"end_time_utc"`
	DurationMS          int64                     `json:"duration_ms"`
	WorkloadModel       core.WorkloadModel        `json:"workload_model"`
	RequestCounts       RequestCounts             `json:"request_counts"`
	Outcomes            map[core.Outcome]int64    `json:"outcomes"`
	StatusCodes         map[string]int64          `json:"status_codes"`
	Latency             LatencySummary            `json:"latency"`
	RateLimits          RateLimitObservations     `json:"rate_limits"`
	GeneratorHealth     GeneratorHealth           `json:"generator_health"`
	Auth                *AuthReportSummary        `json:"auth,omitempty"`
	Scenario            *ScenarioReport           `json:"scenario,omitempty"`
	Thresholds          []ThresholdResult         `json:"thresholds"`
	Incomplete          bool                      `json:"incomplete"`
	CompiledSegments    []profile.Segment         `json:"compiled_segments,omitempty"`
	Segments            []SegmentReport           `json:"segments,omitempty"`
	MeasuredSummary     *MetricsSummary           `json:"measured_summary,omitempty"`
	FirstThrottle       *FirstThrottleObservation `json:"first_throttle,omitempty"`
	Calibration         *metrics.Calibration      `json:"calibration,omitempty"`
}

// AuthReportSummary records sanitized authentication metadata in the JSON report (§11, §13).
type AuthReportSummary struct {
	AuthMode         string `json:"auth_mode"`
	TokenCount       int    `json:"token_count"`
	CookieJarEnabled bool   `json:"cookie_jar_enabled"`
}

// ThresholdResult records the pass/fail evaluation of a single threshold expression (§10, §13).
type ThresholdResult = threshold.ReportResult

// ToJSON marshals the report to formatted JSON bytes.
func (r *Report) ToJSON() ([]byte, error) {
	return json.MarshalIndent(r, "", "  ")
}
