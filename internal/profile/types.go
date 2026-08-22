package profile

import "time"

const (
	StageWarmup   = "warmup"
	StageMeasured = "measured"
	StageCooldown = "cooldown"

	MaxSourceSegments    = 64
	MaxCompiledSegments  = 256
	MaxSegmentNameLength = 64
)

// SourceSegment is the validated configuration input to profile compilation.
type SourceSegment struct {
	Name      string
	Stage     string
	Duration  time.Duration
	Rate      float64
	StartRate float64
	EndRate   float64
	Steps     int
}

// Segment is one immutable constant-rate scheduling interval.
type Segment struct {
	Index            int           `json:"index"`
	Name             string        `json:"name"`
	Stage            string        `json:"stage"`
	StartOffset      time.Duration `json:"-"`
	EndOffset        time.Duration `json:"-"`
	StartOffsetMS    int64         `json:"start_offset_ms"`
	EndOffsetMS      int64         `json:"end_offset_ms"`
	Duration         time.Duration `json:"-"`
	DurationMS       int64         `json:"duration_ms"`
	Rate             float64       `json:"rate"`
	TargetRPS        float64       `json:"target_rps"`
	IncludedMeasured bool          `json:"included_measured"`
}

// Compilation is a deterministic immutable scheduling profile.
type Compilation struct {
	Segments      []Segment
	TotalDuration time.Duration
	PeakTargetRPS float64
}
