package metrics

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/executor"
)

const (
	// MaxErrorSamples is the maximum number of distinct error sample messages retained per worker (§4, §9).
	MaxErrorSamples = 10

	// MaxRateLimitSamples is the maximum number of rate-limit header samples retained per worker (§4, §9).
	MaxRateLimitSamples = 10

	// MaxErrorMessageLength bounds error message string length to prevent memory inflation (§9).
	MaxErrorMessageLength = 256
)

// ErrorSample represents a bounded recorded error message and occurrence count (§4, §9).
type ErrorSample struct {
	Message string `json:"message"`
	Class   string `json:"class"`
	Count   int64  `json:"count"`
}

// RateLimitHeaderSample stores observed rate-limiting headers.
type RateLimitHeaderSample struct {
	Limit     string `json:"limit,omitempty"`
	Remaining string `json:"remaining,omitempty"`
	Reset     string `json:"reset,omitempty"`
	Policy    string `json:"policy,omitempty"`
}

// RateLimitObservations captures 429 throttling and rate-limit header observations (§9, §14).
type RateLimitObservations struct {
	Observed429Count  int64                   `json:"observed_429_count"`
	RetryAfterSamples []string                `json:"retry_after_samples,omitempty"`
	RateLimitHeaders  []RateLimitHeaderSample `json:"rate_limit_headers,omitempty"`
}

// WorkerMetrics maintains lock-free, worker-local metric accumulators for a single VU (§4, §9).
type WorkerMetrics struct {
	WorkerID int

	Planned   int64
	Scheduled int64
	Started   int64
	Completed int64
	Canceled  int64
	Dropped   int64

	Outcomes    map[core.Outcome]int64
	StatusCodes map[int]int64

	AllLatency     Histogram
	SuccessLatency Histogram

	TTFBSumMicroseconds int64
	TTFBCount           int64

	BytesSent     int64
	BytesReceived int64

	ErrorSamples []ErrorSample
	RateLimits   RateLimitObservations
}

// NewWorkerMetrics creates and initializes a new WorkerMetrics instance for the given worker ID.
func NewWorkerMetrics(workerID int) *WorkerMetrics {
	outcomes := make(map[core.Outcome]int64, len(core.AllOutcomes))
	for _, o := range core.AllOutcomes {
		outcomes[o] = 0
	}

	return &WorkerMetrics{
		WorkerID:       workerID,
		Outcomes:       outcomes,
		StatusCodes:    make(map[int]int64),
		AllLatency:     NewLatencyHistogram(),
		SuccessLatency: NewLatencyHistogram(),
		ErrorSamples:   make([]ErrorSample, 0, MaxErrorSamples),
		RateLimits: RateLimitObservations{
			RetryAfterSamples: make([]string, 0, MaxRateLimitSamples),
			RateLimitHeaders:  make([]RateLimitHeaderSample, 0, MaxRateLimitSamples),
		},
	}
}

// RecordResult records the outcome, latency, status code, bytes, and diagnostics of an execution result (§4, §8, §9).
// This method is lock-free and intended to be called exclusively from the owning VU goroutine.
func (w *WorkerMetrics) RecordResult(res *executor.Result) {
	if res == nil {
		return
	}

	// 1. Outcome tracking across canonical 12 states
	w.Outcomes[res.Outcome]++

	// 2. Status code distribution
	if res.StatusCode > 0 {
		w.StatusCodes[res.StatusCode]++
	}

	// 3. Latency recording in microseconds
	latencyUS := res.Latency.Microseconds()
	if latencyUS < 0 {
		latencyUS = 0
	}

	// Record latency for completed HTTP responses or valid outcome measurements
	_ = w.AllLatency.Record(latencyUS)
	if res.Outcome.IsSuccess() {
		_ = w.SuccessLatency.Record(latencyUS)
	}

	// 4. TTFB recording
	if res.TTFB > 0 {
		w.TTFBSumMicroseconds += res.TTFB.Microseconds()
		w.TTFBCount++
	}

	// 5. Byte tracking
	w.BytesSent += res.BytesSent
	w.BytesReceived += res.BytesReceived

	// 6. Error sample recording (strictly bounded)
	if res.Err != nil {
		w.recordErrorSample(res.Err.Error(), res.Outcome.String())
	} else if res.Outcome == core.OutcomeUnexpectedStatus {
		msg := fmt.Sprintf("unexpected HTTP status code %d", res.StatusCode)
		w.recordErrorSample(msg, res.Outcome.String())
	}

	// 7. Rate limit observation recording (strictly bounded)
	if res.StatusCode == http.StatusTooManyRequests || res.Outcome == core.OutcomeRateLimited {
		w.RateLimits.Observed429Count++
	}
	if res.RateLimitInfo != nil {
		w.recordRateLimitInfo(res.RateLimitInfo)
	}
}

