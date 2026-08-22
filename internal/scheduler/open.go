package scheduler

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charleszardd/daegsa/internal/clock"
	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/executor"
	"github.com/charleszardd/daegsa/internal/metrics"
	"github.com/charleszardd/daegsa/internal/plan"
)

var (
	// ErrInvalidRate indicates an open workload configuration with rate <= 0.
	ErrInvalidRate = errors.New("open workload requires rate > 0")

	// ErrInvalidTimeUnit indicates an open workload configuration with time_unit <= 0.
	ErrInvalidTimeUnit = errors.New("open workload requires time_unit > 0")

	// ErrInvalidMaxInFlight indicates an open workload configuration with max_in_flight <= 0.
	ErrInvalidMaxInFlight = errors.New("open workload requires max_in_flight > 0")
)

const (
	// dispatcherWorkerID is the dedicated worker ID for scheduler-level planned/dropped accounting.
	dispatcherWorkerID = -1

	// warningMaxInFlightReached is emitted when max_in_flight capacity is exceeded.
	warningMaxInFlightReached = "max_in_flight reached, dropped requests"

	// warningTargetDegradation is emitted when drops occur due to target slow-down or low concurrency ceiling.
	warningTargetDegradation = "target degradation or low max_in_flight caused dropped requests"
)

type dispatchJob struct {
	ctx          context.Context
	intendedTime time.Time
	actualTime   time.Time
}

// OpenScheduler coordinates open arrival-rate workload generation with precision pacing (§2, §4, §7).
type OpenScheduler struct {
	plan              *plan.Plan
	executor          *executor.HTTPExecutor
	clock             clock.Clock
	healthSampler     *metrics.GeneratorHealthSampler
	stateMachine      *core.LifecycleStateMachine
	inFlightCount     atomic.Int64
	peakInFlight      atomic.Int64
	dispatcherMetrics *metrics.WorkerMetrics
	workerPool        []*metrics.WorkerMetrics
}

// NewOpenScheduler constructs and validates an OpenScheduler for the given plan and executor (§4, §7).
func NewOpenScheduler(p *plan.Plan, exec *executor.HTTPExecutor, clk clock.Clock) (*OpenScheduler, error) {
	if p == nil {
		return nil, ErrInvalidPlan
	}
	if exec == nil {
		return nil, ErrInvalidExecutor
	}
	if p.Model != core.WorkloadModelOpen {
		return nil, fmt.Errorf("%w: got %s", ErrIncompatibleModel, p.Model)
	}
	if p.Rate <= 0 {
		return nil, ErrInvalidRate
	}
	if p.TimeUnit <= 0 {
		return nil, ErrInvalidTimeUnit
	}
	if p.MaxInFlight <= 0 {
		return nil, ErrInvalidMaxInFlight
	}
	if p.Duration <= 0 {
		return nil, ErrZeroDuration
	}

	if clk == nil {
		clk = clock.NewRealClock()
	}
	exec.SetClock(clk)

	workerPoolSize := int(p.MaxInFlight)
	workerPool := make([]*metrics.WorkerMetrics, workerPoolSize)
	for i := 0; i < workerPoolSize; i++ {
		workerPool[i] = metrics.NewWorkerMetrics(i)
	}

	return &OpenScheduler{
		plan:              p,
		executor:          exec,
		clock:             clk,
		healthSampler:     metrics.NewGeneratorHealthSampler(clk),
		stateMachine:      core.NewLifecycleStateMachine(),
		dispatcherMetrics: metrics.NewWorkerMetrics(dispatcherWorkerID),
		workerPool:        workerPool,
	}, nil
}

// InFlight returns the number of requests currently actively executing (§9).
func (s *OpenScheduler) InFlight() int64 {
	return s.inFlightCount.Load()
}

// PeakInFlight returns the peak concurrent in-flight requests observed (§9).
func (s *OpenScheduler) PeakInFlight() int64 {
	return s.peakInFlight.Load()
}

// LifecycleState returns the current execution state of the scheduler (§7).
func (s *OpenScheduler) LifecycleState() core.LifecycleState {
	return s.stateMachine.Current()
}

