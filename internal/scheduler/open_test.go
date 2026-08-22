package scheduler

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/charleszardd/daegsa/internal/auth"
	"github.com/charleszardd/daegsa/internal/clock"
	"github.com/charleszardd/daegsa/internal/config"
	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/executor"
	"github.com/charleszardd/daegsa/internal/metrics"
	"github.com/charleszardd/daegsa/internal/plan"
	"github.com/charleszardd/daegsa/internal/testtarget"
)

func createOpenTestPlan(
	targetURL string,
	rate float64,
	timeUnit time.Duration,
	maxInFlight int64,
	duration time.Duration,
	gracefulStop time.Duration,
) *plan.Plan {
	parsed, _ := url.Parse(targetURL)
	return &plan.Plan{
		Name:               "open-scheduler-test",
		SchemaVersion:      1,
		Fingerprint:        "test-fingerprint-open",
		TargetURL:          parsed,
		Method:             "GET",
		Headers:            make(http.Header),
		ExpectedStatuses:   []int{200},
		RequestTimeout:     5 * time.Second,
		ResponseBodyLimit:  1024 * 1024,
		RedirectPolicy:     "none",
		Model:              core.WorkloadModelOpen,
		Rate:               rate,
		TimeUnit:           timeUnit,
		MaxInFlight:        maxInFlight,
		Duration:           duration,
		GracefulStop:       gracefulStop,
		Treat429AsExpected: false,
		AllowedHosts:       []string{parsed.Hostname()},
	}
}

func TestNewOpenScheduler_Validation(t *testing.T) {
	parsed, _ := url.Parse("http://127.0.0.1:8080")
	validPlan := &plan.Plan{
		TargetURL:    parsed,
		Model:        core.WorkloadModelOpen,
		Rate:         10,
		TimeUnit:     time.Second,
		MaxInFlight:  50,
		Duration:     1 * time.Second,
		AllowedHosts: []string{"127.0.0.1"},
	}

	exec, err := executor.NewHTTPExecutor(validPlan)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	// 1. Nil plan
	if _, err := NewOpenScheduler(nil, exec, nil); !errors.Is(err, ErrInvalidPlan) {
		t.Errorf("expected ErrInvalidPlan for nil plan, got %v", err)
	}

	// 2. Nil executor
	if _, err := NewOpenScheduler(validPlan, nil, nil); !errors.Is(err, ErrInvalidExecutor) {
		t.Errorf("expected ErrInvalidExecutor for nil executor, got %v", err)
	}

	// 3. Incompatible model
	closedPlan := *validPlan
	closedPlan.Model = core.WorkloadModelClosed
	if _, err := NewOpenScheduler(&closedPlan, exec, nil); !errors.Is(err, ErrIncompatibleModel) {
		t.Errorf("expected ErrIncompatibleModel, got %v", err)
	}

	// 4. Rate <= 0
	zeroRatePlan := *validPlan
	zeroRatePlan.Rate = 0
	if _, err := NewOpenScheduler(&zeroRatePlan, exec, nil); !errors.Is(err, ErrInvalidRate) {
		t.Errorf("expected ErrInvalidRate, got %v", err)
	}

	// 5. TimeUnit <= 0
	zeroTUPlan := *validPlan
	zeroTUPlan.TimeUnit = 0
	if _, err := NewOpenScheduler(&zeroTUPlan, exec, nil); !errors.Is(err, ErrInvalidTimeUnit) {
		t.Errorf("expected ErrInvalidTimeUnit, got %v", err)
	}

	// 6. MaxInFlight <= 0
	zeroMIFPlan := *validPlan
	zeroMIFPlan.MaxInFlight = 0
	if _, err := NewOpenScheduler(&zeroMIFPlan, exec, nil); !errors.Is(err, ErrInvalidMaxInFlight) {
		t.Errorf("expected ErrInvalidMaxInFlight, got %v", err)
	}

	// 7. Duration <= 0
	zeroDurPlan := *validPlan
	zeroDurPlan.Duration = 0
	if _, err := NewOpenScheduler(&zeroDurPlan, exec, nil); !errors.Is(err, ErrZeroDuration) {
		t.Errorf("expected ErrZeroDuration, got %v", err)
	}

	// 8. Success
	sched, err := NewOpenScheduler(validPlan, exec, nil)
	if err != nil {
		t.Fatalf("unexpected error creating valid OpenScheduler: %v", err)
	}
	if sched.InFlight() != 0 {
		t.Errorf("expected initial in-flight 0, got %d", sched.InFlight())
	}
	if sched.PeakInFlight() != 0 {
		t.Errorf("expected initial peak in-flight 0, got %d", sched.PeakInFlight())
	}
	if sched.LifecycleState() != core.StateInitialized {
		t.Errorf("expected initial state StateInitialized, got %s", sched.LifecycleState())
	}
}

