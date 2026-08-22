package scheduler

import (
	"context"
	"runtime"
	"testing"
	"time"

	"github.com/charleszardd/daegsa/internal/clock"
	"github.com/charleszardd/daegsa/internal/executor"
	"github.com/charleszardd/daegsa/internal/testtarget"
)

func TestClosedScheduler_ZeroGoroutineLeak(t *testing.T) {
	ts := testtarget.NewServer()
	defer ts.Close()

	// Stabilize baseline goroutines
	runtime.GC()
	time.Sleep(50 * time.Millisecond)
	initialGoroutines := runtime.NumGoroutine()

	p := createTestPlan(ts.URL(), 10, 200*time.Millisecond, 5*time.Millisecond, 500*time.Millisecond)
	exec, err := executor.NewHTTPExecutor(p)
	if err != nil {
		t.Fatalf("failed to create http executor: %v", err)
	}

	sched, err := NewClosedScheduler(p, exec, clock.NewRealClock())
	if err != nil {
		t.Fatalf("failed to create closed scheduler: %v", err)
	}

	_, _, err = sched.Run(context.Background())
	if err != nil {
		t.Fatalf("scheduler run failed: %v", err)
	}

	exec.Close()

	// Wait and GC to ensure goroutine cleanup
	var finalGoroutines int
	for attempt := 0; attempt < 10; attempt++ {
		runtime.GC()
		time.Sleep(20 * time.Millisecond)
		finalGoroutines = runtime.NumGoroutine()
		if finalGoroutines <= initialGoroutines {
			break
		}
	}

	if finalGoroutines > initialGoroutines {
		t.Errorf("goroutine leak detected: initial %d, final %d (+%d)",
			initialGoroutines, finalGoroutines, finalGoroutines-initialGoroutines)
	}
}

func TestClosedScheduler_ZeroConnectionLeak(t *testing.T) {
	ts := testtarget.NewServer()
	defer ts.Close()

	p := createTestPlan(ts.URL(), 5, 200*time.Millisecond, 0, 500*time.Millisecond)
	exec, err := executor.NewHTTPExecutor(p)
	if err != nil {
		t.Fatalf("failed to create http executor: %v", err)
	}

	sched, err := NewClosedScheduler(p, exec, clock.NewRealClock())
	if err != nil {
		t.Fatalf("failed to create closed scheduler: %v", err)
	}

	agg, _, err := sched.Run(context.Background())
	if err != nil {
		t.Fatalf("scheduler run failed: %v", err)
	}

	if agg.RequestCounts.Completed == 0 {
		t.Fatalf("expected completed requests > 0")
	}

	// Close executor and transport
	exec.Close()
}

func TestClosedScheduler_BoundedMemorySoak(t *testing.T) {
	ts := testtarget.NewServer()
	defer ts.Close()

	p := createTestPlan(ts.URL(), 10, 500*time.Millisecond, 0, 500*time.Millisecond)
	exec, err := executor.NewHTTPExecutor(p)
	if err != nil {
		t.Fatalf("failed to create http executor: %v", err)
	}
	defer exec.Close()

	sched, err := NewClosedScheduler(p, exec, clock.NewRealClock())
	if err != nil {
		t.Fatalf("failed to create closed scheduler: %v", err)
	}

	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)

	agg, _, err := sched.Run(context.Background())
	if err != nil {
		t.Fatalf("scheduler run failed: %v", err)
	}

	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)

	if agg.RequestCounts.Completed == 0 {
		t.Fatalf("expected completed requests > 0")
	}

	// Verify heap allocation is bounded (Alloc < 50MB)
	if m2.Alloc > 50*1024*1024 {
		t.Errorf("excessive memory allocation during soak: %d bytes", m2.Alloc)
	}
}
