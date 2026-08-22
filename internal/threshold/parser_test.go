package threshold_test

import (
	"errors"
	"testing"

	"github.com/charleszardd/daegsa/internal/threshold"
)

func TestParseThreshold_ValidExpressions(t *testing.T) {
	tests := []struct {
		name        string
		metricName  string
		expr        string
		wantCat     threshold.MetricCategory
		wantOp      string
		wantTarget  float64
		wantUnit    string
	}{
		// Rates / Percentages
		{
			name:       "http_error_rate lte 1%",
			metricName: threshold.MetricHTTPErrorRate,
			expr:       "<= 1%",
			wantCat:    threshold.MetricCategoryRate,
			wantOp:     threshold.OpLTE,
			wantTarget: 1.0,
			wantUnit:   "%",
		},
		{
			name:       "rate_limited_rate lt 5.5%",
			metricName: threshold.MetricRateLimitedRate,
			expr:       "< 5.5%",
			wantCat:    threshold.MetricCategoryRate,
			wantOp:     threshold.OpLT,
			wantTarget: 5.5,
			wantUnit:   "%",
		},
		{
			name:       "dropped_rate eq 0%",
			metricName: threshold.MetricDroppedRate,
			expr:       "== 0%",
			wantCat:    threshold.MetricCategoryRate,
			wantOp:     threshold.OpEQ,
			wantTarget: 0.0,
			wantUnit:   "%",
		},
		// Latencies
		{
			name:       "p50 lte 250ms",
			metricName: threshold.MetricP50,
			expr:       "<= 250ms",
			wantCat:    threshold.MetricCategoryLatency,
			wantOp:     threshold.OpLTE,
			wantTarget: 250.0,
			wantUnit:   "ms",
		},
		{
			name:       "p90 lt 500ms",
			metricName: threshold.MetricP90,
			expr:       "< 500ms",
			wantCat:    threshold.MetricCategoryLatency,
			wantOp:     threshold.OpLT,
			wantTarget: 500.0,
			wantUnit:   "ms",
		},
		{
			name:       "p95 lte 1s",
			metricName: threshold.MetricP95,
			expr:       "<= 1s",
			wantCat:    threshold.MetricCategoryLatency,
			wantOp:     threshold.OpLTE,
			wantTarget: 1000.0,
			wantUnit:   "s",
		},
		{
			name:       "p99 lte 2.5s",
			metricName: threshold.MetricP99,
			expr:       "<= 2.5s",
			wantCat:    threshold.MetricCategoryLatency,
			wantOp:     threshold.OpLTE,
			wantTarget: 2500.0,
			wantUnit:   "s",
		},
		{
			name:       "p99.9 lte 500µs",
			metricName: threshold.MetricP999,
			expr:       "<= 500µs",
			wantCat:    threshold.MetricCategoryLatency,
			wantOp:     threshold.OpLTE,
			wantTarget: 0.5,
			wantUnit:   "µs",
		},
		{
			name:       "p99.9 lte 500us",
			metricName: threshold.MetricP999,
			expr:       "<= 500us",
			wantCat:    threshold.MetricCategoryLatency,
			wantOp:     threshold.OpLTE,
			wantTarget: 0.5,
			wantUnit:   "µs",
		},
		{
			name:       "min_latency gte 1ms",
			metricName: threshold.MetricMinLatency,
			expr:       ">= 1ms",
			wantCat:    threshold.MetricCategoryLatency,
			wantOp:     threshold.OpGTE,
			wantTarget: 1.0,
			wantUnit:   "ms",
		},
		{
			name:       "max_latency lt 5000ms",
			metricName: threshold.MetricMaxLatency,
			expr:       "< 5000ms",
			wantCat:    threshold.MetricCategoryLatency,
			wantOp:     threshold.OpLT,
			wantTarget: 5000.0,
			wantUnit:   "ms",
		},
		{
			name:       "mean_latency lte 50ms",
			metricName: threshold.MetricMeanLatency,
			expr:       "<= 50ms",
			wantCat:    threshold.MetricCategoryLatency,
			wantOp:     threshold.OpLTE,
			wantTarget: 50.0,
			wantUnit:   "ms",
		},
		// Throughput
		{
			name:       "completed_rps gte 300",
			metricName: threshold.MetricCompletedRPS,
			expr:       ">= 300",
			wantCat:    threshold.MetricCategoryThroughput,
			wantOp:     threshold.OpGTE,
			wantTarget: 300.0,
			wantUnit:   "req/s",
		},
		{
			name:       "completed_rps gte 150 req/s",
			metricName: threshold.MetricCompletedRPS,
			expr:       ">= 150 req/s",
			wantCat:    threshold.MetricCategoryThroughput,
			wantOp:     threshold.OpGTE,
			wantTarget: 150.0,
			wantUnit:   "req/s",
		},
		{
			name:       "started_rps gt 99.5 rps",
			metricName: threshold.MetricStartedRPS,
			expr:       "> 99.5 rps",
			wantCat:    threshold.MetricCategoryThroughput,
			wantOp:     threshold.OpGT,
			wantTarget: 99.5,
			wantUnit:   "req/s",
		},
		{
			name:       "target_rps eq 100",
			metricName: threshold.MetricTargetRPS,
			expr:       "== 100",
			wantCat:    threshold.MetricCategoryThroughput,
			wantOp:     threshold.OpEQ,
			wantTarget: 100.0,
			wantUnit:   "req/s",
		},
		// Counts
		{
			name:       "dropped_requests eq 0",
			metricName: threshold.MetricDroppedRequests,
			expr:       "== 0",
			wantCat:    threshold.MetricCategoryCount,
			wantOp:     threshold.OpEQ,
			wantTarget: 0.0,
			wantUnit:   "",
		},
		{
			name:       "failed_requests eq 0",
			metricName: threshold.MetricFailedRequests,
			expr:       "== 0",
			wantCat:    threshold.MetricCategoryCount,
			wantOp:     threshold.OpEQ,
			wantTarget: 0.0,
			wantUnit:   "",
		},
		{
			name:       "completed_requests gte 1000",
			metricName: threshold.MetricCompletedRequests,
			expr:       ">= 1000",
			wantCat:    threshold.MetricCategoryCount,
			wantOp:     threshold.OpGTE,
			wantTarget: 1000.0,
			wantUnit:   "",
		},
		{
			name:       "canceled_requests lte 5",
			metricName: threshold.MetricCanceledRequests,
			expr:       "<= 5",
			wantCat:    threshold.MetricCategoryCount,
			wantOp:     threshold.OpLTE,
			wantTarget: 5.0,
			wantUnit:   "",
		},
		{
			name:       "canceled_requests ne 10",
			metricName: threshold.MetricCanceledRequests,
			expr:       "!= 10",
			wantCat:    threshold.MetricCategoryCount,
			wantOp:     threshold.OpNE,
			wantTarget: 10.0,
			wantUnit:   "",
		},
		// Concurrency
		{
			name:       "max_in_flight lte 500",
			metricName: threshold.MetricMaxInFlight,
			expr:       "<= 500",
			wantCat:    threshold.MetricCategoryConcurrency,
			wantOp:     threshold.OpLTE,
			wantTarget: 500.0,
			wantUnit:   "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := threshold.ParseThreshold(tt.metricName, tt.expr)
			if err != nil {
				t.Fatalf("ParseThreshold(%q, %q) returned error: %v", tt.metricName, tt.expr, err)
			}
			if got.MetricName != tt.metricName {
				t.Errorf("MetricName = %q, want %q", got.MetricName, tt.metricName)
			}
			if got.Category != tt.wantCat {
				t.Errorf("Category = %q, want %q", got.Category, tt.wantCat)
			}
			if got.Operator != tt.wantOp {
				t.Errorf("Operator = %q, want %q", got.Operator, tt.wantOp)
			}
			if got.TargetValue != tt.wantTarget {
				t.Errorf("TargetValue = %g, want %g", got.TargetValue, tt.wantTarget)
			}
			if got.Unit != tt.wantUnit {
				t.Errorf("Unit = %q, want %q", got.Unit, tt.wantUnit)
			}
		})
	}
}

