package threshold_test

import (
	"testing"
	"time"

	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/metrics"
	"github.com/charleszardd/daegsa/internal/threshold"
)

func TestEvaluate_AllPassing(t *testing.T) {
	hist := metrics.NewLatencyHistogram()
	for i := 1; i <= 100; i++ {
		// Record values from 1ms (1000µs) to 100ms (100000µs)
		_ = hist.Record(int64(i * 1000))
	}

	agg := &metrics.AggregatedMetrics{
		RequestCounts: metrics.RequestCounts{
			Planned:   1000,
			Scheduled: 1000,
			Started:   1000,
			Completed: 1000,
			Canceled:  0,
			Dropped:   0,
		},
		Outcomes: map[core.Outcome]int64{
			core.OutcomeSuccess: 1000,
		},
		Latency: metrics.LatencySummary{
			AllCompleted: metrics.LatencyPercentiles{
				MinMS:  1.0,
				MaxMS:  100.0,
				MeanMS: 50.5,
				P50MS:  50.0,
				P90MS:  90.0,
				P95MS:  95.0,
				P99MS:  99.0,
			},
		},
		AllLatencyHist:      hist,
		Duration:            10 * time.Second,
		AchievedStartRPS:    100.0,
		CompletedThroughput: 100.0,
		ErrorRate:           0.0,
		RateLimitedRate:     0.0,
	}

	evalCtx := threshold.EvaluationContext{
		TargetRPS:   100.0,
		MaxInFlight: 50,
	}

	thresholdRules := map[string]string{
		threshold.MetricHTTPErrorRate:     "<= 1%",
		threshold.MetricRateLimitedRate:   "<= 5%",
		threshold.MetricDroppedRate:       "== 0%",
		threshold.MetricP50:               "<= 60ms",
		threshold.MetricP90:               "<= 95ms",
		threshold.MetricP95:               "<= 100ms",
		threshold.MetricP99:               "<= 100ms",
		threshold.MetricP999:              "<= 110ms",
		threshold.MetricMinLatency:        ">= 0.5ms",
		threshold.MetricMaxLatency:        "<= 150ms",
		threshold.MetricMeanLatency:       "<= 60ms",
		threshold.MetricCompletedRPS:      ">= 90",
		threshold.MetricStartedRPS:        ">= 95 req/s",
		threshold.MetricTargetRPS:         "== 100 req/s",
		threshold.MetricDroppedRequests:   "== 0",
		threshold.MetricFailedRequests:    "== 0",
		threshold.MetricCompletedRequests: ">= 1000",
		threshold.MetricCanceledRequests:  "== 0",
		threshold.MetricMaxInFlight:       "<= 50",
	}

	thresholds, err := threshold.ParseThresholds(thresholdRules)
	if err != nil {
		t.Fatalf("ParseThresholds failed: %v", err)
	}

	results, allPassed, err := threshold.Evaluate(thresholds, agg.ToThresholdSnapshot(), evalCtx)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if !allPassed {
		t.Errorf("expected allPassed to be true, got false")
		for _, r := range results {
			if !r.Passed {
				t.Errorf("failing threshold: %s: target=%s, observed=%s", r.MetricName, r.TargetFormatted, r.ObservedFormatted)
			}
		}
	}

	if len(results) != len(thresholdRules) {
		t.Errorf("expected %d results, got %d", len(thresholdRules), len(results))
	}
}

