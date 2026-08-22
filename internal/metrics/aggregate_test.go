package metrics

import (
	"math"
	"testing"
	"time"

	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/executor"
)

func TestMergeWorkers_ReconciliationAndMath(t *testing.T) {
	numWorkers := 10
	workers := make([]*WorkerMetrics, numWorkers)

	for i := 0; i < numWorkers; i++ {
		wm := NewWorkerMetrics(i)
		wm.Planned = 100
		wm.Started = 100
		wm.Completed = 95
		wm.Canceled = 5
		wm.Dropped = 0

		// Record 90 successes (10ms) and 5 500s (20ms)
		for j := 0; j < 90; j++ {
			wm.RecordResult(&executor.Result{
				Outcome:       core.OutcomeSuccess,
				StatusCode:    200,
				Latency:       10 * time.Millisecond,
				BytesSent:     100,
				BytesReceived: 500,
			})
		}
		for j := 0; j < 5; j++ {
			wm.RecordResult(&executor.Result{
				Outcome:       core.OutcomeUnexpectedStatus,
				StatusCode:    500,
				Latency:       20 * time.Millisecond,
				BytesSent:     100,
				BytesReceived: 100,
			})
		}

		workers[i] = wm
	}

	testDuration := 10 * time.Second
	agg, err := MergeWorkers(workers, testDuration)
	if err != nil {
		t.Fatalf("unexpected error merging workers: %v", err)
	}

	// Verify request counts
	if agg.RequestCounts.Planned != 1000 {
		t.Errorf("expected planned 1000, got %d", agg.RequestCounts.Planned)
	}
	if agg.RequestCounts.Started != 1000 {
		t.Errorf("expected started 1000, got %d", agg.RequestCounts.Started)
	}
	if agg.RequestCounts.Completed != 950 {
		t.Errorf("expected completed 950, got %d", agg.RequestCounts.Completed)
	}
	if agg.RequestCounts.Canceled != 50 {
		t.Errorf("expected canceled 50, got %d", agg.RequestCounts.Canceled)
	}
	if agg.RequestCounts.Dropped != 0 {
		t.Errorf("expected dropped 0, got %d", agg.RequestCounts.Dropped)
	}

	// Verify outcomes
	if agg.Outcomes[core.OutcomeSuccess] != 900 {
		t.Errorf("expected 900 success outcomes, got %d", agg.Outcomes[core.OutcomeSuccess])
	}
	if agg.Outcomes[core.OutcomeUnexpectedStatus] != 50 {
		t.Errorf("expected 50 unexpected status outcomes, got %d", agg.Outcomes[core.OutcomeUnexpectedStatus])
	}

	// Verify status codes
	if agg.StatusCodes["200"] != 900 {
		t.Errorf("expected 900 '200' status codes, got %d", agg.StatusCodes["200"])
	}
	if agg.StatusCodes["500"] != 50 {
		t.Errorf("expected 50 '500' status codes, got %d", agg.StatusCodes["500"])
	}

	// Verify rates
	if math.Abs(agg.AchievedStartRPS-100.0) > 0.01 {
		t.Errorf("expected start RPS ~100.0, got %f", agg.AchievedStartRPS)
	}
	if math.Abs(agg.CompletedThroughput-95.0) > 0.01 {
		t.Errorf("expected throughput ~95.0, got %f", agg.CompletedThroughput)
	}
	// Error rate = (950 - 900) / 950 = 50 / 950 = 5.263%
	expectedErrorRate := (50.0 / 950.0) * 100.0
	if math.Abs(agg.ErrorRate-expectedErrorRate) > 0.01 {
		t.Errorf("expected error rate ~%f%%, got %f%%", expectedErrorRate, agg.ErrorRate)
	}

	// Verify bytes
	expectedSent := int64(10 * (90*100 + 5*100))
	expectedRecv := int64(10 * (90*500 + 5*100))
	if agg.TotalBytesSent != expectedSent {
		t.Errorf("expected bytes sent %d, got %d", expectedSent, agg.TotalBytesSent)
	}
	if agg.TotalBytesReceived != expectedRecv {
		t.Errorf("expected bytes received %d, got %d", expectedRecv, agg.TotalBytesReceived)
	}

	// Verify latencies
	if agg.Latency.AllCompleted.MinMS < 9.0 || agg.Latency.AllCompleted.MinMS > 11.0 {
		t.Errorf("expected min latency ~10ms, got %f", agg.Latency.AllCompleted.MinMS)
	}
	if agg.Latency.ExpectedSuccess.MinMS < 9.0 || agg.Latency.ExpectedSuccess.MinMS > 11.0 {
		t.Errorf("expected success min latency ~10ms, got %f", agg.Latency.ExpectedSuccess.MinMS)
	}
}

func TestMergeWorkers_Empty(t *testing.T) {
	agg, err := MergeWorkers(nil, 5*time.Second)
	if err != nil {
		t.Fatalf("unexpected error merging empty workers: %v", err)
	}
	if agg.RequestCounts.Planned != 0 || agg.AchievedStartRPS != 0.0 {
		t.Errorf("expected zeroes for empty worker merge, got %+v", agg)
	}
}
