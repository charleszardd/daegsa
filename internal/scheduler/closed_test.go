package scheduler

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charleszardd/daegsa/internal/auth"
	"github.com/charleszardd/daegsa/internal/clock"
	"github.com/charleszardd/daegsa/internal/config"
	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/executor"
	"github.com/charleszardd/daegsa/internal/plan"
	"github.com/charleszardd/daegsa/internal/testtarget"
)

const schedulerContractTimeout = 3 * time.Second

func createTestPlan(targetURL string, users int64, duration, thinkTime, gracefulStop time.Duration) *plan.Plan {
	parsed, _ := url.Parse(targetURL)
	return &plan.Plan{
		Name:               "closed-scheduler-test",
		SchemaVersion:      1,
		Fingerprint:        "test-fingerprint",
		TargetURL:          parsed,
		Method:             "GET",
		Headers:            make(http.Header),
		ExpectedStatuses:   []int{200},
		RequestTimeout:     5 * time.Second,
		ResponseBodyLimit:  1024 * 1024,
		RedirectPolicy:     "none",
		Model:              core.WorkloadModelClosed,
		Duration:           duration,
		GracefulStop:       gracefulStop,
		Users:              users,
		ThinkTime:          thinkTime,
		Treat429AsExpected: false,
		AllowedHosts:       []string{parsed.Hostname()},
	}
}

func TestClosedScheduler_Integration_200OK(t *testing.T) {
	ts := testtarget.NewServer()
	defer ts.Close()

	p := createTestPlan(ts.URL(), 5, 200*time.Millisecond, 10*time.Millisecond, 1*time.Second)
	exec, err := executor.NewHTTPExecutor(p)
	if err != nil {
		t.Fatalf("failed to create http executor: %v", err)
	}
	defer exec.Close()

	sched, err := NewClosedScheduler(p, exec, clock.NewRealClock())
	if err != nil {
		t.Fatalf("failed to create closed scheduler: %v", err)
	}

	agg, health, err := sched.Run(context.Background())
	if err != nil {
		t.Fatalf("scheduler run failed: %v", err)
	}

	if agg.RequestCounts.Completed == 0 {
		t.Errorf("expected completed requests > 0, got 0")
	}
	if agg.Outcomes[core.OutcomeSuccess] != agg.RequestCounts.Completed {
		t.Errorf("expected 100%% success, got %d successes out of %d completed",
			agg.Outcomes[core.OutcomeSuccess], agg.RequestCounts.Completed)
	}
	if agg.RequestCounts.Started != agg.RequestCounts.Completed {
		t.Errorf("reconciliation mismatch: started %d != completed %d",
			agg.RequestCounts.Started, agg.RequestCounts.Completed)
	}
	if agg.AchievedStartRPS <= 0 || agg.CompletedThroughput <= 0 {
		t.Errorf("expected positive throughput, got start=%f, comp=%f",
			agg.AchievedStartRPS, agg.CompletedThroughput)
	}
	if health == nil {
		t.Errorf("expected non-nil generator health")
	}
}

func TestClosedScheduler_Integration_DelayedTarget(t *testing.T) {
	ts := testtarget.NewServer()
	defer ts.Close()

	targetURL := ts.URL() + "/?delay=30ms"
	p := createTestPlan(targetURL, 3, 150*time.Millisecond, 0, 1*time.Second)
	exec, err := executor.NewHTTPExecutor(p)
	if err != nil {
		t.Fatalf("failed to create http executor: %v", err)
	}
	defer exec.Close()

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
	if agg.Latency.AllCompleted.MinMS < 20.0 {
		t.Errorf("expected min latency >= 20ms due to delay, got %f ms", agg.Latency.AllCompleted.MinMS)
	}
}

