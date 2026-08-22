package metrics

import (
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/profile"
)

const MinimumReliableAchievedRatio = 0.95

// Calibration describes whether generated load is reliable enough for capacity interpretation.
type Calibration struct {
	TargetRPS           float64  `json:"target_rps"`
	AchievedStartRPS    float64  `json:"achieved_start_rate"`
	CompletedThroughput float64  `json:"completed_throughput"`
	AchievedTargetRatio float64  `json:"achieved_target_ratio"`
	DroppedCount        int64    `json:"dropped_count"`
	DroppedRate         float64  `json:"dropped_rate"`
	SchedulerLagMaxMS   float64  `json:"scheduler_lag_max_ms"`
	Reliable            bool     `json:"reliable"`
	Warnings            []string `json:"warnings,omitempty"`
}

// SegmentMetrics associates one compiled profile segment with its reconciled metrics.
type SegmentMetrics struct {
	Segment     profile.Segment    `json:"segment"`
	Metrics     *AggregatedMetrics `json:"metrics"`
	Calibration Calibration        `json:"calibration"`
}

// SegmentCollector accepts worker-local segment flushes. Workers hold one current
// segment accumulator, so total histogram memory remains O(workers + segments).
type SegmentCollector struct {
	mu           sync.Mutex
	accumulators []*WorkerMetrics
	err          error
}

func NewSegmentCollector(segmentCount int) *SegmentCollector {
	accumulators := make([]*WorkerMetrics, segmentCount)
	for index := range accumulators {
		accumulators[index] = NewWorkerMetrics(index)
	}
	return &SegmentCollector{accumulators: accumulators}
}

func (collector *SegmentCollector) Flush(segmentIndex int, source *WorkerMetrics) {
	if collector == nil || source == nil {
		return
	}
	collector.mu.Lock()
	defer collector.mu.Unlock()
	if collector.err != nil {
		return
	}
	if segmentIndex < 0 || segmentIndex >= len(collector.accumulators) {
		collector.err = fmt.Errorf("segment index %d is out of bounds", segmentIndex)
		return
	}
	collector.err = mergeWorkerMetrics(collector.accumulators[segmentIndex], source)
}

func (collector *SegmentCollector) Snapshots() ([]*WorkerMetrics, error) {
	collector.mu.Lock()
	defer collector.mu.Unlock()
	if collector.err != nil {
		return nil, collector.err
	}
	result := make([]*WorkerMetrics, len(collector.accumulators))
	for index, accumulator := range collector.accumulators {
		result[index] = accumulator.Snapshot()
	}
	return result, nil
}

func mergeWorkerMetrics(destination, source *WorkerMetrics) error {
	destination.Planned += source.Planned
	destination.Scheduled += source.Scheduled
	destination.Started += source.Started
	destination.Completed += source.Completed
	destination.Canceled += source.Canceled
	destination.Dropped += source.Dropped
	for outcome, count := range source.Outcomes {
		destination.Outcomes[outcome] += count
	}
	for status, count := range source.StatusCodes {
		destination.StatusCodes[status] += count
	}
	if err := destination.AllLatency.Merge(source.AllLatency); err != nil {
		return err
	}
	if err := destination.SuccessLatency.Merge(source.SuccessLatency); err != nil {
		return err
	}
	destination.BytesSent += source.BytesSent
	destination.BytesReceived += source.BytesReceived
	destination.RateLimits.Observed429Count += source.RateLimits.Observed429Count
	if destination.RateLimits.HeaderConsistency == nil {
		destination.RateLimits.HeaderConsistency = make(map[string]HeaderConsistency)
	}
	mergeHeaderConsistency(destination.RateLimits.HeaderConsistency, source.RateLimits.HeaderConsistency)
	for _, sample := range source.RateLimits.RetryAfterSamples {
		if len(destination.RateLimits.RetryAfterSamples) < MaxRateLimitSamples && !containsString(destination.RateLimits.RetryAfterSamples, sample) {
			destination.RateLimits.RetryAfterSamples = append(destination.RateLimits.RetryAfterSamples, sample)
		}
	}
	for _, sample := range source.RateLimits.RateLimitHeaders {
		if len(destination.RateLimits.RateLimitHeaders) < MaxRateLimitSamples && !containsHeaderSample(destination.RateLimits.RateLimitHeaders, sample) {
			destination.RateLimits.RateLimitHeaders = append(destination.RateLimits.RateLimitHeaders, sample)
		}
	}
	for _, sample := range source.ErrorSamples {
		destination.recordErrorSample(sample.Message, sample.Class)
	}
	if source.SchedulerLagMaxMS > destination.SchedulerLagMaxMS {
		destination.SchedulerLagMaxMS = source.SchedulerLagMaxMS
	}
	if source.FirstThrottleOffsetNS >= 0 && (destination.FirstThrottleOffsetNS < 0 || source.FirstThrottleOffsetNS < destination.FirstThrottleOffsetNS) {
		destination.FirstThrottleOffsetNS = source.FirstThrottleOffsetNS
	}
	return nil
}

