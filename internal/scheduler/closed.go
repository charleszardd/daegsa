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
	// ErrInvalidPlan indicates a nil or malformed execution plan.
	ErrInvalidPlan = errors.New("invalid execution plan")

	// ErrInvalidExecutor indicates a nil HTTP executor.
	ErrInvalidExecutor = errors.New("invalid http executor")

	// ErrIncompatibleModel indicates a plan workload model other than closed.
	ErrIncompatibleModel = errors.New("closed scheduler requires closed workload model")

	// ErrZeroUsers indicates a closed workload configuration with 0 users.
	ErrZeroUsers = errors.New("closed workload requires users > 0")

	// ErrZeroDuration indicates a workload duration of 0 or negative.
	ErrZeroDuration = errors.New("workload duration must be > 0")
)

// ClosedScheduler coordinates concurrent Virtual User (VU) loops in a closed workload model (§2, §4, §7).
type ClosedScheduler struct {
	plan          *plan.Plan
	executor      *executor.HTTPExecutor
	clock         clock.Clock
	healthSampler *metrics.GeneratorHealthSampler
	stateMachine  *core.LifecycleStateMachine
	workers       []*metrics.WorkerMetrics
	inFlightCount atomic.Int64
}

// NewClosedScheduler constructs and validates a ClosedScheduler for the given plan and executor (§4, §7).
func NewClosedScheduler(p *plan.Plan, exec *executor.HTTPExecutor, clk clock.Clock) (*ClosedScheduler, error) {
	if p == nil {
		return nil, ErrInvalidPlan
	}
	if exec == nil {
		return nil, ErrInvalidExecutor
	}
	if p.Model != core.WorkloadModelClosed {
		return nil, fmt.Errorf("%w: got %s", ErrIncompatibleModel, p.Model)
	}
	if p.Users <= 0 {
		return nil, ErrZeroUsers
	}
	if p.Duration <= 0 {
		return nil, ErrZeroDuration
	}

	if clk == nil {
		clk = clock.NewRealClock()
	}
	exec.SetClock(clk)

	workers := make([]*metrics.WorkerMetrics, p.Users)
	for i := 0; i < int(p.Users); i++ {
		workers[i] = metrics.NewWorkerMetrics(i)
	}

	return &ClosedScheduler{
		plan:          p,
		executor:      exec,
		clock:         clk,
		healthSampler: metrics.NewGeneratorHealthSampler(clk),
		stateMachine:  core.NewLifecycleStateMachine(),
		workers:       workers,
	}, nil
}

// InFlight returns the number of requests currently in flight across all virtual users (§9).
func (s *ClosedScheduler) InFlight() int64 {
	return s.inFlightCount.Load()
}

// LifecycleState returns the current execution state of the scheduler (§7).
func (s *ClosedScheduler) LifecycleState() core.LifecycleState {
	return s.stateMachine.Current()
}

// Run executes the closed workload lifecycle: running N concurrent VUs, honoring think time,
// stopping iterations at duration expiry, draining in-flight requests, and returning aggregated metrics (§7).
func (s *ClosedScheduler) Run(ctx context.Context) (*metrics.AggregatedMetrics, *metrics.GeneratorHealth, error) {
	if err := s.stateMachine.TransitionTo(core.StateRunning); err != nil {
		return nil, nil, fmt.Errorf("failed to start scheduler lifecycle: %w", err)
	}

	s.healthSampler.Start()
	defer s.healthSampler.Stop()

	startTime := s.clock.Now()
	workerCtx, cancelWorkers := context.WithCancel(ctx)
	defer cancelWorkers()

	stopNewRequests := make(chan struct{})
	var wg sync.WaitGroup

	// Launch N concurrent Virtual User worker goroutines
	for i := 0; i < int(s.plan.Users); i++ {
		wg.Add(1)
		go s.runVU(workerCtx, i, s.workers[i], stopNewRequests, &wg)
	}

	// Schedule duration timer
	durationTimer := s.clock.NewTimer(s.plan.Duration)
	defer durationTimer.Stop()

	var hardCanceled bool
	select {
	case <-durationTimer.C():
		// Planned duration elapsed normally
	case <-ctx.Done():
		hardCanceled = true
	}

	// Stop scheduling new iterations
	close(stopNewRequests)

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

	elapsed := s.clock.Since(startTime)
	agg, err := metrics.MergeWorkers(s.workers, elapsed)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to merge worker metrics: %w", err)
	}

	health := s.healthSampler.Collect()
	return agg, &health, nil
}

func (s *ClosedScheduler) runVU(
	ctx context.Context,
	workerID int,
	wm *metrics.WorkerMetrics,
	stopNewRequests <-chan struct{},
	wg *sync.WaitGroup,
) {
	defer wg.Done()

	for {
		// Check if stop was signaled before starting next request
		select {
		case <-stopNewRequests:
			return
		case <-ctx.Done():
			return
		default:
		}

		// Account for planned and started work
		wm.Planned++
		wm.Started++
		s.inFlightCount.Add(1)

		// Execute HTTP request
		res, err := s.executor.ExecuteRequest(ctx, workerID)
		s.inFlightCount.Add(-1)

		if res != nil {
			wm.Completed++
			wm.RecordResult(res)
		} else if err != nil {
			wm.Canceled++
		}

		// Check if stop was signaled before waiting think time
		select {
		case <-stopNewRequests:
			return
		case <-ctx.Done():
			return
		default:
		}

		// Honor think time between iterations
		if s.plan.ThinkTime > 0 {
			thinkTimer := s.clock.NewTimer(s.plan.ThinkTime)
			select {
			case <-thinkTimer.C():
			case <-stopNewRequests:
				thinkTimer.Stop()
				return
			case <-ctx.Done():
				thinkTimer.Stop()
				return
			}
		}
	}
}
