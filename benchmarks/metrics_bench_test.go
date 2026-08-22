package benchmarks

import (
	"net/http"
	"testing"
	"time"

	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/executor"
	"github.com/charleszardd/daegsa/internal/metrics"
)

// BenchmarkHistogram_Record benchmarks recording microsecond latency samples.
func BenchmarkHistogram_Record(b *testing.B) {
	h := metrics.NewLatencyHistogram()
	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = h.Record(int64((i % 1000) * 1000))
	}
}

// BenchmarkHistogram_ValueAtQuantile benchmarks percentile calculation.
func BenchmarkHistogram_ValueAtQuantile(b *testing.B) {
	h := metrics.NewLatencyHistogram()
	for i := 0; i < 10000; i++ {
		_ = h.Record(int64((i % 1000) * 1000))
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_ = h.ValueAtQuantile(99.0)
	}
}

// BenchmarkWorkerMetrics_RecordResult benchmarks lock-free per-request metric recording.
func BenchmarkWorkerMetrics_RecordResult(b *testing.B) {
	wm := metrics.NewWorkerMetrics(0)
	res := &executor.Result{
		Outcome:       core.OutcomeSuccess,
		StatusCode:    http.StatusOK,
		Latency:       15 * time.Millisecond,
		TTFB:          5 * time.Millisecond,
		BytesSent:     128,
		BytesReceived: 1024,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		wm.RecordResult(res)
	}
}

// BenchmarkMetrics_MergeWorkers benchmarks merging 50 worker metric instances.
func BenchmarkMetrics_MergeWorkers(b *testing.B) {
	workers := make([]*metrics.WorkerMetrics, 50)
	for i := 0; i < 50; i++ {
		wm := metrics.NewWorkerMetrics(i)
		wm.Planned = 1000
		wm.Started = 1000
		wm.Completed = 1000
		for j := 0; j < 200; j++ {
			wm.RecordResult(&executor.Result{
				Outcome:       core.OutcomeSuccess,
				StatusCode:    200,
				Latency:       time.Duration(j+1) * time.Millisecond,
				BytesSent:     100,
				BytesReceived: 500,
			})
		}
		workers[i] = wm
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = metrics.MergeWorkers(workers, 30*time.Second)
	}
}
