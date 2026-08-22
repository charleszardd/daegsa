package scheduler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"
	"time"

	"github.com/charleszardd/daegsa/internal/clock"
	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/executor"
	"github.com/charleszardd/daegsa/internal/metrics"
	"github.com/charleszardd/daegsa/internal/plan"
	"github.com/charleszardd/daegsa/internal/profile"
)

var (
	ErrInvalidRate        = errors.New("open workload requires rate > 0")
	ErrInvalidTimeUnit    = errors.New("open workload requires time_unit > 0")
	ErrInvalidMaxInFlight = errors.New("open workload requires max_in_flight > 0")
)

const (
	dispatcherWorkerID        = -1
	warningMaxInFlightReached = "max_in_flight reached, dropped requests"
	warningTargetDegradation  = "target degradation or low max_in_flight caused dropped requests"
	defaultOpenGracefulStop   = 5 * time.Second
)

type dispatchJob struct {
	ctx             context.Context
	intendedTime    time.Time
	actualTime      time.Time
	segmentIndex    int
	plannedOffsetNS int64
}

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
	segments          []profile.Segment
}

func NewOpenScheduler(p *plan.Plan, exec *executor.HTTPExecutor, schedulerClock clock.Clock) (*OpenScheduler, error) {
	if p == nil {
		return nil, ErrInvalidPlan
	}
	if exec == nil {
		return nil, ErrInvalidExecutor
	}
	if p.Model != core.WorkloadModelOpen {
		return nil, fmt.Errorf("%w: got %s", ErrIncompatibleModel, p.Model)
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
	segments := append([]profile.Segment(nil), p.CompiledSegments...)
	if len(segments) == 0 {
		if p.Rate <= 0 {
			return nil, ErrInvalidRate
		}
		segments = []profile.Segment{{Index: 0, Name: "measured", Stage: profile.StageMeasured, Duration: p.Duration, DurationMS: p.Duration.Milliseconds(), EndOffset: p.Duration, EndOffsetMS: p.Duration.Milliseconds(), Rate: p.Rate, TargetRPS: p.Rate / p.TimeUnit.Seconds(), IncludedMeasured: true}}
	}
	if schedulerClock == nil {
		schedulerClock = clock.NewRealClock()
	}
	exec.SetClock(schedulerClock)
	workers := make([]*metrics.WorkerMetrics, int(p.MaxInFlight))
	for workerID := range workers {
		workers[workerID] = metrics.NewWorkerMetrics(workerID)
	}
	return &OpenScheduler{plan: p, executor: exec, clock: schedulerClock, healthSampler: metrics.NewGeneratorHealthSampler(schedulerClock), stateMachine: core.NewLifecycleStateMachine(), dispatcherMetrics: metrics.NewWorkerMetrics(dispatcherWorkerID), workerPool: workers, segments: segments}, nil
}

func (s *OpenScheduler) InFlight() int64                     { return s.inFlightCount.Load() }
func (s *OpenScheduler) PeakInFlight() int64                 { return s.peakInFlight.Load() }
func (s *OpenScheduler) LifecycleState() core.LifecycleState { return s.stateMachine.Current() }

func (s *OpenScheduler) Run(ctx context.Context) (*metrics.AggregatedMetrics, *metrics.GeneratorHealth, error) {
	if err := s.enterStage(s.segments[0].Stage); err != nil {
		return nil, nil, fmt.Errorf("failed to start scheduler lifecycle: %w", err)
	}
	s.healthSampler.Start()
	defer s.healthSampler.Stop()

	startTime := s.clock.Now()
	workerContext, cancelWorkers := context.WithCancel(ctx)
	defer cancelWorkers()
	dispatchChannel := make(chan *dispatchJob, len(s.workerPool))
	segmentCollector := metrics.NewSegmentCollector(len(s.segments))
	segmentDispatcher := make([]*metrics.WorkerMetrics, len(s.segments))
	for index := range segmentDispatcher {
		segmentDispatcher[index] = metrics.NewWorkerMetrics(dispatcherWorkerID)
	}

	var workers sync.WaitGroup
	for workerID, workerMetrics := range s.workerPool {
		workers.Add(1)
		go s.runWorker(workerContext, workerID, workerMetrics, dispatchChannel, segmentCollector, startTime, &workers)
	}

	hardCanceled := false
	previousStage := s.segments[0].Stage
	for _, segment := range s.segments {
		if segment.Stage != previousStage {
			if err := s.enterStage(segment.Stage); err != nil {
				cancelWorkers()
				close(dispatchChannel)
				workers.Wait()
				return nil, nil, err
			}
			previousStage = segment.Stage
		}
		if s.scheduleSegment(ctx, workerContext, startTime, segment, dispatchChannel, segmentDispatcher[segment.Index]) {
			hardCanceled = true
			break
		}
	}
	close(dispatchChannel)
	if hardCanceled {
		_ = s.stateMachine.TransitionTo(core.StateCanceled)
	} else {
		_ = s.stateMachine.TransitionTo(core.StateGracefulStop)
	}

	workersDone := make(chan struct{})
	go func() { workers.Wait(); close(workersDone) }()
	gracefulStop := s.plan.GracefulStop
	if gracefulStop <= 0 {
		gracefulStop = defaultOpenGracefulStop
	}
	graceTimer := s.clock.NewTimer(gracefulStop)
	defer graceTimer.Stop()
	if hardCanceled {
		cancelWorkers()
		<-workersDone
	} else {
		select {
		case <-workersDone:
		case <-graceTimer.C():
			cancelWorkers()
			<-workersDone
		case <-ctx.Done():
			cancelWorkers()
			<-workersDone
			_ = s.stateMachine.TransitionTo(core.StateCanceled)
		}
	}
	_ = s.stateMachine.TransitionTo(core.StateCompleted)

	elapsed := s.clock.Since(startTime)
	allWorkers := append([]*metrics.WorkerMetrics{s.dispatcherMetrics}, s.workerPool...)
	aggregate, err := metrics.MergeWorkers(allWorkers, elapsed)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to merge worker metrics: %w", err)
	}
	workerSegments, err := segmentCollector.Snapshots()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to merge segment metrics: %w", err)
	}

	segmentResults := make([]metrics.SegmentMetrics, len(s.segments))
	measuredAggregates := make([]*metrics.AggregatedMetrics, 0, len(s.segments))
	var measuredDuration time.Duration
	for index, segment := range s.segments {
		segmentAggregate, mergeErr := metrics.MergeWorkers([]*metrics.WorkerMetrics{segmentDispatcher[index], workerSegments[index]}, segment.Duration)
		if mergeErr != nil {
			return nil, nil, mergeErr
		}
		segmentResults[index] = metrics.SegmentMetrics{Segment: segment, Metrics: segmentAggregate, Calibration: metrics.BuildCalibration(segment.TargetRPS, segmentAggregate)}
		if segment.IncludedMeasured {
			measuredAggregates = append(measuredAggregates, segmentAggregate)
			measuredDuration += segment.Duration
		}
	}
	measured, err := metrics.MergeAggregates(measuredAggregates, measuredDuration)
	if err != nil {
		return nil, nil, err
	}
	aggregate.Segments = segmentResults
	aggregate.Measured = measured
	if s.dispatcherMetrics.Dropped > 0 {
		s.healthSampler.AddWarning(warningTargetDegradation)
	}
	health := s.healthSampler.Collect()
	return aggregate, &health, nil
}

