package benchmarks

import (
	"context"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/charleszardd/daegsa/internal/clock"
	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/executor"
	"github.com/charleszardd/daegsa/internal/plan"
	"github.com/charleszardd/daegsa/internal/scheduler"
)

func createBenchPlan(rate float64, maxInFlight int64, duration time.Duration) *plan.Plan {
	parsed, _ := url.Parse("http://127.0.0.1:8080/bench")
	return &plan.Plan{
		Name:              "bench-open-scheduler",
		SchemaVersion:     1,
		Fingerprint:       "bench-fingerprint",
		TargetURL:         parsed,
		Method:            "GET",
		Headers:           make(http.Header),
		ExpectedStatuses:  []int{200},
		RequestTimeout:    1 * time.Second,
		ResponseBodyLimit: 1024,
		RedirectPolicy:    "none",
		Model:             core.WorkloadModelOpen,
		Rate:              rate,
		TimeUnit:          time.Second,
		MaxInFlight:       maxInFlight,
		Duration:          duration,
		GracefulStop:      100 * time.Millisecond,
		AllowedHosts:      []string{"127.0.0.1"},
	}
}

// BenchmarkOpenScheduler_Dispatch_1000RPS benchmarks scheduling at 1,000 req/s.
func BenchmarkOpenScheduler_Dispatch_1000RPS(b *testing.B) {
	benchmarkOpenSchedulerDispatch(b, 1000, 100)
}

// BenchmarkOpenScheduler_Dispatch_10000RPS benchmarks scheduling at 10,000 req/s.
func BenchmarkOpenScheduler_Dispatch_10000RPS(b *testing.B) {
	benchmarkOpenSchedulerDispatch(b, 10000, 500)
}

// BenchmarkOpenScheduler_Dispatch_50000RPS benchmarks scheduling at 50,000 req/s.
func BenchmarkOpenScheduler_Dispatch_50000RPS(b *testing.B) {
	benchmarkOpenSchedulerDispatch(b, 50000, 1000)
}

func benchmarkOpenSchedulerDispatch(b *testing.B, rps float64, maxInFlight int64) {
	p := createBenchPlan(rps, maxInFlight, 100*time.Millisecond)
	exec, err := executor.NewHTTPExecutor(p)
	if err != nil {
		b.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		clk := clock.NewControllableClock(time.Now())
		sched, err := scheduler.NewOpenScheduler(p, exec, clk)
		if err != nil {
			b.Fatalf("failed to create scheduler: %v", err)
		}

		ctx, cancel := context.WithCancel(context.Background())
		done := make(chan struct{})
		go func() {
			_, _, _ = sched.Run(ctx)
			close(done)
		}()

		clk.BlockUntilTimers(1)
		clk.Advance(100 * time.Millisecond)
		cancel()
		<-done
	}
}
