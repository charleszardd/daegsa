package core

import (
	"errors"
	"fmt"
	"time"
)

// WorkloadModel defines the execution model for load generation.
type WorkloadModel string

const (
	// WorkloadModelOpen schedules requests according to an arrival rate independent of response time.
	WorkloadModelOpen WorkloadModel = "open"

	// WorkloadModelClosed executes a fixed number of concurrent virtual users in request-wait loops.
	WorkloadModelClosed WorkloadModel = "closed"
)

// Canonical terminology constants defined across DAEGSA specifications (§2, §9, §13).
const (
	TermOpenModel           = "open model"
	TermClosedModel         = "closed model"
	TermTargetRPS           = "target RPS"
	TermAchievedStartRate   = "achieved start rate"
	TermCompletedThroughput = "completed throughput"
	TermInFlight            = "in flight"
	TermDropped             = "dropped"
	TermCanceled            = "canceled"
	TermRateLimited         = "rate limited"
)

var (
	// ErrInvalidWorkloadModel indicates an unrecognized workload model string.
	ErrInvalidWorkloadModel = errors.New("invalid workload model")

	// ErrInvalidOpenModelParams indicates invalid or missing parameters for the open workload model.
	ErrInvalidOpenModelParams = errors.New("invalid open model parameters")

	// ErrInvalidClosedModelParams indicates invalid or missing parameters for the closed workload model.
	ErrInvalidClosedModelParams = errors.New("invalid closed model parameters")

	// ErrIncompatibleModelParams indicates parameters specified that belong to a different model.
	ErrIncompatibleModelParams = errors.New("incompatible workload model parameters")
)

// IsValid reports whether m is one of the supported workload models.
func (m WorkloadModel) IsValid() bool {
	return m == WorkloadModelOpen || m == WorkloadModelClosed
}

// String returns the string representation of the workload model.
func (m WorkloadModel) String() string {
	return string(m)
}

// OpenModelParams defines the required execution parameters for an open-model test.
type OpenModelParams struct {
	Rate        float64       `json:"rate" yaml:"rate"`
	TimeUnit    time.Duration `json:"time_unit" yaml:"time_unit"`
	MaxInFlight int64         `json:"max_in_flight" yaml:"max_in_flight"`
}

// Validate checks that all open-model parameters satisfy their invariants.
func (p OpenModelParams) Validate() error {
	if p.Rate <= 0 {
		return fmt.Errorf("%w: rate must be > 0, got %v", ErrInvalidOpenModelParams, p.Rate)
	}
	if p.TimeUnit <= 0 {
		return fmt.Errorf("%w: time_unit must be > 0, got %v", ErrInvalidOpenModelParams, p.TimeUnit)
	}
	if p.MaxInFlight <= 0 {
		return fmt.Errorf("%w: max_in_flight must be > 0, got %d", ErrInvalidOpenModelParams, p.MaxInFlight)
	}
	return nil
}

// Interval returns the calculated inter-arrival duration between requests.
func (p OpenModelParams) Interval() time.Duration {
	if p.Rate <= 0 || p.TimeUnit <= 0 {
		return 0
	}
	return time.Duration(float64(p.TimeUnit) / p.Rate)
}

// ClosedModelParams defines the required execution parameters for a closed-model test.
type ClosedModelParams struct {
	Users     int64         `json:"users" yaml:"users"`
	ThinkTime time.Duration `json:"think_time" yaml:"think_time"`
}

// Validate checks that all closed-model parameters satisfy their invariants.
func (p ClosedModelParams) Validate() error {
	if p.Users <= 0 {
		return fmt.Errorf("%w: users must be > 0, got %d", ErrInvalidClosedModelParams, p.Users)
	}
	if p.ThinkTime < 0 {
		return fmt.Errorf("%w: think_time must be >= 0, got %v", ErrInvalidClosedModelParams, p.ThinkTime)
	}
	return nil
}