func (s *OpenScheduler) scheduleSegment(ctx, workerContext context.Context, startTime time.Time, segment profile.Segment, jobs chan<- *dispatchJob, dispatcher *metrics.WorkerMetrics) bool {
	segmentStart := startTime.Add(segment.StartOffset)
	segmentEnd := startTime.Add(segment.EndOffset)
	interval := time.Duration(float64(s.plan.TimeUnit) / segment.Rate)
	nextTargetTick := segmentStart
	for {
		now := s.clock.Now()
		if !now.Before(segmentEnd) {
			return false
		}
		if ctx.Err() != nil {
			return true
		}
		if waitDuration := nextTargetTick.Sub(now); waitDuration > 0 {
			timer := s.clock.NewTimer(waitDuration)
			select {
			case <-timer.C():
			case <-ctx.Done():
				timer.Stop()
				return true
			}
			timer.Stop()
		}
		actualTime := s.clock.Now()
		if !actualTime.Before(segmentEnd) {
			return false
		}
		intendedTime := nextTargetTick
		lag := actualTime.Sub(intendedTime)
		if lag > 0 {
			s.healthSampler.RecordSchedulerLag(lag)
			lagMS := float64(lag.Microseconds()) / 1000
			if lagMS > dispatcher.SchedulerLagMaxMS {
				dispatcher.SchedulerLagMaxMS = lagMS
			}
		}
		if lag > interval {
			nextTargetTick = actualTime.Add(interval)
		} else {
			nextTargetTick = nextTargetTick.Add(interval)
		}
		s.dispatcherMetrics.Planned++
		dispatcher.Planned++
		if s.inFlightCount.Load() >= s.plan.MaxInFlight {
			s.dispatcherMetrics.Dropped++
			s.dispatcherMetrics.Outcomes[core.OutcomeDropped]++
			dispatcher.Dropped++
			dispatcher.Outcomes[core.OutcomeDropped]++
			s.healthSampler.AddWarning(warningMaxInFlightReached)
			continue
		}
		s.dispatcherMetrics.Scheduled++
		dispatcher.Scheduled++
		s.inFlightCount.Add(1)
		for {
			peak, current := s.peakInFlight.Load(), s.inFlightCount.Load()
			if current <= peak || s.peakInFlight.CompareAndSwap(peak, current) {
				break
			}
		}
		jobs <- &dispatchJob{ctx: workerContext, intendedTime: intendedTime, actualTime: actualTime, segmentIndex: segment.Index, plannedOffsetNS: intendedTime.Sub(startTime).Nanoseconds()}
	}
}