func TestOpenScheduler_ExactIntervalSpacing_ControllableClock(t *testing.T) {
	ts := testtarget.NewServer()
	defer ts.Close()

	// 10 req/s, 1s duration -> 10 total requests spaced at 100ms intervals
	p := createOpenTestPlan(ts.URL(), 10, time.Second, 50, 1*time.Second, 500*time.Millisecond)
	exec, err := executor.NewHTTPExecutor(p)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	initialTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	clk := clock.NewControllableClock(initialTime)

	sched, err := NewOpenScheduler(p, exec, clk)
	if err != nil {
		t.Fatalf("failed to create open scheduler: %v", err)
	}

	type runResult struct {
		agg    *metrics.AggregatedMetrics
		health *metrics.GeneratorHealth
		err    error
	}

	resChan := make(chan runResult, 1)
	go func() {
		agg, health, runErr := sched.Run(context.Background())
		resChan <- runResult{agg: agg, health: health, err: runErr}
	}()

	// Note: clk has 1 background ticker from healthSampler.
	// When the scheduler sets its interval timer, active timer count becomes 2.
	for i := 0; i < 10; i++ {
		clk.BlockUntilTimers(2)
		clk.Advance(100 * time.Millisecond)
	}

	res := <-resChan
	if res.err != nil {
		t.Fatalf("scheduler run failed: %v", res.err)
	}

	agg := res.agg
	if agg.RequestCounts.Planned != 10 {
		t.Errorf("expected 10 planned requests, got %d", agg.RequestCounts.Planned)
	}
	if agg.RequestCounts.Started != 10 {
		t.Errorf("expected 10 started requests, got %d", agg.RequestCounts.Started)
	}
	if agg.RequestCounts.Completed != 10 {
		t.Errorf("expected 10 completed requests, got %d", agg.RequestCounts.Completed)
	}
	if agg.RequestCounts.Dropped != 0 {
		t.Errorf("expected 0 dropped requests, got %d", agg.RequestCounts.Dropped)
	}
	if agg.Outcomes[core.OutcomeSuccess] != 10 {
		t.Errorf("expected 10 success outcomes, got %d", agg.Outcomes[core.OutcomeSuccess])
	}
}

func TestOpenScheduler_FractionalSpacing_ControllableClock(t *testing.T) {
	ts := testtarget.NewServer()
	defer ts.Close()

	// 3 req/s, 1s duration -> interval = 333.333ms, 3 requests planned
	p := createOpenTestPlan(ts.URL(), 3, time.Second, 10, 1*time.Second, 500*time.Millisecond)
	exec, err := executor.NewHTTPExecutor(p)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	initialTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	clk := clock.NewControllableClock(initialTime)
	sched, err := NewOpenScheduler(p, exec, clk)
	if err != nil {
		t.Fatalf("failed to create open scheduler: %v", err)
	}

	resChan := make(chan *metrics.AggregatedMetrics, 1)
	go func() {
		agg, _, _ := sched.Run(context.Background())
		resChan <- agg
	}()

	// 3 ticks in 1 second:
	// Tick 0 at T0, timer for T0 + 333.333ms
	// Advance by 334ms -> Tick 1, timer for T0 + 666.666ms
	// Advance by 334ms -> Tick 2, timer for T0 + 1000ms
	// Advance by 334ms -> Duration reached (1002ms >= 1000ms)
	for i := 0; i < 3; i++ {
		clk.BlockUntilTimers(2)
		clk.Advance(334 * time.Millisecond)
	}

	agg := <-resChan
	if agg.RequestCounts.Planned != 3 {
		t.Errorf("expected 3 planned requests for 3 req/s in 1s, got %d", agg.RequestCounts.Planned)
	}
	if agg.RequestCounts.Completed != 3 {
		t.Errorf("expected 3 completed requests, got %d", agg.RequestCounts.Completed)
	}
}

