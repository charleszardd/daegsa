package scheduler

import (
	"context"

	"github.com/charleszardd/daegsa/internal/metrics"
)

// Scheduler defines the common lifecycle execution interface for workload schedulers (§4, §7).
type Scheduler interface {
	// Run executes the scheduled load test until duration expires, graceful drain completes, or context is canceled.
	Run(ctx context.Context) (*metrics.AggregatedMetrics, *metrics.GeneratorHealth, error)
}
