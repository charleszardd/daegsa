package threshold

import "errors"

// Canonical metric name constants (§6, §9, §10).
const (
	// Rate and percentage metrics (0.0% to 100.0%, unit: %)
	MetricHTTPErrorRate   = "http_error_rate"
	MetricRateLimitedRate = "rate_limited_rate"
	MetricDroppedRate     = "dropped_rate"

	// Latency metrics (duration in milliseconds, units: ms, s, µs, us)
	MetricP50         = "p50"
	MetricP90         = "p90"
	MetricP95         = "p95"
	MetricP99         = "p99"
	MetricP999        = "p99.9"
	MetricMinLatency  = "min_latency"
	MetricMaxLatency  = "max_latency"
	MetricMeanLatency = "mean_latency"

	// Throughput metrics (rates in req/s, units: req/s, rps, or bare number)
	MetricCompletedRPS = "completed_rps"
	MetricStartedRPS   = "started_rps"
	MetricTargetRPS    = "target_rps"

	// Request count metrics (non-negative integer counts)
	MetricDroppedRequests   = "dropped_requests"
	MetricFailedRequests    = "failed_requests"
	MetricCompletedRequests = "completed_requests"
	MetricCanceledRequests  = "canceled_requests"

	// Concurrency metrics (non-negative integer concurrency)
	MetricMaxInFlight = "max_in_flight"
)

// Canonical comparison operator constants (§10).
const (
	OpLT  = "<"
	OpLTE = "<="
	OpGT  = ">"
	OpGTE = ">="
	OpEQ  = "=="
	OpNE  = "!="
)

// MetricCategory classifies the unit and value semantics of a metric (§10).
type MetricCategory string

const (
	MetricCategoryRate        MetricCategory = "rate"
	MetricCategoryLatency     MetricCategory = "latency"
	MetricCategoryThroughput  MetricCategory = "throughput"
	MetricCategoryCount       MetricCategory = "count"
	MetricCategoryConcurrency MetricCategory = "concurrency"
)

// CanonicalMetricCategories maps each canonical metric name to its MetricCategory.
var CanonicalMetricCategories = map[string]MetricCategory{
	MetricHTTPErrorRate:   MetricCategoryRate,
	MetricRateLimitedRate: MetricCategoryRate,
	MetricDroppedRate:     MetricCategoryRate,

	MetricP50:         MetricCategoryLatency,
	MetricP90:         MetricCategoryLatency,
	MetricP95:         MetricCategoryLatency,
	MetricP99:         MetricCategoryLatency,
	MetricP999:        MetricCategoryLatency,
	MetricMinLatency:  MetricCategoryLatency,
	MetricMaxLatency:  MetricCategoryLatency,
	MetricMeanLatency: MetricCategoryLatency,

	MetricCompletedRPS: MetricCategoryThroughput,
	MetricStartedRPS:   MetricCategoryThroughput,
	MetricTargetRPS:    MetricCategoryThroughput,

	MetricDroppedRequests:   MetricCategoryCount,
	MetricFailedRequests:    MetricCategoryCount,
	MetricCompletedRequests: MetricCategoryCount,
	MetricCanceledRequests:  MetricCategoryCount,

	MetricMaxInFlight: MetricCategoryConcurrency,
}

// AllCanonicalMetrics is the list of all supported canonical metric names in deterministic order.
var AllCanonicalMetrics = []string{
	MetricHTTPErrorRate,
	MetricRateLimitedRate,
	MetricDroppedRate,
	MetricP50,
	MetricP90,
	MetricP95,
	MetricP99,
	MetricP999,
	MetricMinLatency,
	MetricMaxLatency,
	MetricMeanLatency,
	MetricCompletedRPS,
	MetricStartedRPS,
	MetricTargetRPS,
	MetricDroppedRequests,
	MetricFailedRequests,
	MetricCompletedRequests,
	MetricCanceledRequests,
	MetricMaxInFlight,
}

var (
	// ErrInvalidThreshold indicates a threshold expression or metric name is syntactically or semantically invalid.
	ErrInvalidThreshold = errors.New("invalid threshold")
)

// Threshold represents a validated, parsed performance threshold (§6, §10).
type Threshold struct {
	MetricName    string         `json:"metric_name"`
	StepName      string         `json:"step_name,omitempty"`
	Category      MetricCategory `json:"category"`
	Operator      string         `json:"operator"`
	TargetRaw     string         `json:"target_raw"`
	TargetValue   float64        `json:"target_value"`
	Unit          string         `json:"unit"`
	RawExpression string         `json:"raw_expression"`
}

// Clone creates a deep copy of the Threshold.
func (t *Threshold) Clone() *Threshold {
	if t == nil {
		return nil
	}
	cloned := *t
	return &cloned
}

// Result records the evaluation of a single Threshold against observed metrics (§10, §13).
type Result struct {
	Threshold         *Threshold `json:"threshold"`
	MetricName        string     `json:"metric_name"`
	Operator          string     `json:"operator"`
	TargetFormatted   string     `json:"target_formatted"`
	ObservedValue     float64    `json:"observed_value"`
	ObservedFormatted string     `json:"observed_formatted"`
	Passed            bool       `json:"passed"`
}
