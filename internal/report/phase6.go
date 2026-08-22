package report

import (
	"time"

	"github.com/charleszardd/daegsa/internal/config"
	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/metrics"
	"github.com/charleszardd/daegsa/internal/plan"
	"github.com/charleszardd/daegsa/internal/profile"
)

// MetricsSummary is the stable report-facing projection of aggregated metrics.
type MetricsSummary struct {
	RequestCounts       RequestCounts          `json:"request_counts"`
	Outcomes            map[core.Outcome]int64 `json:"outcomes"`
	StatusCodes         map[string]int64       `json:"status_codes"`
	Latency             LatencySummary         `json:"latency"`
	RateLimits          RateLimitObservations  `json:"rate_limits"`
	AchievedStartRPS    float64                `json:"achieved_start_rate"`
	CompletedThroughput float64                `json:"completed_throughput"`
	ErrorRate           float64                `json:"error_rate"`
	RateLimitedRate     float64                `json:"rate_limited_rate"`
}

type SegmentReport struct {
	Segment     profile.Segment     `json:"segment"`
	Metrics     MetricsSummary      `json:"metrics"`
	Calibration metrics.Calibration `json:"calibration"`
}

type FirstThrottleObservation struct {
	SegmentIndex    int     `json:"segment_index"`
	SegmentName     string  `json:"segment_name"`
	Stage           string  `json:"stage"`
	TargetRPS       float64 `json:"target_rps"`
	PlannedOffsetMS int64   `json:"planned_offset_ms"`
}

func metricsSummary(aggregate *metrics.AggregatedMetrics, p *plan.Plan) *MetricsSummary {
	if aggregate == nil {
		return nil
	}
	summary := &MetricsSummary{
		RequestCounts: aggregate.RequestCounts, Outcomes: aggregate.Outcomes, StatusCodes: aggregate.StatusCodes,
		Latency: aggregate.Latency, RateLimits: aggregate.RateLimits,
		AchievedStartRPS: aggregate.AchievedStartRPS, CompletedThroughput: aggregate.CompletedThroughput,
		ErrorRate: aggregate.ErrorRate, RateLimitedRate: aggregate.RateLimitedRate,
	}
	redactRateLimitObservations(&summary.RateLimits, p)
	return summary
}

func buildPhase6Report(rep *Report, p *plan.Plan, aggregate *metrics.AggregatedMetrics) {
	if rep == nil || p == nil || aggregate == nil || p.SchemaVersion < config.ExpectedSchemaVersion {
		return
	}
	rep.ReportSchemaVersion = ProfileReportSchemaVersion
	rep.CompiledSegments = append([]profile.Segment(nil), p.CompiledSegments...)
	rep.Segments = make([]SegmentReport, 0, len(aggregate.Segments))
	var first *FirstThrottleObservation
	var weightedTarget, measuredSeconds float64
	for _, segmentMetrics := range aggregate.Segments {
		summary := metricsSummary(segmentMetrics.Metrics, p)
		rep.Segments = append(rep.Segments, SegmentReport{Segment: segmentMetrics.Segment, Metrics: *summary, Calibration: segmentMetrics.Calibration})
		if segmentMetrics.Segment.IncludedMeasured {
			seconds := segmentMetrics.Segment.Duration.Seconds()
			weightedTarget += segmentMetrics.Segment.TargetRPS * seconds
			measuredSeconds += seconds
		}
		if segmentMetrics.Metrics != nil && segmentMetrics.Metrics.FirstThrottleOffsetNS >= 0 {
			candidate := &FirstThrottleObservation{SegmentIndex: segmentMetrics.Segment.Index, SegmentName: segmentMetrics.Segment.Name, Stage: segmentMetrics.Segment.Stage, TargetRPS: segmentMetrics.Segment.TargetRPS, PlannedOffsetMS: time.Duration(segmentMetrics.Metrics.FirstThrottleOffsetNS).Milliseconds()}
			if first == nil || candidate.PlannedOffsetMS < first.PlannedOffsetMS {
				first = candidate
			}
		}
	}
	rep.MeasuredSummary = metricsSummary(aggregate.Measured, p)
	rep.FirstThrottle = first
	if measuredSeconds > 0 {
		weightedTarget /= measuredSeconds
	}
	calibration := metrics.BuildCalibration(weightedTarget, aggregate.Measured)
	rep.Calibration = &calibration
}

func redactRateLimitObservations(observations *RateLimitObservations, p *plan.Plan) {
	if observations == nil || p == nil {
		return
	}
	for index, sample := range observations.RetryAfterSamples {
		observations.RetryAfterSamples[index] = config.RedactString(sample, p.KnownSecrets)
	}
	for name, consistency := range observations.HeaderConsistency {
		for index, sample := range consistency.Samples {
			consistency.Samples[index] = config.RedactString(sample, p.KnownSecrets)
		}
		observations.HeaderConsistency[name] = consistency
	}
	for index := range observations.RateLimitHeaders {
		header := &observations.RateLimitHeaders[index]
		header.Limit = config.RedactString(header.Limit, p.KnownSecrets)
		header.Remaining = config.RedactString(header.Remaining, p.KnownSecrets)
		header.Reset = config.RedactString(header.Reset, p.KnownSecrets)
		header.Policy = config.RedactString(header.Policy, p.KnownSecrets)
	}
}
