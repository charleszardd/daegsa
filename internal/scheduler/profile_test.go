package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/charleszardd/daegsa/internal/clock"
	"github.com/charleszardd/daegsa/internal/executor"
	"github.com/charleszardd/daegsa/internal/profile"
	"github.com/charleszardd/daegsa/internal/testtarget"
)

func TestOpenSchedulerProfileReconcilesSegmentsAndMeasuredSummary(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()
	plan := createOpenTestPlan(server.URL(), 20, time.Second, 10, 300*time.Millisecond, time.Second)
	plan.SchemaVersion = 2
	plan.CompiledSegments = []profile.Segment{
		{Index: 0, Name: "warm", Stage: profile.StageWarmup, Duration: 100 * time.Millisecond, EndOffset: 100 * time.Millisecond, Rate: 20, TargetRPS: 20},
		{Index: 1, Name: "measure", Stage: profile.StageMeasured, StartOffset: 100 * time.Millisecond, EndOffset: 200 * time.Millisecond, Duration: 100 * time.Millisecond, Rate: 20, TargetRPS: 20, IncludedMeasured: true},
		{Index: 2, Name: "cool", Stage: profile.StageCooldown, StartOffset: 200 * time.Millisecond, EndOffset: 300 * time.Millisecond, Duration: 100 * time.Millisecond, Rate: 20, TargetRPS: 20},
	}
	exec, err := executor.NewHTTPExecutor(plan)
	if err != nil {
		t.Fatal(err)
	}
	defer exec.Close()
	scheduler, err := NewOpenScheduler(plan, exec, clock.NewRealClock())
	if err != nil {
		t.Fatal(err)
	}
	aggregate, _, err := scheduler.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	var planned, completed int64
	for _, segment := range aggregate.Segments {
		planned += segment.Metrics.RequestCounts.Planned
		completed += segment.Metrics.RequestCounts.Completed
	}
	if aggregate.RequestCounts.Planned != planned || aggregate.RequestCounts.Completed != completed {
		t.Fatalf("root=%+v segment planned=%d completed=%d", aggregate.RequestCounts, planned, completed)
	}
	if aggregate.Measured == nil || aggregate.Measured.RequestCounts.Planned != aggregate.Segments[1].Metrics.RequestCounts.Planned {
		t.Fatal("measured summary did not reconcile")
	}
}

func TestOpenScheduler_Segment429AttributionAndFirstThrottle(t *testing.T) {
	// Server rate-limits everything when ?status=429
	server := testtarget.NewServer()
	defer server.Close()

	plan := createOpenTestPlan(server.URL()+"/?status=429", 20, time.Second, 10, 200*time.Millisecond, time.Second)
	plan.SchemaVersion = 2
	plan.CompiledSegments = []profile.Segment{
		{Index: 0, Name: "warm", Stage: profile.StageWarmup, Duration: 100 * time.Millisecond, EndOffset: 100 * time.Millisecond, Rate: 20, TargetRPS: 20},
		{Index: 1, Name: "measure", Stage: profile.StageMeasured, StartOffset: 100 * time.Millisecond, EndOffset: 200 * time.Millisecond, Duration: 100 * time.Millisecond, Rate: 20, TargetRPS: 20, IncludedMeasured: true},
	}

	exec, err := executor.NewHTTPExecutor(plan)
	if err != nil {
		t.Fatal(err)
	}
	defer exec.Close()

	scheduler, err := NewOpenScheduler(plan, exec, clock.NewRealClock())
	if err != nil {
		t.Fatal(err)
	}

	aggregate, _, err := scheduler.Run(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	// Verify 429 counts recorded per segment and in root aggregate
	var total429 int64
	for _, seg := range aggregate.Segments {
		total429 += seg.Metrics.RateLimits.Observed429Count
		if seg.Metrics.RateLimits.Observed429Count == 0 {
			t.Errorf("expected 429s in segment %s, got 0", seg.Segment.Name)
		}
	}

	if aggregate.RateLimits.Observed429Count != total429 || total429 == 0 {
		t.Fatalf("expected matching 429 counts: root=%d sum=%d", aggregate.RateLimits.Observed429Count, total429)
	}

	// Verify first throttle offset was captured (>= 0)
	if aggregate.Segments[0].Metrics.FirstThrottleOffsetNS < 0 {
		t.Error("expected valid FirstThrottleOffsetNS in segment 0")
	}
}
