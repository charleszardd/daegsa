package metrics

import (
	"testing"

	"github.com/charleszardd/daegsa/internal/executor"
)

func TestHeaderConsistencyTracksErrorsAndDisagreement(t *testing.T) {
	worker := NewWorkerMetrics(0)
	worker.recordHeaderObservations([]executor.HeaderParseObservation{{Name: "limit", Present: true, Valid: true, Value: "10"}, {Name: "limit", Present: true, Valid: true, Value: "20"}, {Name: "limit", Present: true}})
	observation := worker.RateLimits.HeaderConsistency["limit"]
	if observation.ObservedCount != 3 || observation.ParseErrorCount != 1 || observation.AllParsedAgree {
		t.Fatalf("unexpected consistency: %+v", observation)
	}
}