// MergeAggregates merges already-reconciled segment summaries by merging their histograms and counters.
func MergeAggregates(inputs []*AggregatedMetrics, duration time.Duration) (*AggregatedMetrics, error) {
	workers := make([]*WorkerMetrics, 0, len(inputs))
	for index, aggregate := range inputs {
		if aggregate == nil {
			continue
		}
		worker := NewWorkerMetrics(index)
		worker.Planned = aggregate.RequestCounts.Planned
		worker.Scheduled = aggregate.RequestCounts.Scheduled
		worker.Started = aggregate.RequestCounts.Started
		worker.Completed = aggregate.RequestCounts.Completed
		worker.Canceled = aggregate.RequestCounts.Canceled
		worker.Dropped = aggregate.RequestCounts.Dropped
		for outcome, count := range aggregate.Outcomes {
			worker.Outcomes[outcome] = count
		}
		for status, count := range aggregate.StatusCodes {
			code, err := strconv.Atoi(status)
			if err != nil {
				return nil, err
			}
			worker.StatusCodes[code] = count
		}
		worker.AllLatency = aggregate.AllLatencyHist.Copy()
		worker.SuccessLatency = aggregate.SuccessLatencyHist.Copy()
		worker.BytesSent = aggregate.TotalBytesSent
		worker.BytesReceived = aggregate.TotalBytesReceived
		worker.RateLimits = aggregate.RateLimits
		worker.ErrorSamples = append([]ErrorSample(nil), aggregate.ErrorSamples...)
		worker.SchedulerLagMaxMS = aggregate.SchedulerLagMaxMS
		worker.FirstThrottleOffsetNS = aggregate.FirstThrottleOffsetNS
		workers = append(workers, worker)
	}
	return MergeWorkers(workers, duration)
}

func BuildCalibration(targetRPS float64, aggregate *AggregatedMetrics) Calibration {
	calibration := Calibration{TargetRPS: targetRPS, Reliable: true}
	if aggregate == nil {
		return calibration
	}
	calibration.AchievedStartRPS = aggregate.AchievedStartRPS
	calibration.CompletedThroughput = aggregate.CompletedThroughput
	calibration.DroppedCount = aggregate.RequestCounts.Dropped
	calibration.SchedulerLagMaxMS = aggregate.SchedulerLagMaxMS
	if targetRPS > 0 {
		calibration.AchievedTargetRatio = aggregate.AchievedStartRPS / targetRPS
	}
	if aggregate.RequestCounts.Planned > 0 {
		calibration.DroppedRate = float64(aggregate.RequestCounts.Dropped) / float64(aggregate.RequestCounts.Planned) * 100
	}
	if calibration.AchievedTargetRatio < MinimumReliableAchievedRatio {
		calibration.Reliable = false
		calibration.Warnings = append(calibration.Warnings, "achieved start rate was below 95% of target")
	}
	if aggregate.SchedulerLagMaxMS > SchedulerLagWarningThresholdMS {
		calibration.Reliable = false
		calibration.Warnings = append(calibration.Warnings, "scheduler lag exceeded the reliability threshold")
	}
	if aggregate.RequestCounts.Dropped > 0 {
		calibration.Reliable = false
		calibration.Warnings = append(calibration.Warnings, "max_in_flight caused dropped requests")
	}
	return calibration
}

func emptyOutcomes() map[core.Outcome]int64 {
	outcomes := make(map[core.Outcome]int64, len(core.AllOutcomes))
	for _, outcome := range core.AllOutcomes {
		outcomes[outcome] = 0
	}
	return outcomes
}
