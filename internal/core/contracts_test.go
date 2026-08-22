package core_test

import (
	"sync"
	"testing"
	"time"

	"github.com/charleszardd/daegsa/internal/core"
)

func TestLifecycleStateMachine_Transitions(t *testing.T) {
	t.Run("valid full happy path transition", func(t *testing.T) {
		sm := core.NewLifecycleStateMachine()
		if got := sm.Current(); got != core.StateInitialized {
			t.Fatalf("initial state = %s, want %s", got, core.StateInitialized)
		}

		if err := sm.TransitionTo(core.StateWarmup); err != nil {
			t.Fatalf("TransitionTo(Warmup) unexpected error: %v", err)
		}
		if err := sm.TransitionTo(core.StateRunning); err != nil {
			t.Fatalf("TransitionTo(Running) unexpected error: %v", err)
		}
		if err := sm.TransitionTo(core.StateGracefulStop); err != nil {
			t.Fatalf("TransitionTo(GracefulStop) unexpected error: %v", err)
		}
		if err := sm.TransitionTo(core.StateCompleted); err != nil {
			t.Fatalf("TransitionTo(Completed) unexpected error: %v", err)
		}
	})

	t.Run("valid direct running without warmup", func(t *testing.T) {
		sm := core.NewLifecycleStateMachine()
		if err := sm.TransitionTo(core.StateRunning); err != nil {
			t.Fatalf("TransitionTo(Running) unexpected error: %v", err)
		}
		if err := sm.TransitionTo(core.StateGracefulStop); err != nil {
			t.Fatalf("TransitionTo(GracefulStop) unexpected error: %v", err)
		}
		if err := sm.TransitionTo(core.StateCompleted); err != nil {
			t.Fatalf("TransitionTo(Completed) unexpected error: %v", err)
		}
	})

	t.Run("valid cancellation path", func(t *testing.T) {
		sm := core.NewLifecycleStateMachine()
		if err := sm.TransitionTo(core.StateRunning); err != nil {
			t.Fatalf("TransitionTo(Running) error: %v", err)
		}
		if err := sm.TransitionTo(core.StateCanceled); err != nil {
			t.Fatalf("TransitionTo(Canceled) error: %v", err)
		}
		if err := sm.TransitionTo(core.StateCompleted); err != nil {
			t.Fatalf("TransitionTo(Completed) error: %v", err)
		}
	})

	t.Run("invalid backward and skip transitions", func(t *testing.T) {
		sm := core.NewLifecycleStateMachine()

		// Cannot transition directly from Initialized to Completed
		if err := sm.TransitionTo(core.StateCompleted); err == nil {
			t.Errorf("expected error transitioning from Initialized to Completed")
		}

		// Move to Running
		if err := sm.TransitionTo(core.StateRunning); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Cannot transition from Running back to Initialized or Warmup
		if err := sm.TransitionTo(core.StateInitialized); err == nil {
			t.Errorf("expected error transitioning from Running to Initialized")
		}
		if err := sm.TransitionTo(core.StateWarmup); err == nil {
			t.Errorf("expected error transitioning from Running to Warmup")
		}

		// Move to Completed
		if err := sm.TransitionTo(core.StateCompleted); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Completed is terminal; cannot transition anywhere
		if err := sm.TransitionTo(core.StateRunning); err == nil {
			t.Errorf("expected error transitioning out of Completed")
		}
	})
}

func TestLifecycleStateMachine_ConcurrentSafety(t *testing.T) {
	sm := core.NewLifecycleStateMachine()
	_ = sm.TransitionTo(core.StateRunning)

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = sm.Current()
		}()
	}
	wg.Wait()
}

func TestRequestTimestamps_TimingContract(t *testing.T) {
	base := time.Date(2026, 8, 22, 10, 0, 0, 0, time.UTC)

	plannedAt := base
	scheduledAt := base.Add(5 * time.Millisecond)
	dispatchedAt := base.Add(6 * time.Millisecond)
	headersReceivedAt := base.Add(30 * time.Millisecond)
	bodyCompletedAt := base.Add(45 * time.Millisecond)

	timestamps := core.RequestTimestamps{
		PlannedAt:         plannedAt,
		ScheduledAt:       scheduledAt,
		DispatchedAt:      dispatchedAt,
		HeadersReceivedAt: headersReceivedAt,
		BodyCompletedAt:   bodyCompletedAt,
	}

	// Schedule lag: 5ms
	if got := timestamps.ScheduleLag(); got != 5*time.Millisecond {
		t.Errorf("ScheduleLag() = %v, want 5ms", got)
	}

	// TTFB: 30ms - 6ms = 24ms
	if got := timestamps.TTFB(); got != 24*time.Millisecond {
		t.Errorf("TTFB() = %v, want 24ms", got)
	}

	// Latency (dispatch-to-body-consumed): 45ms - 6ms = 39ms
	if got := timestamps.Latency(); got != 39*time.Millisecond {
		t.Errorf("Latency() = %v, want 39ms", got)
	}

	// TotalDuration: 45ms - 0ms = 45ms
	if got := timestamps.TotalDuration(); got != 45*time.Millisecond {
		t.Errorf("TotalDuration() = %v, want 45ms", got)
	}
}

func TestRequestTimestamps_ZeroAndEdgeCases(t *testing.T) {
	zeroTS := core.RequestTimestamps{}
	if got := zeroTS.Latency(); got != 0 {
		t.Errorf("zero timestamps Latency() = %v, want 0", got)
	}
	if got := zeroTS.TTFB(); got != 0 {
		t.Errorf("zero timestamps TTFB() = %v, want 0", got)
	}
	if got := zeroTS.ScheduleLag(); got != 0 {
		t.Errorf("zero timestamps ScheduleLag() = %v, want 0", got)
	}
	if got := zeroTS.TotalDuration(); got != 0 {
		t.Errorf("zero timestamps TotalDuration() = %v, want 0", got)
	}
}
