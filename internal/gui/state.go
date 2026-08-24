package gui

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/charleszardd/daegsa/internal/clock"
	"github.com/charleszardd/daegsa/internal/compare"
	"github.com/charleszardd/daegsa/internal/config"
	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/doctor"
	"github.com/charleszardd/daegsa/internal/executor"
	"github.com/charleszardd/daegsa/internal/metrics"
	"github.com/charleszardd/daegsa/internal/plan"
	"github.com/charleszardd/daegsa/internal/report"
	"github.com/charleszardd/daegsa/internal/safety"
	"github.com/charleszardd/daegsa/internal/scheduler"
	"github.com/charleszardd/daegsa/internal/threshold"
)

// NavTab represents the currently selected workspace view.
type NavTab int

const (
	TabBuilder NavTab = iota
	TabMonitor
	TabCompare
	TabDoctor
)

// RunState represents the execution lifecycle of the load generator.
type RunState int

const (
	StateIdle RunState = iota
	StateRunning
	StateDraining
	StateCompleted
	StateFailed
)

func (s RunState) String() string {
	switch s {
	case StateIdle:
		return "IDLE"
	case StateRunning:
		return "RUNNING"
	case StateDraining:
		return "DRAINING"
	case StateCompleted:
		return "COMPLETED"
	case StateFailed:
		return "FAILED"
	default:
		return "UNKNOWN"
	}
}

// ChartPoint captures a point in time for throughput and latency series.
type ChartPoint struct {
	TimestampSec float64
	TargetRPS    float64
	CompletedRPS float64
	P50MS        float64
	P95MS        float64
	P99MS        float64
}

// TelemetrySnapshot captures real-time engine metrics for UI rendering.
type TelemetrySnapshot struct {
	State            RunState
	Duration         time.Duration
	Elapsed          time.Duration
	Progress         float32
	TargetRPS        float64
	AchievedRPS      float64
	CompletedRPS     float64
	InFlight         int64
	MaxInFlight      int64
	PlannedTotal     int64
	StartedTotal     int64
	CompletedTotal   int64
	DroppedTotal     int64
	ErrorCount       int64
	ErrorRate        float64
	P50MS            float64
	P90MS            float64
	P95MS            float64
	P99MS            float64
	MaxMS            float64
	StatusCodes      map[string]int64
	Outcomes         map[core.Outcome]int64
	ErrorSamples     []metrics.ErrorSample
	ThresholdResults []report.ThresholdResult
	TimeSeries       []ChartPoint
}

// BuilderForm holds input state for building a test plan.
type BuilderForm struct {
	URL              string
	Method           string
	Model            string // "open" or "closed"
	Rate             string
	Users            string
	Duration         string
	MaxInFlight      string
	ThinkTime        string
	AllowDestructive bool
	ConfigYAML       string

	// Validation
	ValidationStatus  string
	ValidationMessage string
	CompiledPlan      *plan.Plan
}

// CompareForm holds inputs and state for report comparison.
type CompareForm struct {
	BaselinePath  string
	CandidatePath string
	BaselineJSON  string
	CandidateJSON string
	Result        *compare.Result
	ErrorMessage  string
}

// State manages reactive data and synchronization across UI components and background engine goroutines.
type State struct {
	mu           sync.RWMutex
	ActiveTab    NavTab
	InvalidateFn func()

	// Execution
	RunState     RunState
	CancelFn     context.CancelFunc
	Telemetry    TelemetrySnapshot
	FinalReport  *report.Report
	ErrorMessage string

	// Forms & Panels
	Builder BuilderForm
	Compare CompareForm
	Doctor  *doctor.DiagnosticReport
}

// NewState creates a default initialized application state.
func NewState(invalidate func()) *State {
	s := &State{
		ActiveTab:    TabBuilder,
		InvalidateFn: invalidate,
		RunState:     StateIdle,
		Builder: BuilderForm{
			URL:              "http://127.0.0.1:8080/api/items",
			Method:           "GET",
			Model:            "open",
			Rate:             "100",
			Users:            "10",
			Duration:         "30s",
			MaxInFlight:      "500",
			ThinkTime:        "20ms",
			AllowDestructive: false,
			ConfigYAML: `# DAEGSA Open-Model Capacity Test
schema_version: 1
name: quick-capacity-test

request:
  url: http://127.0.0.1:8080/api/items
  method: GET

load:
  model: open
  rate: 100
  time_unit: 1s
  duration: 30s
  max_in_flight: 500

thresholds:
  p95: "<= 100ms"
  http_error_rate: "<= 1%"

safety:
  allowed_hosts: [127.0.0.1]
`,
		},
		Telemetry: TelemetrySnapshot{
			StatusCodes: make(map[string]int64),
			Outcomes:    make(map[core.Outcome]int64),
			TimeSeries:  make([]ChartPoint, 0, 120),
		},
	}
	return s
}

