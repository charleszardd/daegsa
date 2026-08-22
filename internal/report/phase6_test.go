package report

import (
	"strings"
	"testing"
	"time"

	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/metrics"
	"github.com/charleszardd/daegsa/internal/plan"
	"github.com/charleszardd/daegsa/internal/profile"
)

func TestBuildReportProfileUsesSchemaV2(t *testing.T) {
	segment := profile.Segment{Index: 0, Name: "measure", Stage: profile.StageMeasured, Duration: time.Second, DurationMS: 1000, EndOffset: time.Second, EndOffsetMS: 1000, Rate: 10, TargetRPS: 10, IncludedMeasured: true}
	worker := metrics.NewWorkerMetrics(0)
	worker.Planned = 10
	worker.Scheduled = 10
	worker.Started = 10
	worker.Completed = 10
	worker.Outcomes[core.OutcomeSuccess] = 10
	aggregate, err := metrics.MergeWorkers([]*metrics.WorkerMetrics{worker}, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	aggregate.Segments = []metrics.SegmentMetrics{{Segment: segment, Metrics: aggregate, Calibration: metrics.BuildCalibration(10, aggregate)}}
	aggregate.Measured = aggregate
	rep := BuildReport(&plan.Plan{SchemaVersion: 2, Model: core.WorkloadModelOpen, CompiledSegments: []profile.Segment{segment}}, aggregate, nil, time.Now(), time.Now().Add(time.Second), false, nil)
	if rep.ReportSchemaVersion != 2 || rep.MeasuredSummary == nil || len(rep.Segments) != 1 {
		t.Fatalf("unexpected v2 report: %+v", rep)
	}
}

func TestReportRateLimitRedaction(t *testing.T) {
	secret := "secret-api-key-value"
	p := &plan.Plan{
		SchemaVersion: 2,
		Model:         core.WorkloadModelOpen,
		KnownSecrets:  []string{secret},
	}

	agg := &metrics.AggregatedMetrics{
		RateLimits: metrics.RateLimitObservations{
			Observed429Count:  1,
			RetryAfterSamples: []string{"retry-after-" + secret},
			RateLimitHeaders: []metrics.RateLimitHeaderSample{
				{Policy: "policy-" + secret, Limit: "100-" + secret, Remaining: "50-" + secret, Reset: "60-" + secret},
			},
			HeaderConsistency: map[string]metrics.HeaderConsistency{
				"limit": {ObservedCount: 1, Samples: []string{"sample-" + secret}},
			},
		},
	}

	rep := BuildReport(p, agg, nil, time.Now(), time.Now().Add(time.Second), false, nil)

	// Verify redaction
	if rep.RateLimits.RetryAfterSamples[0] == "retry-after-"+secret {
		t.Error("RetryAfterSamples not redacted")
	}
	if rep.RateLimits.RateLimitHeaders[0].Policy == "policy-"+secret {
		t.Error("RateLimitHeaders Policy not redacted")
	}
	if rep.RateLimits.HeaderConsistency["limit"].Samples[0] == "sample-"+secret {
		t.Error("HeaderConsistency Samples not redacted")
	}
}

func TestTerminalReport_Phase6Sections(t *testing.T) {
	segment := profile.Segment{Index: 0, Name: "measure", Stage: profile.StageMeasured, Duration: time.Second, DurationMS: 1000, Rate: 10, TargetRPS: 10, IncludedMeasured: true}
	p := &plan.Plan{
		SchemaVersion:    2,
		Model:            core.WorkloadModelOpen,
		CompiledSegments: []profile.Segment{segment},
	}
	agg := &metrics.AggregatedMetrics{
		RequestCounts: metrics.RequestCounts{Started: 10, Completed: 10},
		Outcomes:      map[core.Outcome]int64{core.OutcomeSuccess: 10},
		RateLimits:    metrics.RateLimitObservations{Observed429Count: 0},
	}
	agg.Segments = []metrics.SegmentMetrics{{Segment: segment, Metrics: agg, Calibration: metrics.BuildCalibration(10, agg)}}
	agg.Measured = agg

	health := &metrics.GeneratorHealth{CPUAvailable: false}
	rep := BuildReport(p, agg, health, time.Now(), time.Now().Add(time.Second), false, nil)

	term := FormatTerminalReport(rep, p)

	// Verify profile section and caveat
	if !strings.Contains(term, "PROFILE SEGMENTS") {
		t.Error("missing PROFILE SEGMENTS section")
	}
	if !strings.Contains(term, "No throttling observed at tested rates; this is not a guaranteed safe production limit.") {
		t.Error("missing no-throttling caveat in terminal output")
	}
	if !strings.Contains(term, "Max CPU:           unavailable") {
		t.Error("expected Max CPU: unavailable when CPUAvailable=false")
	}
}