// Run executes the open arrival-rate workload lifecycle: pacing dispatches, enforcing max_in_flight drops,
// tracking scheduler lag, preventing catch-up bursts, draining workers gracefully, and returning aggregated metrics (§7).
func (s *OpenScheduler) Run(ctx context.Context) (*metrics.AggregatedMetrics, *metrics.GeneratorHealth, error) {
	if err := s.stateMachine.TransitionTo(core.StateRunning); err != nil {
		return nil, nil, fmt.Errorf("failed to start scheduler lifecycle: %w", err)
	}

	s.healthSampler.Start()
	defer s.healthSampler.Stop()

	startTime := s.clock.Now()
	endTime := startTime.Add(s.plan.Duration)
	workerCtx, cancelWorkers := context.WithCancel(ctx)
	defer cancelWorkers()

	workerPoolSize := len(s.workerPool)
	dispatchChan := make(chan *dispatchJob, workerPoolSize)
	var wg sync.WaitGroup

	// Launch worker pool
	for i := 0; i < workerPoolSize; i++ {
		wg.Add(1)
		go s.runWorker(workerCtx, i, s.workerPool[i], dispatchChan, &wg)
	}

	interval := time.Duration(float64(s.plan.TimeUnit) / s.plan.Rate)
	nextTargetTick := startTime
	var hardCanceled bool

	// Arrival pacing loop
	for {
		now := s.clock.Now()
		if !now.Before(endTime) {
			break
		}
		if ctx.Err() != nil {
			hardCanceled = true
			break
		}

		waitDur := nextTargetTick.Sub(now)
		if waitDur > 0 {
			timer := s.clock.NewTimer(waitDur)
			select {
			case <-timer.C():
			case <-ctx.Done():
				timer.Stop()
				hardCanceled = true
			}
			timer.Stop()
			if hardCanceled || ctx.Err() != nil {
				hardCanceled = true
				break
			}
		}

		actualTime := s.clock.Now()
		if !actualTime.Before(endTime) {
			break
		}

		intendedTime := nextTargetTick
		lag := actualTime.Sub(intendedTime)
		if lag > 0 {
			s.healthSampler.RecordSchedulerLag(lag)
		}

		// Anti-catch-up burst progression: if lag > interval, advance next target tick from actualTime
		if lag > interval {
			nextTargetTick = actualTime.Add(interval)
		} else {
			nextTargetTick = nextTargetTick.Add(interval)
		}

		// Account for planned tick
		s.dispatcherMetrics.Planned++

		// Strict max_in_flight limit enforcement: atomic concurrency tracking
		currentInFlight := s.inFlightCount.Load()
		if currentInFlight >= s.plan.MaxInFlight {
			s.dispatcherMetrics.Dropped++
			s.dispatcherMetrics.Outcomes[core.OutcomeDropped]++
			s.healthSampler.AddWarning(warningMaxInFlightReached)
			continue
		}

		s.dispatcherMetrics.Scheduled++
		s.inFlightCount.Add(1)
		for {
			peak := s.peakInFlight.Load()
			cur := s.inFlightCount.Load()
			if cur <= peak || s.peakInFlight.CompareAndSwap(peak, cur) {
				break
			}
		}

		dispatchChan <- &dispatchJob{
			ctx:          workerCtx,
			intendedTime: intendedTime,
			actualTime:   actualTime,
		}
	}

	// Stop scheduling new requests
	close(dispatchChan)

	// Transition lifecycle state
	if hardCanceled {
		_ = s.stateMachine.TransitionTo(core.StateCanceled)
	} else {
		_ = s.stateMachine.TransitionTo(core.StateGracefulStop)
	}

	// Track worker completion
	workersDone := make(chan struct{})
	go func() {
		wg.Wait()
		close(workersDone)
	}()

	// Graceful drain handling
	gracefulTimeout := s.plan.GracefulStop
	if gracefulTimeout <= 0 {
		gracefulTimeout = 5 * time.Second
	}
	graceTimer := s.clock.NewTimer(gracefulTimeout)
	defer graceTimer.Stop()

	if hardCanceled {
		cancelWorkers()
		<-workersDone
	} else {
		select {
		case <-workersDone:
			// Clean drain within graceful stop window
		case <-graceTimer.C():
			// Graceful stop timeout expired -> force cancel remaining in-flight requests
			cancelWorkers()
			<-workersDone
		case <-ctx.Done():
			// Hard cancellation while draining
			cancelWorkers()
			<-workersDone
			_ = s.stateMachine.TransitionTo(core.StateCanceled)
		}
	}

	_ = s.stateMachine.TransitionTo(core.StateCompleted)

	// Metrics finalization and aggregation
	elapsed := s.clock.Since(startTime)
	allWorkers := make([]*metrics.WorkerMetrics, 0, len(s.workerPool)+1)
	allWorkers = append(allWorkers, s.dispatcherMetrics)
	allWorkers = append(allWorkers, s.workerPool...)

	agg, err := metrics.MergeWorkers(allWorkers, elapsed)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to merge worker metrics: %w", err)
	}

	if s.dispatcherMetrics.Dropped > 0 {
		s.healthSampler.AddWarning(warningTargetDegradation)
	}

	health := s.healthSampler.Collect()
	return agg, &health, nil
}

func (s *OpenScheduler) runWorker(
	ctx context.Context,
	workerID int,
	wm *metrics.WorkerMetrics,
	dispatchChan <-chan *dispatchJob,
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	for job := range dispatchChan {
		wm.Started++

		res, err := s.executor.ExecuteRequest(job.ctx)
		s.inFlightCount.Add(-1)

		if res != nil {
			if res.Outcome == core.OutcomeCanceled {
				wm.Canceled++
			} else {
				wm.Completed++
			}
			wm.RecordResult(res)
		} else if err != nil {
			wm.Canceled++
		}
	}
}
