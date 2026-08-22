package executor

import (
	"time"

	"github.com/charleszardd/daegsa/internal/core"
)

// Result captures the complete timing, protocol, outcome, and byte metrics of a single HTTP execution (§8, §9).
type Result struct {
	Outcome       core.Outcome
	StatusCode    int
	Protocol      string
	Timestamps    core.RequestTimestamps
	Latency       time.Duration
	TTFB          time.Duration
	TotalDuration time.Duration
	BytesSent     int64
	BytesReceived int64
	Truncated     bool
	RateLimitInfo *RateLimitInfo
	Err           error
}