func TestEvaluate_Failures(t *testing.T) {
	agg := &metrics.AggregatedMetrics{
		RequestCounts: metrics.RequestCounts{
			Planned:   100,
			Scheduled: 100,
			Started:   100,
			Completed: 100,
			Canceled:  0,
			Dropped:   10,
		},
		Outcomes: map[core.Outcome]int64{
			core.OutcomeSuccess:          95,
			core.OutcomeUnexpectedStatus: 5,
		},
		Latency: metrics.LatencySummary{
			AllCompleted: metrics.LatencyPercentiles{
				MinMS:  10.0,
				MaxMS:  800.0,
				MeanMS: 150.0,
				P50MS:  100.0,
				P90MS:  400.0,
				P95MS:  600.0,
				P99MS:  750.0,
			},
		},
		Duration:            10 * time.Second,
		AchievedStartRPS:    10.0,
		CompletedThroughput: 10.0,
		ErrorRate:           5.0,
		RateLimitedRate:     0.0,
	}

	evalCtx := threshold.EvaluationContext{
		TargetRPS:   20.0,
		MaxInFlight: 100,
	}

	thresholdRules := map[string]string{
		threshold.MetricHTTPErrorRate:   "<= 1%",    // Observed 5.0% -> FAIL
		threshold.MetricP95:             "<= 500ms", // Observed 600ms -> FAIL
		threshold.MetricCompletedRPS:    ">= 15",    // Observed 10 req/s -> FAIL
		threshold.MetricDroppedRequests: "== 0",     // Observed 10 -> FAIL
		threshold.MetricFailedRequests:  "== 0",     // Observed 5 -> FAIL
	}

	thresholds, err := threshold.ParseThresholds(thresholdRules)
	if err != nil {
		t.Fatalf("ParseThresholds failed: %v", err)
	}

	results, allPassed, err := threshold.Evaluate(thresholds, agg.ToThresholdSnapshot(), evalCtx)
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if allPassed {
		t.Errorf("expected allPassed to be false, got true")
	}

	for _, r := range results {
		if r.Passed {
			t.Errorf("expected metric %q to fail, but it passed (target=%s, observed=%s)",
				r.MetricName, r.TargetFormatted, r.ObservedFormatted)
		}
	}
}

func TestEvaluate_BoundaryAndFloatEpsilon(t *testing.T) {
	agg := &metrics.AggregatedMetrics{
		RequestCounts: metrics.RequestCounts{
			Planned:   100,
			Started:   100,
			Completed: 100,
		},
		Outcomes: map[core.Outcome]int64{
			core.OutcomeSuccess: 100,
		},
		Latency: metrics.LatencySummary{
			AllCompleted: metrics.LatencyPercentiles{
				P95MS: 500.0,
			},
		},
		CompletedThroughput: 100.0000000001,
		ErrorRate:           1.0000000001, // Slightly above 1.0 within 1e-9 tolerance
	}

	thresholdRules := map[string]string{
		threshold.MetricP95:           "<= 500ms", // Exact match -> PASS
		threshold.MetricCompletedRPS:  "== 100",   // Within epsilon -> PASS
		threshold.MetricHTTPErrorRate: "<= 1%",    // Within epsilon -> PASS
	}

	thresholds, err := threshold.ParseThresholds(thresholdRules)
	if err != nil {
		t.Fatalf("ParseThresholds failed: %v", err)
	}

	_, allPassed, err := threshold.Evaluate(thresholds, agg.ToThresholdSnapshot(), threshold.EvaluationContext{})
	if err != nil {
		t.Fatalf("Evaluate failed: %v", err)
	}

	if !allPassed {
		t.Errorf("expected boundary checks within epsilon to pass, got allPassed=false")
	}
}

func TestEvaluate_ZeroCompletedAndNilSafety(t *testing.T) {
	// Evaluator should not panic on zero MetricsSnapshot
	thresholdRules := map[string]string{
		threshold.MetricHTTPErrorRate:   "<= 1%",
		threshold.MetricDroppedRequests: "== 0",
		threshold.MetricP95:             "<= 500ms",
	}

	thresholds, err := threshold.ParseThresholds(thresholdRules)
	if err != nil {
		t.Fatalf("ParseThresholds failed: %v", err)
	}

	results, allPassed, err := threshold.Evaluate(thresholds, threshold.MetricsSnapshot{}, threshold.EvaluationContext{})
	if err != nil {
		t.Fatalf("Evaluate with nil inputs returned error: %v", err)
	}

	if !allPassed {
		t.Errorf("expected 0 observed against <= 1%%, == 0, <= 500ms to pass, got false")
	}

	if len(results) != 3 {
		t.Errorf("expected 3 results, got %d", len(results))
	}
}