func (w *WorkerMetrics) recordErrorSample(errMsg, class string) {
	// Normalize and truncate message length
	cleaned := strings.TrimSpace(errMsg)
	if len(cleaned) > MaxErrorMessageLength {
		cleaned = cleaned[:MaxErrorMessageLength] + "..."
	}

	// Check if already present
	for i := range w.ErrorSamples {
		if w.ErrorSamples[i].Message == cleaned && w.ErrorSamples[i].Class == class {
			w.ErrorSamples[i].Count++
			return
		}
	}

	// If under max capacity, append new sample
	if len(w.ErrorSamples) < MaxErrorSamples {
		w.ErrorSamples = append(w.ErrorSamples, ErrorSample{
			Message: cleaned,
			Class:   class,
			Count:   1,
		})
	}
}

func (w *WorkerMetrics) recordRateLimitInfo(info *executor.RateLimitInfo) {
	if info == nil {
		return
	}

	// Record Retry-After sample if present and under capacity
	if len(w.RateLimits.RetryAfterSamples) < MaxRateLimitSamples {
		if info.RetryAfterSeconds != nil {
			sample := fmt.Sprintf("%ds", *info.RetryAfterSeconds)
			if !containsString(w.RateLimits.RetryAfterSamples, sample) {
				w.RateLimits.RetryAfterSamples = append(w.RateLimits.RetryAfterSamples, sample)
			}
		} else if info.RetryAfterDate != nil {
			sample := info.RetryAfterDate.Format(http.TimeFormat)
			if !containsString(w.RateLimits.RetryAfterSamples, sample) {
				w.RateLimits.RetryAfterSamples = append(w.RateLimits.RetryAfterSamples, sample)
			}
		}
	}

	// Record RateLimit header sample if present and under capacity
	if len(w.RateLimits.RateLimitHeaders) < MaxRateLimitSamples {
		var sample RateLimitHeaderSample
		hasAny := false

		if info.Limit != nil {
			sample.Limit = fmt.Sprintf("%d", *info.Limit)
			hasAny = true
		}
		if info.Remaining != nil {
			sample.Remaining = fmt.Sprintf("%d", *info.Remaining)
			hasAny = true
		}
		if info.ResetSeconds != nil {
			sample.Reset = fmt.Sprintf("%ds", *info.ResetSeconds)
			hasAny = true
		} else if info.ResetDate != nil {
			sample.Reset = info.ResetDate.Format(http.TimeFormat)
			hasAny = true
		}
		if info.Policy != "" {
			sample.Policy = info.Policy
			hasAny = true
		}

		if hasAny && !containsHeaderSample(w.RateLimits.RateLimitHeaders, sample) {
			w.RateLimits.RateLimitHeaders = append(w.RateLimits.RateLimitHeaders, sample)
		}
	}
}

func containsString(slice []string, val string) bool {
	for _, s := range slice {
		if s == val {
			return true
		}
	}
	return false
}

func containsHeaderSample(slice []RateLimitHeaderSample, val RateLimitHeaderSample) bool {
	for _, s := range slice {
		if s.Limit == val.Limit && s.Remaining == val.Remaining && s.Reset == val.Reset && s.Policy == val.Policy {
			return true
		}
	}
	return false
}

// Snapshot returns an independent copy of the WorkerMetrics.
func (w *WorkerMetrics) Snapshot() *WorkerMetrics {
	outcomes := make(map[core.Outcome]int64, len(w.Outcomes))
	for k, v := range w.Outcomes {
		outcomes[k] = v
	}

	statusCodes := make(map[int]int64, len(w.StatusCodes))
	for k, v := range w.StatusCodes {
		statusCodes[k] = v
	}

	errorSamples := make([]ErrorSample, len(w.ErrorSamples))
	copy(errorSamples, w.ErrorSamples)

	retryAfterSamples := make([]string, len(w.RateLimits.RetryAfterSamples))
	copy(retryAfterSamples, w.RateLimits.RetryAfterSamples)

	rateLimitHeaders := make([]RateLimitHeaderSample, len(w.RateLimits.RateLimitHeaders))
	copy(rateLimitHeaders, w.RateLimits.RateLimitHeaders)

	return &WorkerMetrics{
		WorkerID:            w.WorkerID,
		Planned:             w.Planned,
		Scheduled:           w.Scheduled,
		Started:             w.Started,
		Completed:           w.Completed,
		Canceled:            w.Canceled,
		Dropped:             w.Dropped,
		Outcomes:            outcomes,
		StatusCodes:         statusCodes,
		AllLatency:          w.AllLatency.Copy(),
		SuccessLatency:      w.SuccessLatency.Copy(),
		TTFBSumMicroseconds: w.TTFBSumMicroseconds,
		TTFBCount:           w.TTFBCount,
		BytesSent:           w.BytesSent,
		BytesReceived:       w.BytesReceived,
		ErrorSamples:        errorSamples,
		RateLimits: RateLimitObservations{
			Observed429Count:  w.RateLimits.Observed429Count,
			RetryAfterSamples: retryAfterSamples,
			RateLimitHeaders:  rateLimitHeaders,
		},
	}
}
