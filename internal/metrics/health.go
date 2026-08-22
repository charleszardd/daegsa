package metrics

import (
	"runtime"
	"sync"
	"time"

	"github.com/charleszardd/daegsa/internal/clock"
)

const (
	// DefaultHealthSampleInterval is the default interval between resource health samples (§9, §14).
	DefaultHealthSampleInterval = 250 * time.Millisecond

	// CPUSaturationThresholdPercent is the CPU threshold above which saturation warnings are issued (§14).
	CPUSaturationThresholdPercent = 85.0

	// SchedulerLagWarningThresholdMS is the scheduler drift threshold triggering warnings (§14).
	SchedulerLagWarningThresholdMS = 50.0

	// HighGoroutineThreshold is the goroutine count triggering high concurrency warnings (§14).
	HighGoroutineThreshold int64 = 10000
)

// GeneratorHealth records load generator resource metrics to distinguish client saturation (§13, §14).
type GeneratorHealth struct {
	CPUMaxPercent      float64  `json:"cpu_max_percent"`
	GoroutinesPeak     int64    `json:"goroutines_peak"`
	SchedulerLagMaxMS  float64  `json:"scheduler_lag_max_ms"`
	SaturationWarnings []string `json:"saturation_warnings,omitempty"`
}

// GeneratorHealthSampler collects periodic diagnostics to distinguish client generator saturation (§9, §13, §14).
type GeneratorHealthSampler struct {
	mu                sync.Mutex
	goroutinesPeak    int64
	cpuMaxPercent     float64
	schedulerLagMaxMS float64
	warnings          []string
	sampleInterval    time.Duration
	clock             clock.Clock
	stopChan          chan struct{}
	doneChan          chan struct{}
	running           bool
}

// NewGeneratorHealthSampler creates a new GeneratorHealthSampler backed by the given clock.
func NewGeneratorHealthSampler(clk clock.Clock) *GeneratorHealthSampler {
	if clk == nil {
		clk = clock.NewRealClock()
	}
	return &GeneratorHealthSampler{
		sampleInterval: DefaultHealthSampleInterval,
		clock:          clk,
		stopChan:       make(chan struct{}),
		doneChan:       make(chan struct{}),
		warnings:       make([]string, 0),
	}
}

// Start begins background periodic resource health sampling.
func (s *GeneratorHealthSampler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.stopChan = make(chan struct{})
	s.doneChan = make(chan struct{})
	s.mu.Unlock()

	go s.sampleLoop()
}

func (s *GeneratorHealthSampler) sampleLoop() {
	defer close(s.doneChan)

	ticker := s.clock.NewTicker(s.sampleInterval)
	defer ticker.Stop()

	// Initial sample
	s.sample()

	for {
		select {
		case <-s.stopChan:
			s.sample()
			return
		case <-ticker.C():
			s.sample()
		}
	}
}

func (s *GeneratorHealthSampler) sample() {
	currentGoroutines := int64(runtime.NumGoroutine())

	s.mu.Lock()
	defer s.mu.Unlock()

	if currentGoroutines > s.goroutinesPeak {
		s.goroutinesPeak = currentGoroutines
	}
}

// Stop terminates background sampling and waits for the sampling goroutine to finish.
func (s *GeneratorHealthSampler) Stop() {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return
	}
	s.running = false
	close(s.stopChan)
	s.mu.Unlock()

	<-s.doneChan
}

// RecordSchedulerLag records an observed scheduler timing drift.
func (s *GeneratorHealthSampler) RecordSchedulerLag(lag time.Duration) {
	lagMS := float64(lag.Microseconds()) / 1000.0
	if lagMS < 0 {
		lagMS = 0
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if lagMS > s.schedulerLagMaxMS {
		s.schedulerLagMaxMS = lagMS
	}
}

// RecordCPUSample records an observed CPU utilization percentage.
func (s *GeneratorHealthSampler) RecordCPUSample(percent float64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if percent > s.cpuMaxPercent {
		s.cpuMaxPercent = percent
	}
}

// AddWarning appends a custom diagnostic warning message.
func (s *GeneratorHealthSampler) AddWarning(warning string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, w := range s.warnings {
		if w == warning {
			return
		}
	}
	s.warnings = append(s.warnings, warning)
}

// Collect returns the aggregated generator health snapshot and saturation warnings (§13, §14).
func (s *GeneratorHealthSampler) Collect() GeneratorHealth {
	s.mu.Lock()
	defer s.mu.Unlock()

	warnings := make([]string, len(s.warnings))
	copy(warnings, s.warnings)

	if s.cpuMaxPercent > CPUSaturationThresholdPercent {
		warnings = appendIfMissing(warnings, "client CPU saturation detected (> 85%)")
	}
	if s.goroutinesPeak > HighGoroutineThreshold {
		warnings = appendIfMissing(warnings, "high goroutine count detected")
	}
	if s.schedulerLagMaxMS > SchedulerLagWarningThresholdMS {
		warnings = appendIfMissing(warnings, "scheduler lag exceeded 50ms")
	}

	return GeneratorHealth{
		CPUMaxPercent:      s.cpuMaxPercent,
		GoroutinesPeak:     s.goroutinesPeak,
		SchedulerLagMaxMS:  s.schedulerLagMaxMS,
		SaturationWarnings: warnings,
	}
}

func appendIfMissing(slice []string, val string) []string {
	for _, s := range slice {
		if s == val {
			return slice
		}
	}
	return append(slice, val)
}
