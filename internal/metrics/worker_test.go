package metrics

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/executor"
)

func TestWorkerMetrics_RecordResult(t *testing.T) {
	wm := NewWorkerMetrics(0)

	// Record a success result
	resSuccess := &executor.Result{
		Outcome:       core.OutcomeSuccess,
		StatusCode:    http.StatusOK,
		Latency:       10 * time.Millisecond,
		TTFB:          4 * time.Millisecond,
		BytesSent:     150,
		BytesReceived: 1024,
	}
	wm.RecordResult(resSuccess)

	if wm.Outcomes[core.OutcomeSuccess] != 1 {
		t.Errorf("expected 1 success outcome, got %d", wm.Outcomes[core.OutcomeSuccess])
	}
	if wm.StatusCodes[200] != 1 {
		t.Errorf("expected 1 200 status code, got %d", wm.StatusCodes[200])
	}
	if wm.BytesSent != 150 {
		t.Errorf("expected 150 bytes sent, got %d", wm.BytesSent)
	}
	if wm.BytesReceived != 1024 {
		t.Errorf("expected 1024 bytes received, got %d", wm.BytesReceived)
	}
	if wm.TTFBCount != 1 || wm.TTFBSumMicroseconds != 4000 {
		t.Errorf("expected TTFB 4000µs / count 1, got %d / %d", wm.TTFBSumMicroseconds, wm.TTFBCount)
	}
	if wm.AllLatency.Count() != 1 || wm.SuccessLatency.Count() != 1 {
		t.Errorf("expected 1 sample in AllLatency and SuccessLatency, got %d / %d",
			wm.AllLatency.Count(), wm.SuccessLatency.Count())
	}
}

func TestWorkerMetrics_All12Outcomes(t *testing.T) {
	wm := NewWorkerMetrics(1)

	for _, outcome := range core.AllOutcomes {
		res := &executor.Result{
			Outcome:    outcome,
			StatusCode: 0,
			Latency:    5 * time.Millisecond,
		}
		if outcome.IsSuccess() {
			res.StatusCode = 200
		} else if outcome == core.OutcomeRateLimited {
			res.StatusCode = 429
		} else if outcome == core.OutcomeUnexpectedStatus {
			res.StatusCode = 500
		}
		wm.RecordResult(res)
	}

	for _, outcome := range core.AllOutcomes {
		if wm.Outcomes[outcome] != 1 {
			t.Errorf("expected 1 count for outcome %s, got %d", outcome, wm.Outcomes[outcome])
		}
	}
}

func TestWorkerMetrics_ErrorSampleBounding(t *testing.T) {
	wm := NewWorkerMetrics(2)

	// Record 50 distinct error messages
	for i := 0; i < 50; i++ {
		res := &executor.Result{
			Outcome: core.OutcomeConnectError,
			Latency: 1 * time.Millisecond,
			Err:     errors.New(fmt.Sprintf("connection refused on port %d", 8000+i)),
		}
		wm.RecordResult(res)
	}

	if len(wm.ErrorSamples) > MaxErrorSamples {
		t.Fatalf("expected error samples capped at %d, got %d", MaxErrorSamples, len(wm.ErrorSamples))
	}
	if len(wm.ErrorSamples) != MaxErrorSamples {
		t.Fatalf("expected exactly %d error samples, got %d", MaxErrorSamples, len(wm.ErrorSamples))
	}

	// Record duplicate of first error -> count should increase
	dupRes := &executor.Result{
		Outcome: core.OutcomeConnectError,
		Latency: 1 * time.Millisecond,
		Err:     errors.New("connection refused on port 8000"),
	}
	wm.RecordResult(dupRes)

	if wm.ErrorSamples[0].Count != 2 {
		t.Errorf("expected first error count to be 2, got %d", wm.ErrorSamples[0].Count)
	}
}

func TestWorkerMetrics_RateLimitObservation(t *testing.T) {
	wm := NewWorkerMetrics(3)

	retrySecs := int64(15)
	limit := int64(100)
	remaining := int64(0)
	resetSecs := int64(30)

	res := &executor.Result{
		Outcome:    core.OutcomeRateLimited,
		StatusCode: http.StatusTooManyRequests,
		Latency:    2 * time.Millisecond,
		RateLimitInfo: &executor.RateLimitInfo{
			RetryAfterSeconds: &retrySecs,
			Limit:             &limit,
			Remaining:         &remaining,
			ResetSeconds:      &resetSecs,
			Policy:            "standard",
		},
	}
	wm.RecordResult(res)

	if wm.RateLimits.Observed429Count != 1 {
		t.Errorf("expected 1 429 observed, got %d", wm.RateLimits.Observed429Count)
	}
	if len(wm.RateLimits.RetryAfterSamples) != 1 || wm.RateLimits.RetryAfterSamples[0] != "15s" {
		t.Errorf("expected '15s' retry after sample, got %v", wm.RateLimits.RetryAfterSamples)
	}
	if len(wm.RateLimits.RateLimitHeaders) != 1 {
		t.Fatalf("expected 1 rate limit header sample, got %d", len(wm.RateLimits.RateLimitHeaders))
	}
	hdr := wm.RateLimits.RateLimitHeaders[0]
	if hdr.Limit != "100" || hdr.Remaining != "0" || hdr.Reset != "30s" || hdr.Policy != "standard" {
		t.Errorf("unexpected rate limit header values: %+v", hdr)
	}
}

func TestWorkerMetrics_Snapshot(t *testing.T) {
	wm := NewWorkerMetrics(4)
	wm.Planned = 10
	wm.Started = 8
	wm.Completed = 7

	res := &executor.Result{
		Outcome:    core.OutcomeSuccess,
		StatusCode: 200,
		Latency:    10 * time.Millisecond,
	}
	wm.RecordResult(res)

	snap := wm.Snapshot()

	if snap.WorkerID != wm.WorkerID || snap.Planned != wm.Planned || snap.Completed != wm.Completed {
		t.Errorf("snapshot metadata mismatch: %+v vs %+v", snap, wm)
	}
	if snap.AllLatency.Count() != wm.AllLatency.Count() {
		t.Errorf("snapshot histogram count mismatch")
	}

	// Mutate original and verify snapshot unchanged
	wm.Completed = 100
	_ = wm.AllLatency.Record(50000)

	if snap.Completed == wm.Completed {
		t.Errorf("expected snapshot completed to remain unchanged")
	}
	if snap.AllLatency.Count() == wm.AllLatency.Count() {
		t.Errorf("expected snapshot histogram to remain unchanged")
	}
}
