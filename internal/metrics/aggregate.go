package metrics

import (
	"fmt"
	"strconv"
	"time"

	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/threshold"
)

// RequestCounts aggregates request lifecycle counters (§9, §13).
type RequestCounts struct {
	Planned   int64 `json:"planned"`
	Scheduled int64 `json:"scheduled"`
	Started   int64 `json:"started"`
	Completed int64 `json:"completed"`
	Canceled  int64 `json:"canceled"`
	Dropped   int64 `json:"dropped"`
}

// LatencyPercentiles holds summary distribution metrics in milliseconds.
type LatencyPercentiles struct {
	MinMS  float64 `json:"min_ms"`
	MaxMS  float64 `json:"max_ms"`
	MeanMS float64 `json:"mean_ms"`
	P50MS  float64 `json:"p50_ms"`
	P90MS  float64 `json:"p90_ms"`
	P95MS  float64 `json:"p95_ms"`
	P99MS  float64 `json:"p99_ms"`
}

// LatencySummary separates metrics for all completed responses and expected-success responses.
type LatencySummary struct {
	AllCompleted    LatencyPercentiles `json:"all_completed"`
	ExpectedSuccess LatencyPercentiles `json:"expected_success"`
}

// AggregatedMetrics holds the centrally merged metrics from all workers (§4, §9, §13).
type AggregatedMetrics struct {
	RequestCounts       RequestCounts
	Outcomes            map[core.Outcome]int64
	StatusCodes         map[string]int64
	Latency             LatencySummary
	AllLatencyHist      Histogram
	SuccessLatencyHist  Histogram
	RateLimits          RateLimitObservations
	TotalBytesSent      int64
	TotalBytesReceived  int64
	Duration            time.Duration
	AchievedStartRPS    float64
	CompletedThroughput float64
	ErrorRate           float64
	RateLimitedRate     float64
	ErrorSamples        []ErrorSample
}

// MergeWorkers merges worker-local metric accumulators into a unified AggregatedMetrics snapshot (§4, §9, §13).
func MergeWorkers(workers []*WorkerMetrics, duration time.Duration) (*AggregatedMetrics, error) {
	allLatencyHist := NewLatencyHistogram()
	successLatencyHist := NewLatencyHistogram()

	var reqCounts RequestCounts
	outcomes := make(map[core.Outcome]int64, len(core.AllOutcomes))
	for _, o := range core.AllOutcomes {
		outcomes[o] = 0
	}
	statusCodes := make(map[string]int64)

	var totalBytesSent, totalBytesReceived int64
	var observed429Count int64
	var retryAfterSamples []string
	var rateLimitHeaders []RateLimitHeaderSample
	var mergedErrors []ErrorSample

	for _, w := range workers {
		if w == nil {
			continue
		}

		// Merge request counts
		reqCounts.Planned += w.Planned
		reqCounts.Scheduled += w.Scheduled
		reqCounts.Started += w.Started
		reqCounts.Completed += w.Completed
		reqCounts.Canceled += w.Canceled
		reqCounts.Dropped += w.Dropped

		// Merge outcomes
		for o, count := range w.Outcomes {
			outcomes[o] += count
		}

		// Merge status codes
		for code, count := range w.StatusCodes {
			codeStr := strconv.Itoa(code)
			statusCodes[codeStr] += count
		}

		// Merge latency histograms
		if err := allLatencyHist.Merge(w.AllLatency); err != nil {
			return nil, fmt.Errorf("failed to merge all-latency histogram: %w", err)
		}
		if err := successLatencyHist.Merge(w.SuccessLatency); err != nil {
			return nil, fmt.Errorf("failed to merge success-latency histogram: %w", err)
		}

		// Merge bytes
		totalBytesSent += w.BytesSent
		totalBytesReceived += w.BytesReceived

		// Merge rate limit observations
		observed429Count += w.RateLimits.Observed429Count
		for _, sample := range w.RateLimits.RetryAfterSamples {
			if len(retryAfterSamples) < MaxRateLimitSamples && !containsString(retryAfterSamples, sample) {
				retryAfterSamples = append(retryAfterSamples, sample)
			}
		}
		for _, hdr := range w.RateLimits.RateLimitHeaders {
			if len(rateLimitHeaders) < MaxRateLimitSamples && !containsHeaderSample(rateLimitHeaders, hdr) {
				rateLimitHeaders = append(rateLimitHeaders, hdr)
			}
		}

		// Merge error samples
		for _, errSample := range w.ErrorSamples {
			found := false
			for i := range mergedErrors {
				if mergedErrors[i].Message == errSample.Message && mergedErrors[i].Class == errSample.Class {
					mergedErrors[i].Count += errSample.Count
					found = true
					break
				}
			}
			if !found && len(mergedErrors) < MaxErrorSamples {
				mergedErrors = append(mergedErrors, errSample)
			}
		}
	}

	// Calculate latency percentiles
	latencySummary := LatencySummary{
		AllCompleted:    calculateLatencyPercentiles(allLatencyHist),
		ExpectedSuccess: calculateLatencyPercentiles(successLatencyHist),
	}

	// Calculate rates
	durSec := duration.Seconds()
	var startRPS, throughput, errorRate, rateLimitedRate float64
	if durSec > 0 {
		startRPS = float64(reqCounts.Started) / durSec
		throughput = float64(reqCounts.Completed) / durSec
	}

	if reqCounts.Completed > 0 {
		successCount := outcomes[core.OutcomeSuccess]
		nonSuccess := reqCounts.Completed - successCount
		if nonSuccess < 0 {
			nonSuccess = 0
		}
		errorRate = (float64(nonSuccess) / float64(reqCounts.Completed)) * 100.0

		rateLimitCount := outcomes[core.OutcomeRateLimited]
		if rateLimitCount == 0 {
			rateLimitCount = observed429Count
		}
		rateLimitedRate = (float64(rateLimitCount) / float64(reqCounts.Completed)) * 100.0
	}

	return &AggregatedMetrics{
		RequestCounts:       reqCounts,
		Outcomes:            outcomes,
		StatusCodes:         statusCodes,
		Latency:             latencySummary,
		AllLatencyHist:      allLatencyHist,
		SuccessLatencyHist:  successLatencyHist,
		RateLimits: RateLimitObservations{
			Observed429Count:  observed429Count,
			RetryAfterSamples: retryAfterSamples,
			RateLimitHeaders:  rateLimitHeaders,
		},
		TotalBytesSent:      totalBytesSent,
		TotalBytesReceived:  totalBytesReceived,
		Duration:            duration,
		AchievedStartRPS:    startRPS,
		CompletedThroughput: throughput,
		ErrorRate:           errorRate,
		RateLimitedRate:     rateLimitedRate,
		ErrorSamples:        mergedErrors,
	}, nil
}

