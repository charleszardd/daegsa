package metrics

import (
	"math"
	"testing"
)

func TestHistogram_EmptyBehavior(t *testing.T) {
	h := NewLatencyHistogram()

	if h.Count() != 0 {
		t.Fatalf("expected count 0, got %d", h.Count())
	}
	if h.Min() != 0 {
		t.Errorf("expected min 0 for empty histogram, got %d", h.Min())
	}
	if h.Max() != 0 {
		t.Errorf("expected max 0 for empty histogram, got %d", h.Max())
	}
	if h.Mean() != 0.0 {
		t.Errorf("expected mean 0.0 for empty histogram, got %f", h.Mean())
	}
	if h.ValueAtQuantile(50.0) != 0 {
		t.Errorf("expected p50 0 for empty histogram, got %d", h.ValueAtQuantile(50.0))
	}
	if h.ValueAtQuantile(99.0) != 0 {
		t.Errorf("expected p99 0 for empty histogram, got %d", h.ValueAtQuantile(99.0))
	}
}

func TestHistogram_RecordAndQuantiles(t *testing.T) {
	h := NewLatencyHistogram()

	// Record 1,000 values linearly from 1,000µs (1ms) to 1,000,000µs (1s)
	for i := int64(1); i <= 1000; i++ {
		val := i * 1000 // in µs
		if err := h.Record(val); err != nil {
			t.Fatalf("failed to record value %d: %v", val, err)
		}
	}

	if h.Count() != 1000 {
		t.Fatalf("expected count 1000, got %d", h.Count())
	}

	// Check Min and Max with 3 sig figs tolerance
	if h.Min() < 990 || h.Min() > 1010 {
		t.Errorf("expected min ~1000µs, got %d", h.Min())
	}
	if h.Max() < 990000 || h.Max() > 1010000 {
		t.Errorf("expected max ~1000000µs, got %d", h.Max())
	}

	// Mean should be ~500,500µs
	if math.Abs(h.Mean()-500500.0) > 2000.0 {
		t.Errorf("expected mean ~500500µs, got %f", h.Mean())
	}

	// Check percentiles
	p50 := h.ValueAtQuantile(50.0)
	if p50 < 490000 || p50 > 510000 {
		t.Errorf("expected p50 ~500000µs, got %d", p50)
	}

	p90 := h.ValueAtQuantile(90.0)
	if p90 < 890000 || p90 > 910000 {
		t.Errorf("expected p90 ~900000µs, got %d", p90)
	}

	p99 := h.ValueAtQuantile(99.0)
	if p99 < 980000 || p99 > 1000000 {
		t.Errorf("expected p99 ~990000µs, got %d", p99)
	}
}

func TestHistogram_Clamping(t *testing.T) {
	h := NewLatencyHistogram()

	// Values below MinLatencyMicroseconds (1µs) or negative
	_ = h.Record(-50)
	_ = h.Record(0)
	// Values above MaxLatencyMicroseconds (3,600,000,000µs)
	_ = h.Record(MaxLatencyMicroseconds + 5000000)

	if h.Count() != 3 {
		t.Fatalf("expected count 3, got %d", h.Count())
	}

	if h.Min() != MinLatencyMicroseconds {
		t.Errorf("expected clamped min %d, got %d", MinLatencyMicroseconds, h.Min())
	}
	if h.Max() < MaxLatencyMicroseconds-1000 {
		t.Errorf("expected clamped max near %d, got %d", MaxLatencyMicroseconds, h.Max())
	}
}

func TestHistogram_Merge(t *testing.T) {
	h1 := NewLatencyHistogram()
	h2 := NewLatencyHistogram()

	// h1 gets 500 samples of 10ms (10,000µs)
	for i := 0; i < 500; i++ {
		_ = h1.Record(10000)
	}

	// h2 gets 500 samples of 20ms (20,000µs)
	for i := 0; i < 500; i++ {
		_ = h2.Record(20000)
	}

	if err := h1.Merge(h2); err != nil {
		t.Fatalf("unexpected merge error: %v", err)
	}

	if h1.Count() != 1000 {
		t.Fatalf("expected merged count 1000, got %d", h1.Count())
	}

	// Mean should be ~15,000µs
	if math.Abs(h1.Mean()-15000.0) > 100.0 {
		t.Errorf("expected merged mean ~15000µs, got %f", h1.Mean())
	}

	// Test merging nil
	if err := h1.Merge(nil); err != ErrNilHistogram {
		t.Errorf("expected ErrNilHistogram when merging nil, got %v", err)
	}
}

func TestHistogram_CopyAndReset(t *testing.T) {
	h := NewLatencyHistogram()
	_ = h.Record(5000)
	_ = h.Record(15000)

	copied := h.Copy()
	if copied.Count() != 2 {
		t.Fatalf("expected copied count 2, got %d", copied.Count())
	}

	h.Reset()
	if h.Count() != 0 {
		t.Fatalf("expected reset count 0, got %d", h.Count())
	}
	if copied.Count() != 2 {
		t.Fatalf("expected copied count still 2 after resetting original, got %d", copied.Count())
	}
}
