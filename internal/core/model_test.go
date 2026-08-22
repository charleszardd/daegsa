package core_test

import (
	"testing"
	"time"

	"github.com/charleszardd/daegsa/internal/core"
)

func TestWorkloadModel_IsValid(t *testing.T) {
	tests := []struct {
		name  string
		model core.WorkloadModel
		want  bool
	}{
		{name: "open model is valid", model: core.WorkloadModelOpen, want: true},
		{name: "closed model is valid", model: core.WorkloadModelClosed, want: true},
		{name: "empty model is invalid", model: "", want: false},
		{name: "unknown model is invalid", model: "hybrid", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.model.IsValid(); got != tt.want {
				t.Errorf("WorkloadModel(%q).IsValid() = %v, want %v", tt.model, got, tt.want)
			}
		})
	}
}

func TestOpenModelParams_Validate(t *testing.T) {
	tests := []struct {
		name    string
		params  core.OpenModelParams
		wantErr bool
	}{
		{
			name: "valid open params",
			params: core.OpenModelParams{
				Rate:        100,
				TimeUnit:    1 * time.Second,
				MaxInFlight: 500,
			},
			wantErr: false,
		},
		{
			name: "zero rate is invalid",
			params: core.OpenModelParams{
				Rate:        0,
				TimeUnit:    1 * time.Second,
				MaxInFlight: 500,
			},
			wantErr: true,
		},
		{
			name: "negative rate is invalid",
			params: core.OpenModelParams{
				Rate:        -10,
				TimeUnit:    1 * time.Second,
				MaxInFlight: 500,
			},
			wantErr: true,
		},
		{
			name: "zero time unit is invalid",
			params: core.OpenModelParams{
				Rate:        100,
				TimeUnit:    0,
				MaxInFlight: 500,
			},
			wantErr: true,
		},
		{
			name: "zero max in flight is invalid",
			params: core.OpenModelParams{
				Rate:        100,
				TimeUnit:    1 * time.Second,
				MaxInFlight: 0,
			},
			wantErr: true,
		},
		{
			name: "negative max in flight is invalid",
			params: core.OpenModelParams{
				Rate:        100,
				TimeUnit:    1 * time.Second,
				MaxInFlight: -50,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.params.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("OpenModelParams.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestOpenModelParams_Interval(t *testing.T) {
	params := core.OpenModelParams{
		Rate:        100,
		TimeUnit:    1 * time.Second,
		MaxInFlight: 500,
	}
	expected := 10 * time.Millisecond
	if got := params.Interval(); got != expected {
		t.Errorf("Interval() = %v, want %v", got, expected)
	}

	invalidParams := core.OpenModelParams{Rate: 0, TimeUnit: 1 * time.Second}
	if got := invalidParams.Interval(); got != 0 {
		t.Errorf("Interval() on zero rate = %v, want 0", got)
	}
}

func TestClosedModelParams_Validate(t *testing.T) {
	tests := []struct {
		name    string
		params  core.ClosedModelParams
		wantErr bool
	}{
		{
			name: "valid closed params with zero think time",
			params: core.ClosedModelParams{
				Users:     50,
				ThinkTime: 0,
			},
			wantErr: false,
		},
		{
			name: "valid closed params with positive think time",
			params: core.ClosedModelParams{
				Users:     10,
				ThinkTime: 250 * time.Millisecond,
			},
			wantErr: false,
		},
		{
			name: "zero users is invalid",
			params: core.ClosedModelParams{
				Users:     0,
				ThinkTime: 100 * time.Millisecond,
			},
			wantErr: true,
		},
		{
			name: "negative users is invalid",
			params: core.ClosedModelParams{
				Users:     -5,
				ThinkTime: 100 * time.Millisecond,
			},
			wantErr: true,
		},
		{
			name: "negative think time is invalid",
			params: core.ClosedModelParams{
				Users:     10,
				ThinkTime: -100 * time.Millisecond,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.params.Validate()
			if (err != nil) != tt.wantErr {
				t.Errorf("ClosedModelParams.Validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestTerminologyConstants(t *testing.T) {
	// Verify canonical terms match exact specification strings
	if core.TermOpenModel != "open model" {
		t.Errorf("TermOpenModel = %q, want 'open model'", core.TermOpenModel)
	}
	if core.TermClosedModel != "closed model" {
		t.Errorf("TermClosedModel = %q, want 'closed model'", core.TermClosedModel)
	}
	if core.TermTargetRPS != "target RPS" {
		t.Errorf("TermTargetRPS = %q, want 'target RPS'", core.TermTargetRPS)
	}
	if core.TermAchievedStartRate != "achieved start rate" {
		t.Errorf("TermAchievedStartRate = %q, want 'achieved start rate'", core.TermAchievedStartRate)
	}
	if core.TermCompletedThroughput != "completed throughput" {
		t.Errorf("TermCompletedThroughput = %q, want 'completed throughput'", core.TermCompletedThroughput)
	}
	if core.TermInFlight != "in flight" {
		t.Errorf("TermInFlight = %q, want 'in flight'", core.TermInFlight)
	}
	if core.TermDropped != "dropped" {
		t.Errorf("TermDropped = %q, want 'dropped'", core.TermDropped)
	}
	if core.TermCanceled != "canceled" {
		t.Errorf("TermCanceled = %q, want 'canceled'", core.TermCanceled)
	}
	if core.TermRateLimited != "rate limited" {
		t.Errorf("TermRateLimited = %q, want 'rate limited'", core.TermRateLimited)
	}
}