func TestClosedScheduler_Integration_429RateLimited(t *testing.T) {
	rl := testtarget.NewRateLimiter(testtarget.RateLimiterConfig{
		RequestsPerWindow: 0, // Reject all
		Window:            1 * time.Second,
		HeaderStyle:       testtarget.RateLimitHeaderStyleAll,
	})
	ts := testtarget.NewServer(testtarget.WithRateLimiter(rl))
	defer ts.Close()

	p := createTestPlan(ts.URL(), 2, 100*time.Millisecond, 10*time.Millisecond, 1*time.Second)
	exec, err := executor.NewHTTPExecutor(p)
	if err != nil {
		t.Fatalf("failed to create http executor: %v", err)
	}
	defer exec.Close()

	sched, err := NewClosedScheduler(p, exec, clock.NewRealClock())
	if err != nil {
		t.Fatalf("failed to create closed scheduler: %v", err)
	}

	agg, _, err := sched.Run(context.Background())
	if err != nil {
		t.Fatalf("scheduler run failed: %v", err)
	}

	if agg.Outcomes[core.OutcomeRateLimited] == 0 {
		t.Errorf("expected rate limited outcomes > 0, got %d", agg.Outcomes[core.OutcomeRateLimited])
	}
	if agg.RateLimits.Observed429Count == 0 {
		t.Errorf("expected observed 429 count > 0, got %d", agg.RateLimits.Observed429Count)
	}
}

func TestClosedScheduler_Integration_500ServerError(t *testing.T) {
	ts := testtarget.NewServer()
	defer ts.Close()

	targetURL := ts.URL() + "/?status=500"
	p := createTestPlan(targetURL, 2, 100*time.Millisecond, 0, 1*time.Second)
	exec, err := executor.NewHTTPExecutor(p)
	if err != nil {
		t.Fatalf("failed to create http executor: %v", err)
	}
	defer exec.Close()

	sched, err := NewClosedScheduler(p, exec, clock.NewRealClock())
	if err != nil {
		t.Fatalf("failed to create closed scheduler: %v", err)
	}

	agg, _, err := sched.Run(context.Background())
	if err != nil {
		t.Fatalf("scheduler run failed: %v", err)
	}

	if agg.Outcomes[core.OutcomeUnexpectedStatus] == 0 {
		t.Errorf("expected unexpected status outcomes > 0, got %d", agg.Outcomes[core.OutcomeUnexpectedStatus])
	}
	if agg.StatusCodes["500"] == 0 {
		t.Errorf("expected status 500 count > 0, got %d", agg.StatusCodes["500"])
	}
}