func TestParseThreshold_InvalidExpressions(t *testing.T) {
	tests := []struct {
		name       string
		metricName string
		expr       string
	}{
		{
			name:       "empty metric name",
			metricName: "",
			expr:       "<= 1%",
		},
		{
			name:       "unknown metric name",
			metricName: "custom_magic_metric",
			expr:       "<= 1%",
		},
		{
			name:       "empty expression",
			metricName: threshold.MetricHTTPErrorRate,
			expr:       "",
		},
		{
			name:       "missing operator",
			metricName: threshold.MetricHTTPErrorRate,
			expr:       "1%",
		},
		{
			name:       "missing target value",
			metricName: threshold.MetricHTTPErrorRate,
			expr:       "<=",
		},
		{
			name:       "rate missing percent unit",
			metricName: threshold.MetricHTTPErrorRate,
			expr:       "<= 5",
		},
		{
			name:       "rate unit duration mismatch",
			metricName: threshold.MetricHTTPErrorRate,
			expr:       "<= 500ms",
		},
		{
			name:       "rate out of bounds high",
			metricName: threshold.MetricHTTPErrorRate,
			expr:       "<= 105%",
		},
		{
			name:       "rate out of bounds negative",
			metricName: threshold.MetricHTTPErrorRate,
			expr:       "<= -1%",
		},
		{
			name:       "latency missing duration unit",
			metricName: threshold.MetricP95,
			expr:       "<= 500",
		},
		{
			name:       "latency percent unit mismatch",
			metricName: threshold.MetricP95,
			expr:       "<= 5%",
		},
		{
			name:       "latency negative value",
			metricName: threshold.MetricP95,
			expr:       "<= -500ms",
		},
		{
			name:       "throughput invalid unit",
			metricName: threshold.MetricCompletedRPS,
			expr:       ">= 100ms",
		},
		{
			name:       "throughput percent unit mismatch",
			metricName: threshold.MetricCompletedRPS,
			expr:       ">= 100%",
		},
		{
			name:       "count with decimal",
			metricName: threshold.MetricDroppedRequests,
			expr:       "== 0.5",
		},
		{
			name:       "count with unit",
			metricName: threshold.MetricDroppedRequests,
			expr:       "== 0ms",
		},
		{
			name:       "count negative",
			metricName: threshold.MetricDroppedRequests,
			expr:       "<= -1",
		},
		{
			name:       "concurrency with decimal",
			metricName: threshold.MetricMaxInFlight,
			expr:       "<= 10.5",
		},
		{
			name:       "concurrency with unit",
			metricName: threshold.MetricMaxInFlight,
			expr:       "<= 100req/s",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := threshold.ParseThreshold(tt.metricName, tt.expr)
			if err == nil {
				t.Fatalf("ParseThreshold(%q, %q) expected error, got nil", tt.metricName, tt.expr)
			}
			if !errors.Is(err, threshold.ErrInvalidThreshold) {
				t.Errorf("error %v does not wrap ErrInvalidThreshold", err)
			}
		})
	}
}

func TestParseThresholds_DeterministicOrdering(t *testing.T) {
	thresholdMap := map[string]string{
		threshold.MetricP99:             "<= 1s",
		threshold.MetricHTTPErrorRate:   "<= 1%",
		threshold.MetricCompletedRPS:    ">= 90",
		threshold.MetricDroppedRequests: "== 0",
	}

	thresholds, err := threshold.ParseThresholds(thresholdMap)
	if err != nil {
		t.Fatalf("ParseThresholds returned error: %v", err)
	}

	if len(thresholds) != 4 {
		t.Fatalf("expected 4 thresholds, got %d", len(thresholds))
	}

	// Should be sorted alphabetically by metric name:
	// completed_rps, dropped_requests, http_error_rate, p99
	expectedOrder := []string{
		threshold.MetricCompletedRPS,
		threshold.MetricDroppedRequests,
		threshold.MetricHTTPErrorRate,
		threshold.MetricP99,
	}

	for i, expected := range expectedOrder {
		if thresholds[i].MetricName != expected {
			t.Errorf("thresholds[%d].MetricName = %q, want %q", i, thresholds[i].MetricName, expected)
		}
	}
}