// RequestRedraw requests a UI frame invalidation safely.
func (s *State) RequestRedraw() {
	if s.InvalidateFn != nil {
		s.InvalidateFn()
	}
}

// ValidateCurrentPlan parses and verifies the currently active builder form / YAML.
func (s *State) ValidateCurrentPlan(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	var cfg *config.Config

	if strings.TrimSpace(s.Builder.ConfigYAML) != "" {
		expanded, expErr := config.ExpandEnv([]byte(s.Builder.ConfigYAML), nil)
		if expErr != nil {
			s.Builder.ValidationStatus = "FAIL"
			s.Builder.ValidationMessage = fmt.Sprintf("Environment expansion error: %v", expErr)
			return expErr
		}
		parsed, parseErr := config.ParseAndValidateYAML(expanded)
		if parseErr != nil {
			s.Builder.ValidationStatus = "FAIL"
			s.Builder.ValidationMessage = fmt.Sprintf("YAML validation error: %v", parseErr)
			return parseErr
		}
		cfg = parsed
	} else {
		rateVal, _ := strconv.ParseFloat(s.Builder.Rate, 64)
		usersVal, _ := strconv.Atoi(s.Builder.Users)
		durVal, durErr := time.ParseDuration(s.Builder.Duration)
		if durErr != nil {
			durVal = 30 * time.Second
		}
		mifVal, _ := strconv.ParseInt(s.Builder.MaxInFlight, 10, 64)
		if mifVal <= 0 {
			mifVal = 500
		}
		ttVal, _ := time.ParseDuration(s.Builder.ThinkTime)

		cfg = &config.Config{
			SchemaVersion: config.LegacySchemaVersion,
			Name:          "gui-custom-test",
			Request: config.RequestConfig{
				URL:    s.Builder.URL,
				Method: s.Builder.Method,
			},
			Load: config.LoadConfig{
				Model:       core.WorkloadModel(s.Builder.Model),
				Rate:        rateVal,
				TimeUnit:    config.Duration(1 * time.Second),
				Users:       int64(usersVal),
				Duration:    config.Duration(durVal),
				MaxInFlight: mifVal,
				ThinkTime:   config.Duration(ttVal),
			},
		}
	}

	// Safety Preflight
	engine := safety.NewPreflightEngine()
	preflight, preErr := engine.Check(ctx, cfg, safety.SafetyFlags{
		AllowDestructive: s.Builder.AllowDestructive,
		NonInteractive:   true,
	})
	if preErr != nil {
		s.Builder.ValidationStatus = "SAFETY REFUSAL"
		s.Builder.ValidationMessage = fmt.Sprintf("Safety preflight failed: %v", preErr)
		return preErr
	}

	compiled, planErr := plan.BuildPlan(cfg, preflight)
	if planErr != nil {
		s.Builder.ValidationStatus = "FAIL"
		s.Builder.ValidationMessage = fmt.Sprintf("Plan build failed: %v", planErr)
		return planErr
	}

	s.Builder.CompiledPlan = compiled
	s.Builder.ValidationStatus = "PASS"
	s.Builder.ValidationMessage = fmt.Sprintf("Valid %s model plan compiled for %s", compiled.Model, compiled.TargetURL.String())
	return nil
}

// StartExecution launches the load test in a background goroutine.
func (s *State) StartExecution(ctx context.Context) error {
	if err := s.ValidateCurrentPlan(ctx); err != nil {
		return err
	}

	s.mu.Lock()
	if s.RunState == StateRunning || s.RunState == StateDraining {
		s.mu.Unlock()
		return fmt.Errorf("a test is already running")
	}

	compiledPlan := s.Builder.CompiledPlan
	if compiledPlan == nil {
		s.mu.Unlock()
		return fmt.Errorf("no compiled plan available")
	}

	runCtx, cancel := context.WithCancel(context.Background())
	s.CancelFn = cancel
	s.RunState = StateRunning
	s.ActiveTab = TabMonitor
	s.ErrorMessage = ""
	s.FinalReport = nil
	s.Telemetry = TelemetrySnapshot{
		State:       StateRunning,
		Duration:    compiledPlan.Duration,
		TargetRPS:   compiledPlan.TargetRPS(),
		MaxInFlight: compiledPlan.MaxInFlight,
		StatusCodes: make(map[string]int64),
		Outcomes:    make(map[core.Outcome]int64),
		TimeSeries:  make([]ChartPoint, 0, 120),
	}
	s.mu.Unlock()
	s.RequestRedraw()

	// Launch background execution
	go s.runEngine(runCtx, compiledPlan)
	return nil
}