func TestClosedScheduler_HardCancellation(t *testing.T) {
	ts := testtarget.NewServer()
	defer ts.Close()

	p := createTestPlan(ts.URL()+"/?delay=100ms", 4, 10*time.Second, 10*time.Millisecond, 1*time.Second)
	exec, err := executor.NewHTTPExecutor(p)
	if err != nil {
		t.Fatalf("failed to create http executor: %v", err)
	}
	defer exec.Close()

	sched, err := NewClosedScheduler(p, exec, clock.NewRealClock())
	if err != nil {
		t.Fatalf("failed to create closed scheduler: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	agg, _, err := sched.Run(ctx)
	if err != nil {
		t.Fatalf("scheduler run unexpected error on cancel: %v", err)
	}

	if sched.LifecycleState() != core.StateCompleted {
		t.Errorf("expected state completed after cancellation, got %s", sched.LifecycleState())
	}
	if agg == nil {
		t.Fatalf("expected non-nil aggregated metrics")
	}
}

func TestClosedScheduler_GracefulStopTimeout(t *testing.T) {
	ts := testtarget.NewServer()
	defer ts.Close()

	// Target hangs on request
	targetURL := ts.URL() + "/?hang=true"
	p := createTestPlan(targetURL, 2, 50*time.Millisecond, 0, 50*time.Millisecond)
	p.RequestTimeout = 5 * time.Second

	exec, err := executor.NewHTTPExecutor(p)
	if err != nil {
		t.Fatalf("failed to create http executor: %v", err)
	}
	defer exec.Close()

	sched, err := NewClosedScheduler(p, exec, clock.NewRealClock())
	if err != nil {
		t.Fatalf("failed to create closed scheduler: %v", err)
	}

	agg, _, err := sched.Run(context.Background())
	if err != nil {
		t.Fatalf("scheduler run failed: %v", err)
	}

	// Because requests hang past graceful stop, they should be canceled
	if agg.Outcomes[core.OutcomeCanceled] == 0 {
		t.Errorf("expected canceled outcomes due to graceful stop timeout, got %d", agg.Outcomes[core.OutcomeCanceled])
	}
}

func TestClosedScheduler_ConcurrencyInvariant(t *testing.T) {
	ts := testtarget.NewServer()
	defer ts.Close()

	const users = 8
	var maxConcurrency atomic.Int64

	p := createTestPlan(ts.URL()+"/?delay=10ms", users, 100*time.Millisecond, 0, 1*time.Second)
	exec, err := executor.NewHTTPExecutor(p)
	if err != nil {
		t.Fatalf("failed to create http executor: %v", err)
	}
	defer exec.Close()

	sched, err := NewClosedScheduler(p, exec, clock.NewRealClock())
	if err != nil {
		t.Fatalf("failed to create closed scheduler: %v", err)
	}

	// Poll in-flight count in background
	stopPolling := make(chan struct{})
	go func() {
		ticker := time.NewTicker(1 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopPolling:
				return
			case <-ticker.C:
				current := sched.InFlight()
				for {
					old := maxConcurrency.Load()
					if current <= old || maxConcurrency.CompareAndSwap(old, current) {
						break
					}
				}
			}
		}
	}()

	_, _, err = sched.Run(context.Background())
	close(stopPolling)

	if err != nil {
		t.Fatalf("scheduler run failed: %v", err)
	}

	if maxConcurrency.Load() > users {
		t.Errorf("concurrency invariant violated: max concurrency %d > %d users",
			maxConcurrency.Load(), users)
	}
}

func TestClosedScheduler_TokenPool_Deterministic(t *testing.T) {
	tokens := []string{"token-alpha", "token-beta", "token-gamma"}
	const users = 6

	var requestMu sync.Mutex
	observedWorkerTokens := make(map[int]string, users)
	validationErrors := make(chan string, users)
	firstWaveArrived := make(chan struct{})
	var firstWaveOnce sync.Once

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		token := strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
		workerCookie, cookieErr := request.Cookie("worker_id")
		if cookieErr != nil {
			validationErrors <- "missing VU identity cookie: " + cookieErr.Error()
			<-request.Context().Done()
			return
		}
		workerID, parseErr := strconv.Atoi(workerCookie.Value)
		if parseErr != nil || workerID < 0 || workerID >= users {
			validationErrors <- "invalid VU identity cookie " + workerCookie.Value
			<-request.Context().Done()
			return
		}
		expectedToken := tokens[workerID%len(tokens)]
		if token != expectedToken {
			validationErrors <- fmt.Sprintf("VU %d token = %q, want %q", workerID, token, expectedToken)
			<-request.Context().Done()
			return
		}

		requestMu.Lock()
		observedWorkerTokens[workerID] = token
		if len(observedWorkerTokens) == users {
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

	p := createTestPlan(server.URL, users, 5*time.Second, 0, 1*time.Second)
	p.Authenticator = authenticator
	p.KnownSecrets = tokens
	jarManager, jarErr := auth.NewVUJarManager(true, users)
	if jarErr != nil {
		t.Fatalf("failed to create worker identity jars: %v", jarErr)
	}
	parsedTargetURL, parseErr := url.Parse(server.URL)
	if parseErr != nil {
		t.Fatalf("failed to parse target URL: %v", parseErr)
	}
	for workerID := 0; workerID < users; workerID++ {
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
		t.Fatalf("failed to create http executor: %v", err)
	}
	defer exec.Close()

	scheduler, err := NewClosedScheduler(p, exec, clock.NewRealClock())
	if err != nil {
		t.Fatalf("failed to create closed scheduler: %v", err)
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
		t.Fatal("timed out waiting for every closed-model VU to issue its first request")
	}

	if runErr := <-runDone; runErr != nil {
		t.Fatalf("scheduler run failed: %v", runErr)
	}

	requestMu.Lock()
	defer requestMu.Unlock()
	for workerID := 0; workerID < users; workerID++ {
		expectedToken := tokens[workerID%len(tokens)]
		if got := observedWorkerTokens[workerID]; got != expectedToken {
			t.Errorf("VU %d token = %q, want %q", workerID, got, expectedToken)
		}
	}
}

func TestClosedScheduler_CookieJar_Isolation(t *testing.T) {
	const users = 5
	tokens := []string{"vu-0", "vu-1", "vu-2", "vu-3", "vu-4"}

	var stateMu sync.Mutex
	persistedSessions := make(map[string]bool, users)
	allSessionsPersisted := make(chan struct{})
	var allSessionsOnce sync.Once
	validationErrors := make(chan string, users)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		token := strings.TrimPrefix(request.Header.Get("Authorization"), "Bearer ")
		expectedSession := "session-" + token
		sessionCookie, cookieErr := request.Cookie("session")
		if cookieErr == http.ErrNoCookie {
			http.SetCookie(w, &http.Cookie{Name: "session", Value: expectedSession, Path: "/"})
			w.WriteHeader(http.StatusOK)
			return
		}
		if cookieErr != nil {
			validationErrors <- cookieErr.Error()
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if sessionCookie.Value != expectedSession {
			validationErrors <- "token " + token + " received another VU's session cookie"
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		stateMu.Lock()
		persistedSessions[token] = true
		if len(persistedSessions) == users {
			allSessionsOnce.Do(func() { close(allSessionsPersisted) })
		}
		stateMu.Unlock()
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
	jarManager, err := auth.NewVUJarManager(true, users)
	if err != nil {
		t.Fatalf("failed to create cookie jar manager: %v", err)
	}

	p := createTestPlan(server.URL, users, 5*time.Second, 0, 1*time.Second)
	p.Authenticator = authenticator
	p.KnownSecrets = tokens
	p.JarManager = jarManager
	p.CookieJarEnabled = true

	exec, err := executor.NewHTTPExecutor(p)
	if err != nil {
		t.Fatalf("failed to create http executor: %v", err)
	}
	defer exec.Close()

	scheduler, err := NewClosedScheduler(p, exec, clock.NewRealClock())
	if err != nil {
		t.Fatalf("failed to create closed scheduler: %v", err)
	}

	runContext, cancelRun := context.WithCancel(context.Background())
	runDone := make(chan error, 1)
	go func() {
		_, _, runErr := scheduler.Run(runContext)
		runDone <- runErr
	}()

	select {
	case <-allSessionsPersisted:
		cancelRun()
	case validationErr := <-validationErrors:
		cancelRun()
		t.Fatal(validationErr)
	case <-time.After(schedulerContractTimeout):
		cancelRun()
		t.Fatal("timed out waiting for all VUs to persist and return isolated session cookies")
	}

	if runErr := <-runDone; runErr != nil {
		t.Fatalf("scheduler run failed: %v", runErr)
	}
	stateMu.Lock()
	defer stateMu.Unlock()
	if len(persistedSessions) != users {
		t.Fatalf("persisted session count = %d, want %d", len(persistedSessions), users)
	}
}

func TestClosedScheduler_MultiStepScenario(t *testing.T) {
	var loginHits, itemsHits int64

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/auth/login":
			atomic.AddInt64(&loginHits, 1)
			http.SetCookie(w, &http.Cookie{Name: "session_id", Value: "sess_123", Path: "/"})
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"token":"tok_valid_999"}`))
		case "/api/items":
			authHdr := r.Header.Get("Authorization")
			cookie, _ := r.Cookie("session_id")
			if authHdr != "Bearer tok_valid_999" || cookie == nil || cookie.Value != "sess_123" {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			atomic.AddInt64(&itemsHits, 1)
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"items":["apple","banana"]}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	serverURL, _ := url.Parse(server.URL)
	jarManager, err := auth.NewVUJarManager(true, 3)
	if err != nil {
		t.Fatalf("failed to create jar manager: %v", err)
	}

	sc := &plan.CompiledScenario{
		Name: "auth_and_items",
		Steps: []*plan.CompiledStep{
			{
				Name:             "login",
				URL:              server.URL + "/auth/login",
				Method:           "POST",
				ExpectedStatuses: []int{200},
				Timeout:          2 * time.Second,
				ExtractRules: map[string]plan.ExtractionRule{
					"token": {
						From:       plan.ExtractSourceJSON,
						Expression: "token",
					},
				},
				OnFailure: plan.OnFailureStop,
			},
			{
				Name:   "get_items",
				URL:    server.URL + "/api/items",
				Method: "GET",
				Headers: http.Header{
					"Authorization": {"Bearer ${token}"},
				},
				ExpectedStatuses: []int{200},
				Timeout:          2 * time.Second,
				OnFailure:        plan.OnFailureStop,
			},
		},
	}

	p := &plan.Plan{
		Name:              "scenario-scheduler-test",
		SchemaVersion:     1,
		Fingerprint:       "fingerprint",
		TargetURL:         serverURL,
		Method:            "SCENARIO",
		Headers:           make(http.Header),
		ExpectedStatuses:  []int{200},
		RequestTimeout:    2 * time.Second,
		ResponseBodyLimit: 1024 * 1024,
		RedirectPolicy:    "same-origin",
		Scenario:          sc,
		Model:             core.WorkloadModelClosed,
		Duration:          200 * time.Millisecond,
		GracefulStop:      1 * time.Second,
		Users:             3,
		ThinkTime:         10 * time.Millisecond,
		AllowedHosts:      []string{serverURL.Hostname()},
		JarManager:        jarManager,
		CookieJarEnabled:  true,
	}

	exec, err := executor.NewHTTPExecutor(p)
	if err != nil {
		t.Fatalf("failed to create http executor: %v", err)
	}
	defer exec.Close()

	scheduler, err := NewClosedScheduler(p, exec, clock.NewRealClock())
	if err != nil {
		t.Fatalf("failed to create closed scheduler: %v", err)
	}

	agg, _, err := scheduler.Run(context.Background())
	if err != nil {
		t.Fatalf("scheduler run failed: %v", err)
	}

	if agg.ScenarioIterations.Completed == 0 {
		t.Errorf("expected completed scenario iterations > 0, got %d", agg.ScenarioIterations.Completed)
	}
	if agg.ScenarioIterations.Failed != 0 {
		t.Errorf("expected failed scenario iterations == 0, got %d", agg.ScenarioIterations.Failed)
	}

	if len(agg.Steps) != 2 {
		t.Fatalf("expected 2 steps in aggregate, got %d", len(agg.Steps))
	}

	loginStep := agg.Steps["login"]
	itemsStep := agg.Steps["get_items"]
	if loginStep == nil || itemsStep == nil {
		t.Fatalf("missing login or items step aggregate")
	}

	if loginStep.RequestCounts.Completed != itemsStep.RequestCounts.Completed {
		t.Errorf("login completed (%d) != items completed (%d)", loginStep.RequestCounts.Completed, itemsStep.RequestCounts.Completed)
	}

	if agg.RequestCounts.Completed != loginStep.RequestCounts.Completed+itemsStep.RequestCounts.Completed {
		t.Errorf("root completed (%d) != sum of steps (%d)",
			agg.RequestCounts.Completed, loginStep.RequestCounts.Completed+itemsStep.RequestCounts.Completed)
	}
}
