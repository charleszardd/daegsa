package benchmarks

import (
	"testing"
	"time"
)

// BenchmarkTimerResolution characterizes the precision of time.Now() monotonic calls.
func BenchmarkTimerResolution(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = time.Now()
	}
}

// TestTimerResolutionCharacterization calculates the minimum observable delta of time.Now().
func TestTimerResolutionCharacterization(t *testing.T) {
	var minDelta time.Duration = time.Hour
	samples := 100

	for i := 0; i < samples; i++ {
		t1 := time.Now()
		var t2 time.Time
		for {
			t2 = time.Now()
			if !t2.Equal(t1) {
				break
			}
		}
		delta := t2.Sub(t1)
		if delta > 0 && delta < minDelta {
			minDelta = delta
		}
	}

	t.Logf("Windows AMD64 Minimum observable time.Now() resolution: %v", minDelta)
}

// TestSleepResolutionCharacterization characterizes the minimum effective resolution of time.Sleep.
func TestSleepResolutionCharacterization(t *testing.T) {
	requested := 1 * time.Millisecond
	iterations := 20
	var totalElapsed time.Duration

	for i := 0; i < iterations; i++ {
		start := time.Now()
		time.Sleep(requested)
		elapsed := time.Since(start)
		totalElapsed += elapsed
	}

	avgSleep := totalElapsed / time.Duration(iterations)
	t.Logf("Windows AMD64 time.Sleep(1ms) observed average: %v", avgSleep)
}

// TestSchedulerDriftCharacterization characterizes cumulative drift over 1000 intervals.
func TestSchedulerDriftCharacterization(t *testing.T) {
	targetInterval := 100 * time.Microsecond
	ticks := 1000

	start := time.Now()
	for i := 1; i <= ticks; i++ {
		targetTime := start.Add(time.Duration(i) * targetInterval)
		for time.Now().Before(targetTime) {
			// tight spin for microsecond-level characterization
		}
	}
	totalElapsed := time.Since(start)
	expectedElapsed := time.Duration(ticks) * targetInterval
	drift := totalElapsed - expectedElapsed

	t.Logf("Scheduler drift over %d ticks of %v: total=%v, expected=%v, drift=%v",
		ticks, targetInterval, totalElapsed, expectedElapsed, drift)
}