func TestOpenScheduler_StrictMaxInFlightSaturationAndDrops(t *testing.T) {
	ts := testtarget.NewServer()
	defer ts.Close()

	// 100 req/s, 100ms duration (10 total ticks at 10ms interval), max_in_flight = 3
	// Target hangs so in-flight capacity saturates immediately after 3 requests
	p := createOpenTestPlan(ts.URL()+"/?hang=true", 100, time.Second, 3, 100*time.Millisecond, 50*time.Millisecond)
	p.RequestTimeout = 5 * time.Second

	exec, err := executor.NewHTTPExecutor(p)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	initialTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	clk := clock.NewControllableClock(initialTime)
	sched, err := NewOpenScheduler(p, exec, clk)
	if err != nil {
		t.Fatalf("failed to create open scheduler: %v", err)
	}

	type runRes struct {
		agg    *metrics.AggregatedMetrics
		health *metrics.GeneratorHealth
	}
	resChan := make(chan runRes, 1)
	go func() {
		agg, health, _ := sched.Run(context.Background())
		resChan <- runRes{agg: agg, health: health}
	}()

	// Step through the 10 scheduling ticks (10ms each)
	for i := 0; i < 10; i++ {
		clk.BlockUntilTimers(2)
		clk.Advance(10 * time.Millisecond)
	}

	// Advance through graceful stop window (50ms) so hanging requests are canceled
	clk.BlockUntilTimers(2)
	clk.Advance(100 * time.Millisecond)

	out := <-resChan
	agg := out.agg

	// Planned: 10, Started: 3 (max_in_flight), Dropped: 7
	if agg.RequestCounts.Planned != 10 {
		t.Errorf("expected 10 planned, got %d", agg.RequestCounts.Planned)
	}
	if agg.RequestCounts.Started != 3 {
		t.Errorf("expected 3 started, got %d", agg.RequestCounts.Started)
	}
	if agg.RequestCounts.Dropped != 7 {
		t.Errorf("expected 7 dropped, got %d", agg.RequestCounts.Dropped)
	}
	if agg.Outcomes[core.OutcomeDropped] != 7 {
		t.Errorf("expected 7 OutcomeDropped, got %d", agg.Outcomes[core.OutcomeDropped])
	}

	// Invariant: Planned == Started + Dropped
	if agg.RequestCounts.Planned != agg.RequestCounts.Started+agg.RequestCounts.Dropped {
		t.Errorf("invariant violated: Planned (%d) != Started (%d) + Dropped (%d)",
			agg.RequestCounts.Planned, agg.RequestCounts.Started, agg.RequestCounts.Dropped)
	}

	// Health warnings must contain drop notification
	if len(out.health.SaturationWarnings) == 0 {
		t.Errorf("expected saturation warnings for dropped requests")
	}
}

func TestOpenScheduler_AntiCatchUpBurstProgression(t *testing.T) {
	ts := testtarget.NewServer()
	defer ts.Close()

	// 10 req/s (100ms interval), 2s duration, max_in_flight 50
	p := createOpenTestPlan(ts.URL(), 10, time.Second, 50, 2*time.Second, 500*time.Millisecond)
	exec, err := executor.NewHTTPExecutor(p)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	initialTime := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	clk := clock.NewControllableClock(initialTime)
	sched, err := NewOpenScheduler(p, exec, clk)
	if err != nil {
		t.Fatalf("failed to create open scheduler: %v", err)
	}

	resChan := make(chan *metrics.GeneratorHealth, 1)
	go func() {
		_, health, _ := sched.Run(context.Background())
		resChan <- health
	}()

	// Wait for tick 0 to execute and scheduler to register timer for tick 1 (T0 + 100ms)
	clk.BlockUntilTimers(2)

	// Jump forward 500ms in a single leap (generator freeze)
	clk.Advance(500 * time.Millisecond)

	// Finish remaining duration (up to 2s)
	for i := 0; i < 15; i++ {
		clk.BlockUntilTimers(2)
		clk.Advance(100 * time.Millisecond)
	}

	health := <-resChan
	// Lag must have been recorded (400ms)
	if health.SchedulerLagMaxMS < 300.0 {
		t.Errorf("expected scheduler lag >= 300ms after jump, got %f ms", health.SchedulerLagMaxMS)
	}
	// Warning must be generated
	hasLagWarning := false
	for _, w := range health.SaturationWarnings {
		if w == "scheduler lag exceeded 50ms" {
			hasLagWarning = true
			break
		}
	}
	if !hasLagWarning {
		t.Errorf("expected 'scheduler lag exceeded 50ms' warning, got %v", health.SaturationWarnings)
	}
}

