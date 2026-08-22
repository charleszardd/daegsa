package benchmarks

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/charleszardd/daegsa/internal/clock"
	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/testtarget"
)

// BenchmarkControllableClockAdvance benchmarks virtual time advancement across 10,000 scheduled timers.
func BenchmarkControllableClockAdvance(b *testing.B) {
	initial := time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC)
	c := clock.NewControllableClock(initial)

	const timerCount = 10000
	for i := 0; i < timerCount; i++ {
		_ = c.NewTimer(time.Duration(i+1) * time.Millisecond)
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		c.Advance(1 * time.Millisecond)
	}
}

// BenchmarkOutcomeClassifier_ZeroAlloc benchmarks OutcomeClassifier.Classify to enforce 0 heap allocations.
func BenchmarkOutcomeClassifier_ZeroAlloc(b *testing.B) {
	classifier := core.NewOutcomeClassifier()
	input := core.ClassifyInput{
		StatusCode:       http.StatusOK,
		ExpectedStatuses: []int{http.StatusOK},
		Err:              nil,
		Canceled:         false,
		Dropped:          false,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = classifier.Classify(input)
	}
}

// TestOutcomeClassifier_StrictZeroAllocations strictly verifies that Classify performs 0 heap allocations.
func TestOutcomeClassifier_StrictZeroAllocations(t *testing.T) {
	classifier := core.NewOutcomeClassifier()
	expectedStatuses := []int{200, 201}
	input := core.ClassifyInput{
		StatusCode:       200,
		ExpectedStatuses: expectedStatuses,
	}

	allocs := testing.AllocsPerRun(1000, func() {
		_ = classifier.Classify(input)
	})

	if allocs != 0 {
		t.Errorf("OutcomeClassifier.Classify allocated %f objects, want 0", allocs)
	}
}

// BenchmarkTargetServerThroughput benchmarks TargetServer under local concurrent requests.
func BenchmarkTargetServerThroughput(b *testing.B) {
	ts := testtarget.NewServer()
	defer ts.Close()

	client := &http.Client{
		Transport: &http.Transport{
			MaxIdleConns:        100,
			MaxIdleConnsPerHost: 100,
		},
	}

	url := ts.URL() + "/benchmark?status=200"

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req, _ := http.NewRequestWithContext(context.Background(), "GET", url, nil)
			resp, err := client.Do(req)
			if err == nil {
				_ = resp.Body.Close()
			}
		}
	})
}