// StopGracefully signals the engine to drain.
func (s *State) StopGracefully() {
	s.mu.Lock()
	if s.RunState == StateRunning {
		s.RunState = StateDraining
	}
	s.mu.Unlock()
	s.RequestRedraw()
}

// AbortExecution terminates the execution context immediately.
func (s *State) AbortExecution() {
	s.mu.Lock()
	if s.CancelFn != nil {
		s.CancelFn()
	}
	s.RunState = StateFailed
	s.mu.Unlock()
	s.RequestRedraw()
}

// runEngine executes the scheduler and streams telemetry.
func (s *State) runEngine(ctx context.Context, p *plan.Plan) {
	exec, err := executor.NewHTTPExecutor(p)
	if err != nil {
		s.mu.Lock()
		s.RunState = StateFailed
		s.ErrorMessage = fmt.Sprintf("HTTP executor error: %v", err)
		s.mu.Unlock()
		s.RequestRedraw()
		return
	}
	defer exec.Close()

	startTime := time.Now().UTC()

	var sched scheduler.Scheduler
	if p.Model == core.WorkloadModelClosed {
		clSched, clErr := scheduler.NewClosedScheduler(p, exec, clock.NewRealClock())
		if clErr != nil {
			s.mu.Lock()
			s.RunState = StateFailed
			s.ErrorMessage = fmt.Sprintf("Closed scheduler error: %v", clErr)
			s.mu.Unlock()
			s.RequestRedraw()
			return
		}
		sched = clSched
	} else {
		opSched, opErr := scheduler.NewOpenScheduler(p, exec, clock.NewRealClock())
		if opErr != nil {
			s.mu.Lock()
			s.RunState = StateFailed
			s.ErrorMessage = fmt.Sprintf("Open scheduler error: %v", opErr)
			s.mu.Unlock()
			s.RequestRedraw()
			return
		}
		sched = opSched
	}

	// Live telemetry poller ticker
	ticker := time.NewTicker(250 * time.Millisecond)
	tickerDone := make(chan struct{})

	go func() {
		defer close(tickerDone)
		for {
			select {
			case <-ctx.Done():
				return
			case now := <-ticker.C:
				elapsed := now.Sub(startTime)
				if elapsed > p.Duration && p.Duration > 0 {
					elapsed = p.Duration
				}
				progress := float32(0.0)
				if p.Duration > 0 {
					progress = float32(elapsed.Seconds() / p.Duration.Seconds())
					if progress > 1.0 {
						progress = 1.0
					}
				}

				s.mu.Lock()
				if s.RunState == StateRunning || s.RunState == StateDraining {
					s.Telemetry.Elapsed = elapsed
					s.Telemetry.Progress = progress

					// Append time series chart point
					sec := elapsed.Seconds()
					s.Telemetry.TimeSeries = append(s.Telemetry.TimeSeries, ChartPoint{
						TimestampSec: sec,
						TargetRPS:    p.TargetRPS(),
						CompletedRPS: s.Telemetry.CompletedRPS,
						P50MS:        s.Telemetry.P50MS,
						P95MS:        s.Telemetry.P95MS,
						P99MS:        s.Telemetry.P99MS,
					})
					// Keep max 240 chart points to bound memory
					if len(s.Telemetry.TimeSeries) > 240 {
						s.Telemetry.TimeSeries = s.Telemetry.TimeSeries[len(s.Telemetry.TimeSeries)-240:]
					}
				}
				s.mu.Unlock()
				s.RequestRedraw()
			}
		}
	}()

	agg, health, runErr := sched.Run(ctx)
	ticker.Stop()
	<-tickerDone

	endTime := time.Now().UTC()
	incomplete := (runErr != nil) || (ctx.Err() != nil)

	evaluationAggregate := agg
	if agg != nil && agg.Measured != nil {
		evaluationAggregate = agg.Measured
	}

	var thresholdResults []report.ThresholdResult
	allPassed := true
	if p != nil && len(p.Thresholds) > 0 && evaluationAggregate != nil {
		evalResults, passed, evalErr := threshold.EvaluateWithSteps(
			p.Thresholds,
			evaluationAggregate.ToThresholdSnapshot(),
			evaluationAggregate.ToStepThresholdSnapshots(),
			p.ToEvaluationContext(),
		)
		if evalErr == nil {
			allPassed = passed
			thresholdResults = threshold.ToReportResults(evalResults)
		}
	}

	finalRep := report.BuildReport(p, agg, health, startTime, endTime, incomplete, thresholdResults)

	s.mu.Lock()
	defer s.mu.Unlock()

	s.FinalReport = finalRep
	if runErr != nil {
		s.RunState = StateFailed
		s.ErrorMessage = runErr.Error()
	} else if !allPassed {
		s.RunState = StateCompleted
		s.ErrorMessage = "Completed with threshold assertion failures"
	} else {
		s.RunState = StateCompleted
	}

	// Update final telemetry metrics
	if evaluationAggregate != nil {
		s.Telemetry.CompletedTotal = evaluationAggregate.RequestCounts.Completed
		s.Telemetry.StartedTotal = evaluationAggregate.RequestCounts.Started
		s.Telemetry.DroppedTotal = evaluationAggregate.RequestCounts.Dropped
		s.Telemetry.CompletedRPS = evaluationAggregate.CompletedThroughput
		s.Telemetry.AchievedRPS = evaluationAggregate.AchievedStartRPS
		s.Telemetry.P50MS = evaluationAggregate.Latency.AllCompleted.P50MS
		s.Telemetry.P90MS = evaluationAggregate.Latency.AllCompleted.P90MS
		s.Telemetry.P95MS = evaluationAggregate.Latency.AllCompleted.P95MS
		s.Telemetry.P99MS = evaluationAggregate.Latency.AllCompleted.P99MS
		s.Telemetry.MaxMS = evaluationAggregate.Latency.AllCompleted.MaxMS
		s.Telemetry.StatusCodes = evaluationAggregate.StatusCodes
		s.Telemetry.Outcomes = evaluationAggregate.Outcomes
		s.Telemetry.ErrorSamples = evaluationAggregate.ErrorSamples
		s.Telemetry.ErrorRate = evaluationAggregate.ErrorRate
		s.Telemetry.ThresholdResults = thresholdResults
		s.Telemetry.Progress = 1.0
	}
	s.RequestRedraw()
}

