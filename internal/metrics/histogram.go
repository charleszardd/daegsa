package metrics

import (
	"errors"
	"fmt"

	"github.com/HdrHistogram/hdrhistogram-go"
)

const (
	// MinLatencyMicroseconds is the minimum trackable latency value (1µs) (§3, §9).
	MinLatencyMicroseconds int64 = 1

	// MaxLatencyMicroseconds is the maximum trackable latency value (1 hour = 3,600,000,000µs) (§3, §9).
	MaxLatencyMicroseconds int64 = 3600 * 1000 * 1000

	// LatencySignificantFigures is the number of significant decimal digits of precision maintained (§3, §9).
	LatencySignificantFigures int = 3
)

var (
	// ErrNilHistogram indicates an attempt to merge a nil histogram.
	ErrNilHistogram = errors.New("cannot merge nil histogram")

	// ErrIncompatibleHistogram indicates an attempt to merge an incompatible histogram type.
	ErrIncompatibleHistogram = errors.New("incompatible histogram type")
)

// Histogram defines the bounded latency tracking abstraction spanning 1µs to 1h with 3 sig figs (§3, §9).
type Histogram interface {
	// Record records a latency value in microseconds. Values are bounded to [MinLatencyMicroseconds, MaxLatencyMicroseconds].
	Record(valueMicroseconds int64) error

	// ValueAtQuantile returns the value at the given percentile (q in [0.0, 100.0]) in microseconds.
	ValueAtQuantile(q float64) int64

	// Min returns the minimum recorded value in microseconds, or 0 if empty.
	Min() int64

	// Max returns the maximum recorded value in microseconds, or 0 if empty.
	Max() int64

	// Mean returns the arithmetic mean of all recorded values in microseconds, or 0.0 if empty.
	Mean() float64

	// Count returns the total number of recorded values.
	Count() int64

	// Merge combines recorded samples from another histogram into this one.
	Merge(other Histogram) error

	// Reset clears all recorded values while preserving bucket configuration.
	Reset()

	// Copy returns a deep copy of this histogram.
	Copy() Histogram
}

// HDRHistogram implements Histogram backed by hdrhistogram.Histogram.
type HDRHistogram struct {
	raw *hdrhistogram.Histogram
}

// NewLatencyHistogram constructs a bounded HDR histogram configured for 1µs to 1h with 3 significant figures.
func NewLatencyHistogram() *HDRHistogram {
	return &HDRHistogram{
		raw: hdrhistogram.New(MinLatencyMicroseconds, MaxLatencyMicroseconds, LatencySignificantFigures),
	}
}

// Record records a latency value in microseconds, clamping out-of-range values to configured bounds.
func (h *HDRHistogram) Record(valueMicroseconds int64) error {
	v := valueMicroseconds
	if v < MinLatencyMicroseconds {
		v = MinLatencyMicroseconds
	} else if v > MaxLatencyMicroseconds {
		v = MaxLatencyMicroseconds
	}
	return h.raw.RecordValue(v)
}

// ValueAtQuantile returns the value in microseconds at percentile q (0.0 to 100.0).
func (h *HDRHistogram) ValueAtQuantile(q float64) int64 {
	if h.raw.TotalCount() == 0 {
		return 0
	}
	return h.raw.ValueAtQuantile(q)
}

// Min returns the minimum recorded value in microseconds, or 0 if no samples have been recorded.
func (h *HDRHistogram) Min() int64 {
	if h.raw.TotalCount() == 0 {
		return 0
	}
	return h.raw.Min()
}

// Max returns the maximum recorded value in microseconds, or 0 if no samples have been recorded.
func (h *HDRHistogram) Max() int64 {
	if h.raw.TotalCount() == 0 {
		return 0
	}
	return h.raw.Max()
}

// Mean returns the arithmetic mean in microseconds, or 0.0 if no samples have been recorded.
func (h *HDRHistogram) Mean() float64 {
	if h.raw.TotalCount() == 0 {
		return 0.0
	}
	return h.raw.Mean()
}

// Count returns the total number of recorded samples.
func (h *HDRHistogram) Count() int64 {
	return h.raw.TotalCount()
}

// Merge merges another Histogram into this one.
func (h *HDRHistogram) Merge(other Histogram) error {
	if other == nil {
		return ErrNilHistogram
	}
	otherHDR, ok := other.(*HDRHistogram)
	if !ok {
		return fmt.Errorf("%w: expected *HDRHistogram, got %T", ErrIncompatibleHistogram, other)
	}
	if otherHDR.raw.TotalCount() == 0 {
		return nil
	}
	_ = h.raw.Merge(otherHDR.raw)
	return nil
}

// Reset clears all recorded data.
func (h *HDRHistogram) Reset() {
	h.raw.Reset()
}

// Copy returns an independent deep copy of the histogram.
func (h *HDRHistogram) Copy() Histogram {
	snapshot := h.raw.Export()
	imported := hdrhistogram.Import(snapshot)
	return &HDRHistogram{
		raw: imported,
	}
}
