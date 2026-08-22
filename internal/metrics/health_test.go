package metrics

import (
	"testing"
	"time"

	"github.com/charleszardd/daegsa/internal/clock"
)

func TestGeneratorHealth_SamplingAndLifecycle(t *testing.T) {
	mockClock := clock.NewControllableClock(time.Now())
	sampler := NewGeneratorHealthSampler(mockClock)

	sampler.Start()
	// Advance clock to trigger sample tick
	mockClock.Advance(300 * time.Millisecond)

	sampler.RecordSchedulerLag(10 * time.Millisecond)
	sampler.RecordCPUSample(45.0)

	sampler.Stop()

	health := sampler.Collect()

	if health.GoroutinesPeak <= 0 {
		t.Errorf("expected positive goroutines peak, got %d", health.GoroutinesPeak)
	}
	if health.CPUMaxPercent != 45.0 {
		t.Errorf("expected CPU 45.0%%, got %f", health.CPUMaxPercent)
	}
	if health.SchedulerLagMaxMS != 10.0 {
		t.Errorf("expected scheduler lag 10.0ms, got %f", health.SchedulerLagMaxMS)
	}
	if len(health.SaturationWarnings) != 0 {
		t.Errorf("expected no saturation warnings under thresholds, got %v", health.SaturationWarnings)
	}
}

func TestGeneratorHealth_SaturationWarnings(t *testing.T) {
	sampler := NewGeneratorHealthSampler(clock.NewRealClock())

	sampler.RecordCPUSample(92.5)                  // > 85%
	sampler.RecordSchedulerLag(75 * time.Millisecond) // > 50ms

	health := sampler.Collect()

	if len(health.SaturationWarnings) != 2 {
		t.Fatalf("expected 2 saturation warnings, got %d: %v", len(health.SaturationWarnings), health.SaturationWarnings)
	}

	hasCPUWarning := false
	hasLagWarning := false
	for _, w := range health.SaturationWarnings {
		if w == "client CPU saturation detected (> 85%)" {
			hasCPUWarning = true
		}
		if w == "scheduler lag exceeded 50ms" {
			hasLagWarning = true
		}
	}

	if !hasCPUWarning {
		t.Errorf("missing CPU saturation warning")
	}
	if !hasLagWarning {
		t.Errorf("missing scheduler lag warning")
	}
}
