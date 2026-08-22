package profile

import (
	"errors"
	"fmt"
	"math"
	"time"
)

var ErrInvalidProfile = errors.New("invalid load profile")

// Compile expands validated source segments into constant-rate scheduling segments.
func Compile(source []SourceSegment, timeUnit time.Duration) (*Compilation, error) {
	if len(source) == 0 || len(source) > MaxSourceSegments {
		return nil, fmt.Errorf("%w: source segment count must be between 1 and %d", ErrInvalidProfile, MaxSourceSegments)
	}
	if timeUnit <= 0 {
		return nil, fmt.Errorf("%w: time unit must be positive", ErrInvalidProfile)
	}

	compiled := make([]Segment, 0, len(source))
	var offset time.Duration
	var peakTargetRPS float64
	for _, segment := range source {
		steps := segment.Steps
		if steps == 0 {
			steps = 1
		}
		if len(compiled)+steps > MaxCompiledSegments {
			return nil, fmt.Errorf("%w: compiled segment count exceeds %d", ErrInvalidProfile, MaxCompiledSegments)
		}
		baseDuration := segment.Duration / time.Duration(steps)
		remainder := segment.Duration % time.Duration(steps)
		for step := 0; step < steps; step++ {
			duration := baseDuration
			if time.Duration(step) < remainder {
				duration++
			}
			rate := segment.Rate
			if steps > 1 {
				rate = segment.StartRate + (segment.EndRate-segment.StartRate)*float64(step)/float64(steps-1)
			}
			targetRPS := rate / timeUnit.Seconds()
			if rate <= 0 || math.IsNaN(rate) || math.IsInf(rate, 0) || targetRPS <= 0 {
				return nil, fmt.Errorf("%w: segment %q has invalid rate", ErrInvalidProfile, segment.Name)
			}
			interval := time.Duration(float64(timeUnit) / rate)
			if interval <= 0 {
				return nil, fmt.Errorf("%w: segment %q rate produces a zero scheduling interval", ErrInvalidProfile, segment.Name)
			}
			if duration <= 0 || offset > time.Duration(math.MaxInt64)-duration {
				return nil, fmt.Errorf("%w: segment %q has invalid or overflowing duration", ErrInvalidProfile, segment.Name)
			}
			endOffset := offset + duration
			name := segment.Name
			if steps > 1 {
				name = fmt.Sprintf("%s-%d", segment.Name, step+1)
			}
			compiled = append(compiled, Segment{
				Index:            len(compiled),
				Name:             name,
				Stage:            segment.Stage,
				StartOffset:      offset,
				EndOffset:        endOffset,
				StartOffsetMS:    offset.Milliseconds(),
				EndOffsetMS:      endOffset.Milliseconds(),
				Duration:         duration,
				DurationMS:       duration.Milliseconds(),
				Rate:             rate,
				TargetRPS:        targetRPS,
				IncludedMeasured: segment.Stage == StageMeasured,
			})
			offset = endOffset
			if targetRPS > peakTargetRPS {
				peakTargetRPS = targetRPS
			}
		}
	}
	return &Compilation{Segments: compiled, TotalDuration: offset, PeakTargetRPS: peakTargetRPS}, nil
}
