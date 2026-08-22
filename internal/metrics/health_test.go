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

	sampler.RecordCPUSample(92.5)                     // > 85%
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

func TestBuildCalibration_Diagnostics(t *testing.T) {
	// 1. Healthy calibration
	aggHealthy := &AggregatedMetrics{
		AchievedStartRPS:    100.0,
		CompletedThroughput: 100.0,
		RequestCounts:       RequestCounts{Planned: 100, Started: 100, Completed: 100},
		SchedulerLagMaxMS:   5.0,
	}
	calHealthy := BuildCalibration(100.0, aggHealthy)
	if !calHealthy.Reliable || len(calHealthy.Warnings) != 0 {
		t.Errorf("expected reliable calibration without warnings, got reliable=%v warnings=%v", calHealthy.Reliable, calHealthy.Warnings)
	}

	// 2. Low achieved start rate (<95%)
	aggLowRate := &AggregatedMetrics{
		AchievedStartRPS:    90.0, // 90/100 = 0.90 < 0.95
		CompletedThroughput: 90.0,
		RequestCounts:       RequestCounts{Planned: 100, Started: 90, Completed: 90},
	}
	calLowRate := BuildCalibration(100.0, aggLowRate)
	if calLowRate.Reliable || len(calLowRate.Warnings) == 0 {
		t.Error("expected unreliable calibration for low achieved start rate")
	}

	// 3. Scheduler lag (>50ms)
	aggLag := &AggregatedMetrics{
		AchievedStartRPS:    100.0,
		CompletedThroughput: 100.0,
		RequestCounts:       RequestCounts{Planned: 100, Started: 100, Completed: 100},
		SchedulerLagMaxMS:   60.0,
	}
	calLag := BuildCalibration(100.0, aggLag)
	if calLag.Reliable || len(calLag.Warnings) == 0 {
		t.Error("expected unreliable calibration for high scheduler lag")
	}

	// 4. Dropped requests due to max_in_flight
	aggDropped := &AggregatedMetrics{
		AchievedStartRPS:    100.0,
		CompletedThroughput: 80.0,
		RequestCounts:       RequestCounts{Planned: 100, Started: 80, Dropped: 20},
	}
	calDropped := BuildCalibration(100.0, aggDropped)
	if calDropped.Reliable || len(calDropped.Warnings) == 0 {
		t.Error("expected unreliable calibration when requests are dropped")
	}
}

func TestGeneratorHealth_CPUAvailableFlag(t *testing.T) {
	sampler := NewGeneratorHealthSampler(clock.NewRealClock())
	health := sampler.Collect()
	if health.CPUAvailable {
		t.Error("expected CPUAvailable to be false before any CPU samples recorded")
	}

	sampler.RecordCPUSample(25.0)
	healthWithSample := sampler.Collect()
	if !healthWithSample.CPUAvailable {
		t.Error("expected CPUAvailable to be true after recording CPU sample")
	}
}
