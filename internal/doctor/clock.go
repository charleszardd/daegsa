package doctor

import (
	"context"
	"fmt"
	"time"
)

const (
	clockSampleIterations = 500
	targetSleepDuration   = 2 * time.Millisecond
	warnTimerResolution   = 5 * time.Millisecond
	warnSleepDeviation    = 8 * time.Millisecond
)

// CheckClockPrecision evaluates OS monotonic timer resolution and sleep accuracy (§14).
func CheckClockPrecision(ctx context.Context) CheckResult {
	start := time.Now()

	// 1. Monotonic timer resolution measurement
	var minDelta time.Duration
	prev := time.Now()
	for i := 0; i < clockSampleIterations; i++ {
		if ctx.Err() != nil {
			return CheckResult{
				Name:     "Timer & Clock Precision",
				Category: CategoryClock,
				Status:   StatusFail,
				Summary:  "Clock check canceled",
				Detail:   ctx.Err().Error(),
				Duration: time.Since(start),
			}
		}
		curr := time.Now()
		delta := curr.Sub(prev)
		if delta > 0 && (minDelta == 0 || delta < minDelta) {
			minDelta = delta
		}
		prev = curr
	}

	// 2. Sleep accuracy measurement
	sleepStart := time.Now()
	time.Sleep(targetSleepDuration)
	sleepObserved := time.Since(sleepStart)
	sleepDeviation := sleepObserved - targetSleepDuration
	if sleepDeviation < 0 {
		sleepDeviation = -sleepDeviation
	}

	elapsed := time.Since(start)

	detail := fmt.Sprintf("Observed timer resolution: %v; Sleep deviation: %v (target: %v, actual: %v)",
		minDelta, sleepDeviation, targetSleepDuration, sleepObserved)

	// Evaluate status
	if minDelta > warnTimerResolution || sleepDeviation > warnSleepDeviation {
		return CheckResult{
			Name:       "Timer & Clock Precision",
			Category:   CategoryClock,
			Status:     StatusWarn,
			Summary:    fmt.Sprintf("Coarse timer resolution (%v) or sleep jitter (%v)", minDelta, sleepDeviation),
			Detail:     detail,
			Suggestion: "OS timer resolution is coarse. On Windows, ensure Windows timer resolution tuning or run tests with --arrival-rate pacing that accommodates ~15ms timer quanta.",
			Duration:   elapsed,
		}
	}

	return CheckResult{
		Name:     "Timer & Clock Precision",
		Category: CategoryClock,
		Status:   StatusPass,
		Summary:  fmt.Sprintf("High-resolution timer (<%v) and accurate sleep pacing", minDelta.Truncate(time.Microsecond)),
		Detail:   detail,
		Duration: elapsed,
	}
}
