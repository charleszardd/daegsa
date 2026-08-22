package threshold

import (
	"fmt"
	"math"
)

const floatEpsilon = 1e-9

// MetricsSnapshot contains aggregated load test measurements needed for threshold evaluation (§9, §10).
type MetricsSnapshot struct {
	PlannedRequests     int64
	ScheduledRequests   int64
	StartedRequests     int64
	CompletedRequests   int64
	CanceledRequests    int64
	DroppedRequests     int64
	SuccessfulRequests  int64
	MinLatencyMS        float64
	MaxLatencyMS        float64
	MeanLatencyMS       float64
	P50LatencyMS        float64
	P90LatencyMS        float64
	P95LatencyMS        float64
	P99LatencyMS        float64
	P999LatencyMS       float64
	AchievedStartRPS    float64
	CompletedThroughput float64
	ErrorRate           float64
	RateLimitedRate     float64
}

// EvaluationContext provides target workload parameters for threshold evaluation (§10).
type EvaluationContext struct {
	TargetRPS   float64
	MaxInFlight int64
}

// ReportResult is the serializable report representation of a threshold evaluation result (§13).
type ReportResult struct {
	Expression string `json:"expression"`
	Target     string `json:"target"`
	Observed   string `json:"observed"`
	Passed     bool   `json:"passed"`
}

// Evaluate evaluates a slice of parsed Threshold rules against a MetricsSnapshot and EvaluationContext (§10).
//
// Returns:
//   - results: detailed slice of Result for each threshold in the input order
//   - allPassed: true if all evaluated thresholds passed (or if no thresholds were configured)
//   - err: non-nil only if evaluation encountered an unrecoverable structural error
func Evaluate(thresholds []*Threshold, snap MetricsSnapshot, evalCtx EvaluationContext) ([]Result, bool, error) {
	if len(thresholds) == 0 {
		return make([]Result, 0), true, nil
	}

	results := make([]Result, 0, len(thresholds))
	allPassed := true

	for _, t := range thresholds {
		if t == nil {
			continue
		}

		observedVal, observedFmt := extractObserved(t.MetricName, t.Category, snap, evalCtx)
		passed := compareValues(observedVal, t.TargetValue, t.Operator)
		if !passed {
			allPassed = false
		}

		targetFmt := t.RawExpression
		if targetFmt == "" {
			targetFmt = fmt.Sprintf("%s %s", t.Operator, t.TargetRaw)
		}

		results = append(results, Result{
			Threshold:         t,
			MetricName:        t.MetricName,
			Operator:          t.Operator,
			TargetFormatted:   targetFmt,
			ObservedValue:     observedVal,
			ObservedFormatted: observedFmt,
			Passed:            passed,
		})
	}

	return results, allPassed, nil
}

// ToReportResults converts evaluation Results to serializable ReportResults (§13).
func ToReportResults(results []Result) []ReportResult {
	if len(results) == 0 {
		return make([]ReportResult, 0)
	}

	reportResults := make([]ReportResult, len(results))
	for i, r := range results {
		expr := r.MetricName + " " + r.TargetFormatted
		if r.Threshold != nil && r.Threshold.RawExpression != "" {
			expr = r.MetricName + " " + r.Threshold.RawExpression
		}

		reportResults[i] = ReportResult{
			Expression: expr,
			Target:     r.TargetFormatted,
			Observed:   r.ObservedFormatted,
			Passed:     r.Passed,
		}
	}

	return reportResults
}

func extractObserved(metricName string, category MetricCategory, snap MetricsSnapshot, evalCtx EvaluationContext) (float64, string) {
	switch metricName {
	// Rate & percentage metrics
	case MetricHTTPErrorRate:
		val := snap.ErrorRate
		return val, fmt.Sprintf("%.2f%%", val)

	case MetricRateLimitedRate:
		val := snap.RateLimitedRate
		return val, fmt.Sprintf("%.2f%%", val)

	case MetricDroppedRate:
		val := 0.0
		if snap.PlannedRequests > 0 {
			val = (float64(snap.DroppedRequests) / float64(snap.PlannedRequests)) * 100.0
		} else if totalScheduled := snap.StartedRequests + snap.DroppedRequests; totalScheduled > 0 {
			val = (float64(snap.DroppedRequests) / float64(totalScheduled)) * 100.0
		}
		return val, fmt.Sprintf("%.2f%%", val)

	// Latency metrics
	case MetricP50:
		val := snap.P50LatencyMS
		return val, fmt.Sprintf("%.2fms", val)

	case MetricP90:
		val := snap.P90LatencyMS
		return val, fmt.Sprintf("%.2fms", val)

	case MetricP95:
		val := snap.P95LatencyMS
		return val, fmt.Sprintf("%.2fms", val)

	case MetricP99:
		val := snap.P99LatencyMS
		return val, fmt.Sprintf("%.2fms", val)

	case MetricP999:
		val := snap.P999LatencyMS
		return val, fmt.Sprintf("%.2fms", val)

	case MetricMinLatency:
		val := snap.MinLatencyMS
		return val, fmt.Sprintf("%.2fms", val)

	case MetricMaxLatency:
		val := snap.MaxLatencyMS
		return val, fmt.Sprintf("%.2fms", val)

	case MetricMeanLatency:
		val := snap.MeanLatencyMS
		return val, fmt.Sprintf("%.2fms", val)

	// Throughput metrics
	case MetricCompletedRPS:
		val := snap.CompletedThroughput
		return val, fmt.Sprintf("%.2f req/s", val)

	case MetricStartedRPS:
		val := snap.AchievedStartRPS
		return val, fmt.Sprintf("%.2f req/s", val)

	case MetricTargetRPS:
		val := evalCtx.TargetRPS
		return val, fmt.Sprintf("%.2f req/s", val)

	// Request count metrics
	case MetricDroppedRequests:
		val := snap.DroppedRequests
		return float64(val), fmt.Sprintf("%d", val)

	case MetricFailedRequests:
		failed := snap.CompletedRequests - snap.SuccessfulRequests
		if failed < 0 {
			failed = 0
		}
		return float64(failed), fmt.Sprintf("%d", failed)

	case MetricCompletedRequests:
		val := snap.CompletedRequests
		return float64(val), fmt.Sprintf("%d", val)

	case MetricCanceledRequests:
		val := snap.CanceledRequests
		return float64(val), fmt.Sprintf("%d", val)

	// Concurrency metrics
	case MetricMaxInFlight:
		val := evalCtx.MaxInFlight
		return float64(val), fmt.Sprintf("%d", val)

	default:
		return 0.0, "0"
	}
}

func compareValues(observed, target float64, op string) bool {
	switch op {
	case OpLT:
		return observed < (target - floatEpsilon)
	case OpLTE:
		return observed <= (target + floatEpsilon)
	case OpGT:
		return observed > (target + floatEpsilon)
	case OpGTE:
		return observed >= (target - floatEpsilon)
	case OpEQ:
		return math.Abs(observed-target) <= floatEpsilon
	case OpNE:
		return math.Abs(observed-target) > floatEpsilon
	default:
		return false
	}
}