func TestToReportResults(t *testing.T) {
	t1, _ := threshold.ParseThreshold(threshold.MetricHTTPErrorRate, "<= 1%")
	t2, _ := threshold.ParseThreshold(threshold.MetricP95, "<= 500ms")

	results := []threshold.Result{
		{
			Threshold:         t1,
			MetricName:        threshold.MetricHTTPErrorRate,
			Operator:          threshold.OpLTE,
			TargetFormatted:   "<= 1%",
			ObservedValue:     0.0,
			ObservedFormatted: "0.00%",
			Passed:            true,
		},
		{
			Threshold:         t2,
			MetricName:        threshold.MetricP95,
			Operator:          threshold.OpLTE,
			TargetFormatted:   "<= 500ms",
			ObservedValue:     650.0,
			ObservedFormatted: "650.00ms",
			Passed:            false,
		},
	}

	reportResults := threshold.ToReportResults(results)
	if len(reportResults) != 2 {
		t.Fatalf("expected 2 report results, got %d", len(reportResults))
	}

	if reportResults[0].Expression != "http_error_rate <= 1%" {
		t.Errorf("Expression = %q, want %q", reportResults[0].Expression, "http_error_rate <= 1%")
	}
	if reportResults[0].Target != "<= 1%" {
		t.Errorf("Target = %q, want %q", reportResults[0].Target, "<= 1%")
	}
	if reportResults[0].Observed != "0.00%" {
		t.Errorf("Observed = %q, want %q", reportResults[0].Observed, "0.00%")
	}
	if !reportResults[0].Passed {
		t.Errorf("Passed = %v, want true", reportResults[0].Passed)
	}

	if reportResults[1].Expression != "p95 <= 500ms" {
		t.Errorf("Expression = %q, want %q", reportResults[1].Expression, "p95 <= 500ms")
	}
	if reportResults[1].Passed {
		t.Errorf("Passed = %v, want false", reportResults[1].Passed)
	}
}

func TestEvaluate_StepThresholds(t *testing.T) {
	thresholdMap := map[string]string{
		"p95":                        "<= 200ms", // root threshold
		"step.login.p95":             "<= 50ms",  // passing step threshold
		"step.items.p95":             "<= 30ms",  // failing step threshold (observed 80ms)
		"step.items.http_error_rate": "<= 1%",    // passing step threshold
	}

	thresholds, err := threshold.ParseThresholds(thresholdMap)
	if err != nil {
		t.Fatalf("ParseThresholds error: %v", err)
	}

	rootSnap := threshold.MetricsSnapshot{
		P95LatencyMS: 120.0, // <= 200ms (PASS)
	}

	stepSnaps := map[string]threshold.MetricsSnapshot{
		"login": {
			P95LatencyMS: 40.0, // <= 50ms (PASS)
		},
		"items": {
			P95LatencyMS: 80.0, // <= 30ms (FAIL)
			ErrorRate:    0.0,  // <= 1% (PASS)
		},
	}

	evalCtx := threshold.EvaluationContext{
		TargetRPS:   100.0,
		MaxInFlight: 50,
	}

	results, allPassed, err := threshold.EvaluateWithSteps(thresholds, rootSnap, stepSnaps, evalCtx)
	if err != nil {
		t.Fatalf("EvaluateWithSteps error: %v", err)
	}

	if allPassed {
		t.Errorf("expected allPassed=false due to step.items.p95 violation")
	}

	if len(results) != 4 {
		t.Fatalf("expected 4 results, got %d", len(results))
	}

	passCount := 0
	failCount := 0
	for _, r := range results {
		if r.Passed {
			passCount++
		} else {
			failCount++
			if r.MetricName != "step.items.p95" {
				t.Errorf("unexpected failing metric: %q", r.MetricName)
			}
		}
	}

	if passCount != 3 || failCount != 1 {
		t.Errorf("expected 3 pass and 1 fail, got pass=%d fail=%d", passCount, failCount)
	}
}