// RunDiagnostics executes system readiness diagnostics.
func (s *State) RunDiagnostics(ctx context.Context) {
	s.mu.Lock()
	s.Doctor = nil
	s.mu.Unlock()
	s.RequestRedraw()

	rep := doctor.RunDiagnostics(ctx, doctor.Options{Timeout: 5 * time.Second})

	s.mu.Lock()
	s.Doctor = rep
	s.mu.Unlock()
	s.RequestRedraw()
}

// RunComparison executes report diff comparison.
func (s *State) RunComparison() {
	s.mu.Lock()
	defer s.mu.Unlock()

	var rep1, rep2 *report.Report
	var err error

	if s.Compare.BaselinePath != "" {
		rep1, err = compare.LoadReport(s.Compare.BaselinePath)
	} else if s.Compare.BaselineJSON != "" {
		rep1, err = loadReportFromBytes([]byte(s.Compare.BaselineJSON))
	} else {
		s.Compare.ErrorMessage = "Please provide Baseline report JSON or file path"
		return
	}
	if err != nil {
		s.Compare.ErrorMessage = fmt.Sprintf("Baseline error: %v", err)
		return
	}

	if s.Compare.CandidatePath != "" {
		rep2, err = compare.LoadReport(s.Compare.CandidatePath)
	} else if s.Compare.CandidateJSON != "" {
		rep2, err = loadReportFromBytes([]byte(s.Compare.CandidateJSON))
	} else {
		s.Compare.ErrorMessage = "Please provide Candidate report JSON or file path"
		return
	}
	if err != nil {
		s.Compare.ErrorMessage = fmt.Sprintf("Candidate error: %v", err)
		return
	}

	res, compErr := compare.Compare(rep1, rep2)
	if compErr != nil {
		s.Compare.ErrorMessage = fmt.Sprintf("Comparison error: %v", compErr)
		return
	}

	s.Compare.Result = res
	s.Compare.ErrorMessage = ""
	s.RequestRedraw()
}

func loadReportFromBytes(data []byte) (*report.Report, error) {
	tmpFile, err := os.CreateTemp("", "daegsa-rep-*.json")
	if err != nil {
		return nil, err
	}
	defer os.Remove(tmpFile.Name())
	if _, err := tmpFile.Write(data); err != nil {
		tmpFile.Close()
		return nil, err
	}
	tmpFile.Close()
	return compare.LoadReport(tmpFile.Name())
}
