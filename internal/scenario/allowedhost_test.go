package scenario_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charleszardd/daegsa/internal/clock"
	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/safety"
	"github.com/charleszardd/daegsa/internal/scenario"
)

func TestScenarioExecutorRejectsSubstitutedDisallowedHostBeforeRequest(t *testing.T) {
	var requestCount atomic.Int64
	target := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		requestCount.Add(1)
		responseWriter.WriteHeader(http.StatusOK)
	}))
	defer target.Close()

	targetURL, err := url.Parse(target.URL)
	if err != nil {
		t.Fatalf("parse target URL: %v", err)
	}
	step := &scenario.CompiledStep{
		Name:             "dynamic-host",
		URL:              "http://${runtime_host}/items",
		Method:           http.MethodGet,
		ExpectedStatuses: []int{http.StatusOK},
		Timeout:          time.Second,
		RedirectPolicy:   core.RedirectPolicyNone,
	}
	executor := scenario.NewScenarioExecutor(
		&scenario.CompiledScenario{Name: "runtime-host", Steps: []*scenario.CompiledStep{step}},
		nil,
		[]string{"localhost"},
		nil,
		clock.NewRealClock(),
	)
	state := scenario.NewVUState(0, nil, map[string]string{"runtime_host": targetURL.Host})

	result, executeErr := executor.ExecuteStep(context.Background(), state, step)
	if executeErr != nil {
		t.Fatalf("ExecuteStep() error = %v", executeErr)
	}
	if result == nil || result.Result == nil {
		t.Fatal("ExecuteStep() returned no result")
	}
	if !errors.Is(result.Result.Err, safety.ErrHostNotAllowed) {
		t.Fatalf("result error = %v, want ErrHostNotAllowed", result.Result.Err)
	}
	if got := requestCount.Load(); got != 0 {
		t.Fatalf("unauthorized target received %d request(s), want 0", got)
	}
}

func TestScenarioExecutorUsesStepRedirectPolicyAndAllowlist(t *testing.T) {
	var redirectedRequestCount atomic.Int64
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, _ *http.Request) {
		redirectedRequestCount.Add(1)
		responseWriter.WriteHeader(http.StatusOK)
	}))
	defer redirectTarget.Close()

	initialTarget := httptest.NewServer(http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		http.Redirect(responseWriter, request, redirectTarget.URL, http.StatusFound)
	}))
	defer initialTarget.Close()
	initialURL := strings.Replace(initialTarget.URL, "127.0.0.1", "localhost", 1)

	step := &scenario.CompiledStep{
		Name:             "redirect",
		URL:              initialURL,
		Method:           http.MethodGet,
		ExpectedStatuses: []int{http.StatusOK},
		Timeout:          time.Second,
		RedirectPolicy:   core.RedirectPolicyAll,
	}
	// The compiled step redirect policy is authoritative.
	executor := scenario.NewScenarioExecutor(
		&scenario.CompiledScenario{Name: "redirect-policy", Steps: []*scenario.CompiledStep{step}},
		nil,
		[]string{"localhost"},
		nil,
		clock.NewRealClock(),
	)

	result, executeErr := executor.ExecuteStep(context.Background(), scenario.NewVUState(0, nil, nil), step)
	if executeErr != nil {
		t.Fatalf("ExecuteStep() error = %v", executeErr)
	}
	if result == nil || result.Result == nil {
		t.Fatal("ExecuteStep() returned no result")
	}
	if !errors.Is(result.Result.Err, safety.ErrHostNotAllowed) {
		t.Fatalf("result error = %v, want ErrHostNotAllowed", result.Result.Err)
	}
	if got := redirectedRequestCount.Load(); got != 0 {
		t.Fatalf("unauthorized redirect target received %d request(s), want 0", got)
	}
}