func TestOpenScheduler_GracefulDrain(t *testing.T) {
	ts := testtarget.NewServer()
	defer ts.Close()

	p := createOpenTestPlan(ts.URL()+"/?delay=30ms", 10, time.Second, 10, 100*time.Millisecond, 1*time.Second)
	exec, err := executor.NewHTTPExecutor(p)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	sched, err := NewOpenScheduler(p, exec, clock.NewRealClock())
	if err != nil {
		t.Fatalf("failed to create scheduler: %v", err)
	}

	agg, _, err := sched.Run(context.Background())
	if err != nil {
		t.Fatalf("scheduler run failed: %v", err)
	}

	if sched.LifecycleState() != core.StateCompleted {
		t.Errorf("expected StateCompleted, got %s", sched.LifecycleState())
	}
	if agg.RequestCounts.Completed == 0 {
		t.Errorf("expected completed requests > 0")
	}
	if agg.RequestCounts.Canceled != 0 {
		t.Errorf("expected 0 canceled requests on graceful drain, got %d", agg.RequestCounts.Canceled)
	}
}

func TestOpenScheduler_GracefulStopTimeoutExpiration(t *testing.T) {
	ts := testtarget.NewServer()
	defer ts.Close()

	// Target hangs
	p := createOpenTestPlan(ts.URL()+"/?hang=true", 10, time.Second, 10, 50*time.Millisecond, 50*time.Millisecond)
	p.RequestTimeout = 5 * time.Second

	exec, err := executor.NewHTTPExecutor(p)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	sched, err := NewOpenScheduler(p, exec, clock.NewRealClock())
	if err != nil {
		t.Fatalf("failed to create scheduler: %v", err)
	}

	agg, _, err := sched.Run(context.Background())
	if err != nil {
		t.Fatalf("scheduler run failed: %v", err)
	}

	// Requests must be canceled after graceful stop timeout
	if agg.Outcomes[core.OutcomeCanceled] == 0 {
		t.Errorf("expected canceled outcomes due to graceful stop timeout, got %d", agg.Outcomes[core.OutcomeCanceled])
	}
	if agg.RequestCounts.Canceled == 0 {
		t.Errorf("expected canceled request count > 0, got %d", agg.RequestCounts.Canceled)
	}
}

