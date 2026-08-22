package profile

import (
	"reflect"
	"testing"
	"time"
)

func TestCompileRampIsDeterministicAndPartitionsDurationExactly(t *testing.T) {
	source := []SourceSegment{{Name: "ramp", Stage: StageMeasured, Duration: 10*time.Nanosecond + 1, StartRate: 10, EndRate: 30, Steps: 3}}
	first, err := Compile(source, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	second, err := Compile(source, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first, second) {
		t.Fatal("identical profile input compiled differently")
	}
	if first.TotalDuration != source[0].Duration {
		t.Fatalf("duration = %v, want %v", first.TotalDuration, source[0].Duration)
	}
	wantRates := []float64{10, 20, 30}
	for i, want := range wantRates {
		if first.Segments[i].Rate != want {
			t.Errorf("segment %d rate = %v, want %v", i, first.Segments[i].Rate, want)
		}
	}
}

func TestCompileRejectsZeroInterval(t *testing.T) {
	_, err := Compile([]SourceSegment{{Name: "too-fast", Stage: StageMeasured, Duration: time.Second, Rate: float64(time.Second) + 1}}, time.Second)
	if err == nil {
		t.Fatal("expected zero-interval error")
	}
}

func TestCompileBoundsAndInvalidInputs(t *testing.T) {
	// Empty source segments
	if _, err := Compile(nil, time.Second); err == nil {
		t.Error("expected error on nil/empty source segments")
	}

	// Invalid time unit
	if _, err := Compile([]SourceSegment{{Name: "m", Stage: StageMeasured, Duration: time.Second, Rate: 10}}, 0); err == nil {
		t.Error("expected error on zero time unit")
	}
	if _, err := Compile([]SourceSegment{{Name: "m", Stage: StageMeasured, Duration: time.Second, Rate: 10}}, -time.Second); err == nil {
		t.Error("expected error on negative time unit")
	}

	// MaxSourceSegments exceeded
	manySource := make([]SourceSegment, MaxSourceSegments+1)
	for i := range manySource {
		manySource[i] = SourceSegment{Name: "seg", Stage: StageMeasured, Duration: time.Second, Rate: 10}
	}
	if _, err := Compile(manySource, time.Second); err == nil {
		t.Error("expected error when MaxSourceSegments is exceeded")
	}

	// MaxCompiledSegments exceeded
	rampWithTooManySteps := []SourceSegment{
		{Name: "big-ramp", Stage: StageMeasured, Duration: time.Hour, StartRate: 1, EndRate: 100, Steps: MaxCompiledSegments + 1},
	}
	if _, err := Compile(rampWithTooManySteps, time.Second); err == nil {
		t.Error("expected error when MaxCompiledSegments is exceeded")
	}

	// Invalid rate (<= 0)
	if _, err := Compile([]SourceSegment{{Name: "zero", Stage: StageMeasured, Duration: time.Second, Rate: 0}}, time.Second); err == nil {
		t.Error("expected error on zero rate")
	}
	if _, err := Compile([]SourceSegment{{Name: "neg", Stage: StageMeasured, Duration: time.Second, Rate: -5}}, time.Second); err == nil {
		t.Error("expected error on negative rate")
	}

	// Invalid duration (<= 0)
	if _, err := Compile([]SourceSegment{{Name: "zero-dur", Stage: StageMeasured, Duration: 0, Rate: 10}}, time.Second); err == nil {
		t.Error("expected error on zero duration")
	}
}

func TestCompileMultiSegmentOrderingAndOffsets(t *testing.T) {
	source := []SourceSegment{
		{Name: "warm", Stage: StageWarmup, Duration: 100 * time.Millisecond, Rate: 10},
		{Name: "ramp", Stage: StageMeasured, Duration: 200 * time.Millisecond, StartRate: 10, EndRate: 30, Steps: 2},
		{Name: "cool", Stage: StageCooldown, Duration: 50 * time.Millisecond, Rate: 5},
	}

	comp, err := Compile(source, time.Second)
	if err != nil {
		t.Fatalf("unexpected compilation error: %v", err)
	}

	if len(comp.Segments) != 4 {
		t.Fatalf("expected 4 compiled segments, got %d", len(comp.Segments))
	}
	if comp.TotalDuration != 350*time.Millisecond {
		t.Errorf("expected total duration 350ms, got %v", comp.TotalDuration)
	}
	if comp.PeakTargetRPS != 30.0 {
		t.Errorf("expected peak target RPS 30.0, got %v", comp.PeakTargetRPS)
	}

	// Verify segment properties and sequential offsets
	var expectedOffset time.Duration
	for i, seg := range comp.Segments {
		if seg.Index != i {
			t.Errorf("segment %d has index %d", i, seg.Index)
		}
		if seg.StartOffset != expectedOffset {
			t.Errorf("segment %d startOffset=%v, want %v", i, seg.StartOffset, expectedOffset)
		}
		expectedOffset += seg.Duration
		if seg.EndOffset != expectedOffset {
			t.Errorf("segment %d endOffset=%v, want %v", i, seg.EndOffset, expectedOffset)
		}
		if seg.IncludedMeasured != (seg.Stage == StageMeasured) {
			t.Errorf("segment %d IncludedMeasured=%v, want %v", i, seg.IncludedMeasured, seg.Stage == StageMeasured)
		}
	}
}
