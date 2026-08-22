package core

import (
	"errors"
	"fmt"
	"sync"
)

// LifecycleState represents a phase in the load test lifecycle (§7).
type LifecycleState string

const (
	// StateInitialized indicates the test has been validated and prepared, but traffic has not started.
	StateInitialized LifecycleState = "initialized"

	// StateWarmup indicates optional preliminary traffic is running (excluded from threshold metrics).
	StateWarmup LifecycleState = "warmup"

	// StateRunning indicates the primary measured load test duration is actively running.
	StateRunning LifecycleState = "running"

	// StateCooldown indicates profile cool-down traffic is running and excluded from thresholds.
	StateCooldown LifecycleState = "cooldown"

	// StateGracefulStop indicates no new requests are scheduled, and in-flight requests are draining.
	StateGracefulStop LifecycleState = "graceful_stop"

	// StateCanceled indicates the test was interrupted/aborted before normal graceful completion.
	StateCanceled LifecycleState = "canceled"

	// StateCompleted indicates all execution and draining have concluded and final reports can be generated.
	StateCompleted LifecycleState = "completed"
)

var (
	// ErrInvalidStateTransition is returned when an illegal lifecycle transition is attempted.
	ErrInvalidStateTransition = errors.New("invalid lifecycle state transition")
)

// IsValid reports whether s is a recognized lifecycle state.
func (s LifecycleState) IsValid() bool {
	switch s {
	case StateInitialized, StateWarmup, StateRunning, StateCooldown, StateGracefulStop, StateCanceled, StateCompleted:
		return true
	default:
		return false
	}
}

// String returns the string representation of the lifecycle state.
func (s LifecycleState) String() string {
	return string(s)
}

// CanTransitionTo reports whether transitioning from current state s to target next is legal.
func (s LifecycleState) CanTransitionTo(next LifecycleState) bool {
	switch s {
	case StateInitialized:
		return next == StateWarmup || next == StateRunning || next == StateCanceled
	case StateWarmup:
		return next == StateRunning || next == StateGracefulStop || next == StateCanceled
	case StateRunning:
		return next == StateCooldown || next == StateGracefulStop || next == StateCanceled || next == StateCompleted
	case StateCooldown:
		return next == StateGracefulStop || next == StateCanceled
	case StateGracefulStop:
		return next == StateCompleted || next == StateCanceled
	case StateCanceled:
		return next == StateCompleted
	case StateCompleted:
		return false
	default:
		return false
	}
}

// LifecycleStateMachine manages thread-safe lifecycle transitions for a load test run.
type LifecycleStateMachine struct {
	mu      sync.RWMutex
	current LifecycleState
}

// NewLifecycleStateMachine creates a new state machine starting in StateInitialized.
func NewLifecycleStateMachine() *LifecycleStateMachine {
	return &LifecycleStateMachine{
		current: StateInitialized,
	}
}

// Current returns the current lifecycle state.
func (m *LifecycleStateMachine) Current() LifecycleState {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.current
}

// TransitionTo attempts to transition the state machine to target next.
// Returns an error if the transition is illegal according to the lifecycle state rules.
func (m *LifecycleStateMachine) TransitionTo(next LifecycleState) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.current.CanTransitionTo(next) {
		return fmt.Errorf("%w: cannot transition from %s to %s", ErrInvalidStateTransition, m.current, next)
	}

	m.current = next
	return nil
}