func TestOpenScheduler_HardCancellation(t *testing.T) {
	ts := testtarget.NewServer()
	defer ts.Close()

	p := createOpenTestPlan(ts.URL()+"/?delay=100ms", 20, time.Second, 20, 5*time.Second, 1*time.Second)
	exec, err := executor.NewHTTPExecutor(p)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	sched, err := NewOpenScheduler(p, exec, clock.NewRealClock())
	if err != nil {
		t.Fatalf("failed to create scheduler: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	agg, _, err := sched.Run(ctx)
	if err != nil {
		t.Fatalf("unexpected run error on cancel: %v", err)
	}

	if agg == nil {
		t.Fatalf("expected non-nil aggregated metrics")
	}
}

func TestOpenScheduler_Integration_Fast200OK(t *testing.T) {
	ts := testtarget.NewServer()
	defer ts.Close()

	// 100 req/s for 500ms -> ~50 requests
	p := createOpenTestPlan(ts.URL(), 100, time.Second, 50, 500*time.Millisecond, 1*time.Second)
	exec, err := executor.NewHTTPExecutor(p)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	sched, err := NewOpenScheduler(p, exec, clock.NewRealClock())
	if err != nil {
		t.Fatalf("failed to create scheduler: %v", err)
	}

	agg, health, err := sched.Run(context.Background())
	if err != nil {
		t.Fatalf("scheduler run failed: %v", err)
	}

	if agg.RequestCounts.Completed == 0 {
		t.Fatalf("expected completed requests > 0, got 0")
	}
	if agg.RequestCounts.Dropped != 0 {
		t.Errorf("expected 0 dropped requests on fast 200 OK, got %d", agg.RequestCounts.Dropped)
	}
	if agg.Outcomes[core.OutcomeSuccess] != agg.RequestCounts.Completed {
		t.Errorf("expected 100%% success, got %d of %d", agg.Outcomes[core.OutcomeSuccess], agg.RequestCounts.Completed)
	}
	if health == nil {
		t.Errorf("expected non-nil health diagnostics")
	}
}

func TestOpenScheduler_Integration_HigherRateTarget(t *testing.T) {
	ts := testtarget.NewServer()
	defer ts.Close()

	// 500 req/s for 200ms -> ~100 requests
	p := createOpenTestPlan(ts.URL(), 500, time.Second, 100, 200*time.Millisecond, 1*time.Second)
	exec, err := executor.NewHTTPExecutor(p)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	sched, err := NewOpenScheduler(p, exec, clock.NewRealClock())
	if err != nil {
		t.Fatalf("failed to create scheduler: %v", err)
	}

	agg, _, err := sched.Run(context.Background())
	if err != nil {
		t.Fatalf("scheduler run failed: %v", err)
	}

	if agg.RequestCounts.Completed == 0 {
		t.Fatalf("expected completed requests > 0")
	}
	if agg.RequestCounts.Dropped != 0 {
		t.Errorf("expected 0 dropped requests, got %d", agg.RequestCounts.Dropped)
	}
}

func TestOpenScheduler_Integration_SlowTargetOverloadAndDroppedRequests(t *testing.T) {
	ts := testtarget.NewServer()
	defer ts.Close()

	// 100 req/s for 1s, delay=500ms, max_in_flight=10
	// Concurrency saturates at 10 requests, causing dropped requests for subsequent ticks
	targetURL := ts.URL() + "/?delay=500ms"
	p := createOpenTestPlan(targetURL, 100, time.Second, 10, 1*time.Second, 1*time.Second)
	exec, err := executor.NewHTTPExecutor(p)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	sched, err := NewOpenScheduler(p, exec, clock.NewRealClock())
	if err != nil {
		t.Fatalf("failed to create scheduler: %v", err)
	}

	agg, health, err := sched.Run(context.Background())
	if err != nil {
		t.Fatalf("scheduler run failed: %v", err)
	}

	if agg.RequestCounts.Dropped == 0 {
		t.Fatalf("expected dropped requests > 0 due to slow target overload, got 0")
	}
	if agg.Outcomes[core.OutcomeDropped] != agg.RequestCounts.Dropped {
		t.Errorf("expected OutcomeDropped (%d) == Dropped count (%d)",
			agg.Outcomes[core.OutcomeDropped], agg.RequestCounts.Dropped)
	}
	if agg.RequestCounts.Planned != agg.RequestCounts.Started+agg.RequestCounts.Dropped {
		t.Errorf("invariant violated: Planned (%d) != Started (%d) + Dropped (%d)",
			agg.RequestCounts.Planned, agg.RequestCounts.Started, agg.RequestCounts.Dropped)
	}
	if agg.AchievedStartRPS <= 0 || agg.CompletedThroughput <= 0 {
		t.Errorf("expected positive start rate and throughput, got start=%f, comp=%f",
			agg.AchievedStartRPS, agg.CompletedThroughput)
	}
	if len(health.SaturationWarnings) == 0 {
		t.Errorf("expected health saturation warnings for slow target overload")
	}
}

func TestOpenScheduler_Integration_429RateLimitedTarget(t *testing.T) {
	rl := testtarget.NewRateLimiter(testtarget.RateLimiterConfig{
		RequestsPerWindow: 0, // Reject all with 429
		Window:            1 * time.Second,
		HeaderStyle:       testtarget.RateLimitHeaderStyleAll,
	})
	ts := testtarget.NewServer(testtarget.WithRateLimiter(rl))
	defer ts.Close()

	p := createOpenTestPlan(ts.URL(), 50, time.Second, 20, 200*time.Millisecond, 1*time.Second)
	exec, err := executor.NewHTTPExecutor(p)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	sched, err := NewOpenScheduler(p, exec, clock.NewRealClock())
	if err != nil {
		t.Fatalf("failed to create scheduler: %v", err)
	}

	agg, _, err := sched.Run(context.Background())
	if err != nil {
		t.Fatalf("scheduler run failed: %v", err)
	}

	if agg.Outcomes[core.OutcomeRateLimited] == 0 {
		t.Errorf("expected OutcomeRateLimited > 0, got %d", agg.Outcomes[core.OutcomeRateLimited])
	}
	if agg.RateLimits.Observed429Count == 0 {
		t.Errorf("expected Observed429Count > 0, got %d", agg.RateLimits.Observed429Count)
	}
}

func TestOpenScheduler_Integration_ServerErrorAndDisconnect(t *testing.T) {
	ts := testtarget.NewServer()
	defer ts.Close()

	// Mode 1: 500 status code
	targetURL := ts.URL() + "/?status=500"
	p := createOpenTestPlan(targetURL, 20, time.Second, 10, 100*time.Millisecond, 1*time.Second)
	exec, err := executor.NewHTTPExecutor(p)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	sched, err := NewOpenScheduler(p, exec, clock.NewRealClock())
	if err != nil {
		t.Fatalf("failed to create scheduler: %v", err)
	}

	agg, _, err := sched.Run(context.Background())
	if err != nil {
		t.Fatalf("scheduler run failed: %v", err)
	}

	if agg.Outcomes[core.OutcomeUnexpectedStatus] == 0 {
		t.Errorf("expected OutcomeUnexpectedStatus > 0 for 500 status, got %d", agg.Outcomes[core.OutcomeUnexpectedStatus])
	}
	if sched.InFlight() != 0 {
		t.Errorf("expected in-flight count to return to 0 after run, got %d", sched.InFlight())
	}
}

func TestOpenScheduler_TokenPool_Deterministic(t *testing.T) {
	tokens := []string{"token-0", "token-1", "token-2", "token-3", "token-4"}
	const workerLanes = 10

	var requestMu sync.Mutex
	observedWorkerTokens := make(map[int]string, workerLanes)
	validationErrors := make(chan string, workerLanes)
	firstWaveArrived := make(chan struct{})
	var firstWaveOnce sync.Once

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		token := strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
		workerCookie, cookieErr := request.Cookie("worker_id")
		if cookieErr != nil {
			validationErrors <- "missing lane identity cookie: " + cookieErr.Error()
			<-request.Context().Done()
			return
		}
		workerID, parseErr := strconv.Atoi(workerCookie.Value)
		if parseErr != nil || workerID < 0 || workerID >= workerLanes {
			validationErrors <- "invalid lane identity cookie " + workerCookie.Value
			<-request.Context().Done()
			return
		}
		expectedToken := tokens[workerID%len(tokens)]
		if token != expectedToken {
			validationErrors <- fmt.Sprintf("lane %d token = %q, want %q", workerID, token, expectedToken)
			<-request.Context().Done()
			return
		}

		requestMu.Lock()
		observedWorkerTokens[workerID] = token
		if len(observedWorkerTokens) == workerLanes {
			firstWaveOnce.Do(func() { close(firstWaveArrived) })
		}
		requestMu.Unlock()

		<-request.Context().Done()
	}))
	defer server.Close()

	authenticator, err := auth.NewAuthenticator(&config.AuthConfig{
		Type:      config.AuthTypeTokenPool,
		TokenPool: tokens,
	})
	if err != nil {
		t.Fatalf("failed to create authenticator: %v", err)
	}

	p := createOpenTestPlan(server.URL, 10000, time.Second, workerLanes, 5*time.Second, 1*time.Second)
	p.Authenticator = authenticator
	p.KnownSecrets = tokens
	jarManager, jarErr := auth.NewVUJarManager(true, workerLanes)
	if jarErr != nil {
		t.Fatalf("failed to create worker identity jars: %v", jarErr)
	}
	parsedTargetURL, parseErr := url.Parse(server.URL)
	if parseErr != nil {
		t.Fatalf("failed to parse target URL: %v", parseErr)
	}
	for workerID := 0; workerID < workerLanes; workerID++ {
		jarManager.GetJar(workerID).SetCookies(parsedTargetURL, []*http.Cookie{{
			Name:  "worker_id",
			Value: strconv.Itoa(workerID),
			Path:  "/",
		}})
	}
	p.JarManager = jarManager
	p.CookieJarEnabled = true
	exec, err := executor.NewHTTPExecutor(p)
	if err != nil {
		t.Fatalf("failed to create executor: %v", err)
	}
	defer exec.Close()

	scheduler, err := NewOpenScheduler(p, exec, clock.NewRealClock())
	if err != nil {
		t.Fatalf("failed to create scheduler: %v", err)
	}

	runContext, cancelRun := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() {
		_, _, runErr := scheduler.Run(runContext)
		runDone <- runErr
	}()

	select {
	case <-firstWaveArrived:
		cancelRun()
	case validationErr := <-validationErrors:
		cancelRun()
		t.Fatal(validationErr)
	case <-time.After(schedulerContractTimeout):
		cancelRun()
		t.Fatal("timed out waiting for every open-model worker lane to issue its first request")
	}

	if runErr := <-runDone; runErr != nil {
		t.Fatalf("scheduler run failed: %v", runErr)
	}

	requestMu.Lock()
	defer requestMu.Unlock()
	for workerID := 0; workerID < workerLanes; workerID++ {
		expectedToken := tokens[workerID%len(tokens)]
		if got := observedWorkerTokens[workerID]; got != expectedToken {
			t.Errorf("lane %d token = %q, want %q", workerID, got, expectedToken)
		}
	}
}
