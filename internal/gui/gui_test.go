package gui

import (
	"context"
	"testing"
	"time"

	"github.com/charleszardd/daegsa/internal/core"
)

func TestNewTheme(t *testing.T) {
	th := NewTheme()
	if th == nil {
		t.Fatal("expected non-nil Theme")
	}
	if th.Material == nil {
		t.Fatal("expected non-nil Material theme")
	}
	if th.BgDark.A != 255 {
		t.Errorf("expected opaque background, got alpha %d", th.BgDark.A)
	}
}

func TestNewState(t *testing.T) {
	redrawCount := 0
	state := NewState(func() {
		redrawCount++
	})

	if state == nil {
		t.Fatal("expected non-nil State")
	}
	if state.ActiveTab != TabBuilder {
		t.Errorf("expected initial tab TabBuilder, got %v", state.ActiveTab)
	}
	if state.RunState != StateIdle {
		t.Errorf("expected initial state StateIdle, got %v", state.RunState)
	}

	state.RequestRedraw()
	if redrawCount != 1 {
		t.Errorf("expected redraw count 1, got %d", redrawCount)
	}
}

func TestState_ValidateCurrentPlan_Valid(t *testing.T) {
	state := NewState(nil)
	state.Builder.ConfigYAML = `schema_version: 1
name: test-validation
request:
  url: http://127.0.0.1:8080/api/test
  method: GET
load:
  model: open
  rate: 50
  duration: 5s
  max_in_flight: 100
safety:
  allowed_hosts: [127.0.0.1]
`

	err := state.ValidateCurrentPlan(context.Background())
	if err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
	if state.Builder.ValidationStatus != "PASS" {
		t.Errorf("expected validation status PASS, got %s", state.Builder.ValidationStatus)
	}
	if state.Builder.CompiledPlan == nil {
		t.Fatal("expected compiled plan to be set")
	}
	if state.Builder.CompiledPlan.Model != core.WorkloadModelOpen {
		t.Errorf("expected Open model, got %s", state.Builder.CompiledPlan.Model)
	}
}

func TestState_ValidateCurrentPlan_InvalidYAML(t *testing.T) {
	state := NewState(nil)
	state.Builder.ConfigYAML = `malformed: [yaml`

	err := state.ValidateCurrentPlan(context.Background())
	if err == nil {
		t.Fatal("expected validation error for malformed YAML, got nil")
	}
	if state.Builder.ValidationStatus != "FAIL" {
		t.Errorf("expected validation status FAIL, got %s", state.Builder.ValidationStatus)
	}
}

func TestState_RunDiagnostics(t *testing.T) {
	state := NewState(nil)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	state.RunDiagnostics(ctx)
	if state.Doctor == nil {
		t.Fatal("expected doctor report to be populated")
	}
	if len(state.Doctor.Checks) == 0 {
		t.Error("expected at least one diagnostic check")
	}
}

func TestRunState_String(t *testing.T) {
	tests := []struct {
		state RunState
		want  string
	}{
		{StateIdle, "IDLE"},
		{StateRunning, "RUNNING"},
		{StateDraining, "DRAINING"},
		{StateCompleted, "COMPLETED"},
		{StateFailed, "FAILED"},
		{RunState(99), "UNKNOWN"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("state %d String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}