func (s *OpenScheduler) runWorker(ctx context.Context, workerID int, overall *metrics.WorkerMetrics, jobs <-chan *dispatchJob, collector *metrics.SegmentCollector, startTime time.Time, workers *sync.WaitGroup) {
	defer workers.Done()
	currentSegment := -1
	var current *metrics.WorkerMetrics
	flush := func() {
		if current != nil {
			collector.Flush(currentSegment, current)
		}
	}
	defer flush()
	for job := range jobs {
		if job.segmentIndex != currentSegment {
			flush()
			currentSegment = job.segmentIndex
			current = metrics.NewWorkerMetrics(workerID)
		}
		overall.Started++
		current.Started++
		result, executionErr := s.executor.ExecuteRequest(job.ctx, workerID)
		s.inFlightCount.Add(-1)
		if result != nil {
			if result.Outcome == core.OutcomeCanceled {
				overall.Canceled++
				current.Canceled++
			} else {
				overall.Completed++
				current.Completed++
			}
			overall.RecordResult(result)
			current.RecordResult(result)
			if result.StatusCode == http.StatusTooManyRequests && (current.FirstThrottleOffsetNS < 0 || job.plannedOffsetNS < current.FirstThrottleOffsetNS) {
				current.FirstThrottleOffsetNS = job.plannedOffsetNS
			}
		} else if executionErr != nil {
			overall.Canceled++
			current.Canceled++
		}
	}
}

func (s *OpenScheduler) enterStage(stage string) error {
	switch stage {
	case profile.StageWarmup:
		return s.stateMachine.TransitionTo(core.StateWarmup)
	case profile.StageMeasured:
		return s.stateMachine.TransitionTo(core.StateRunning)
	case profile.StageCooldown:
		return s.stateMachine.TransitionTo(core.StateCooldown)
	default:
		return fmt.Errorf("unknown profile stage %q", stage)
	}
}