func calculateLatencyPercentiles(h Histogram) LatencyPercentiles {
	if h.Count() == 0 {
		return LatencyPercentiles{
			MinMS:  0,
			MaxMS:  0,
			MeanMS: 0,
			P50MS:  0,
			P90MS:  0,
			P95MS:  0,
			P99MS:  0,
		}
	}

	return LatencyPercentiles{
		MinMS:  float64(h.Min()) / 1000.0,
		MaxMS:  float64(h.Max()) / 1000.0,
		MeanMS: h.Mean() / 1000.0,
		P50MS:  float64(h.ValueAtQuantile(50.0)) / 1000.0,
		P90MS:  float64(h.ValueAtQuantile(90.0)) / 1000.0,
		P95MS:  float64(h.ValueAtQuantile(95.0)) / 1000.0,
		P99MS:  float64(h.ValueAtQuantile(99.0)) / 1000.0,
	}
}

// ToThresholdSnapshot extracts a flat MetricsSnapshot from AggregatedMetrics for threshold evaluation (§9, §10).
func (a *AggregatedMetrics) ToThresholdSnapshot() threshold.MetricsSnapshot {
	if a == nil {
		return threshold.MetricsSnapshot{}
	}
	p999 := a.Latency.AllCompleted.P99MS
	if a.AllLatencyHist != nil && a.AllLatencyHist.Count() > 0 {
		p999 = float64(a.AllLatencyHist.ValueAtQuantile(99.9)) / 1000.0
	}
	return threshold.MetricsSnapshot{
		PlannedRequests:     a.RequestCounts.Planned,
		ScheduledRequests:   a.RequestCounts.Scheduled,
		StartedRequests:     a.RequestCounts.Started,
		CompletedRequests:   a.RequestCounts.Completed,
		CanceledRequests:    a.RequestCounts.Canceled,
		DroppedRequests:     a.RequestCounts.Dropped,
		SuccessfulRequests:  a.Outcomes[core.OutcomeSuccess],
		MinLatencyMS:        a.Latency.AllCompleted.MinMS,
		MaxLatencyMS:        a.Latency.AllCompleted.MaxMS,
		MeanLatencyMS:       a.Latency.AllCompleted.MeanMS,
		P50LatencyMS:        a.Latency.AllCompleted.P50MS,
		P90LatencyMS:        a.Latency.AllCompleted.P90MS,
		P95LatencyMS:        a.Latency.AllCompleted.P95MS,
		P99LatencyMS:        a.Latency.AllCompleted.P99MS,
		P999LatencyMS:       p999,
		AchievedStartRPS:    a.AchievedStartRPS,
		CompletedThroughput: a.CompletedThroughput,
		ErrorRate:           a.ErrorRate,
		RateLimitedRate:     a.RateLimitedRate,
	}
}
