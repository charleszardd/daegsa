package scenario_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/charleszardd/daegsa/internal/clock"
	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/safety"
	"github.com/charleszardd/daegsa/internal/scenario"
)

func TestScenarioExecutorRedirectPolicyDefaultsFailClosed(t *testing.T) {
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

	for _, redirectPolicy := range []string{"", "unsupported"} {
		step := &scenario.CompiledStep{
			Name:             "fail-closed",
			URL:              initialTarget.URL,
			Method:           http.MethodGet,
			ExpectedStatuses: []int{http.StatusFound},
			Timeout:          time.Second,
			RedirectPolicy:   redirectPolicy,
		}
		executor := scenario.NewScenarioExecutor(nil, nil, []string{"127.0.0.1"}, nil, clock.NewRealClock())
		result, err := executor.ExecuteStep(context.Background(), scenario.NewVUState(0, nil, nil), step)
		if err != nil {
			t.Fatalf("ExecuteStep(%q) error = %v", redirectPolicy, err)
		}
		if !result.Succeeded || result.Result.StatusCode != http.StatusFound {
			t.Fatalf("ExecuteStep(%q) result = %+v, want unfollowed 302 success", redirectPolicy, result)
		}
	}

	if got := redirectedRequestCount.Load(); got != 0 {
		t.Fatalf("fail-closed policies followed %d redirect(s), want 0", got)
	}
}

func TestScenarioExecutorConcurrentStepRedirectPoliciesRemainIsolated(t *testing.T) {
	const requestsPerPolicy = 12

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

	executor := scenario.NewScenarioExecutor(nil, nil, []string{"127.0.0.1"}, nil, clock.NewRealClock())
	steps := []*scenario.CompiledStep{
		{
			Name:             "do-not-follow",
			URL:              initialTarget.URL,
			Method:           http.MethodGet,
			ExpectedStatuses: []int{http.StatusFound},
			Timeout:          time.Second,
			RedirectPolicy:   core.RedirectPolicyNone,
		},
		{
			Name:             "follow-allowlisted",
			URL:              initialTarget.URL,
			Method:           http.MethodGet,
			ExpectedStatuses: []int{http.StatusOK},
			Timeout:          time.Second,
			RedirectPolicy:   core.RedirectPolicyAll,
		},
	}

	var waitGroup sync.WaitGroup
	errorChannel := make(chan error, len(steps)*requestsPerPolicy)
	for _, step := range steps {
		for requestIndex := 0; requestIndex < requestsPerPolicy; requestIndex++ {
			waitGroup.Add(1)
			go func(compiledStep *scenario.CompiledStep, vuID int) {
				defer waitGroup.Done()
				result, err := executor.ExecuteStep(context.Background(), scenario.NewVUState(vuID, nil, nil), compiledStep)
				if err != nil {
					errorChannel <- err
					return
				}
				if result == nil || !result.Succeeded {
					errorChannel <- errors.New("scenario step did not satisfy its own redirect policy")
				}
			}(step, requestIndex)
		}
	}
	waitGroup.Wait()
	close(errorChannel)
	for err := range errorChannel {
		t.Error(err)
	}

	if got := redirectedRequestCount.Load(); got != requestsPerPolicy {
		t.Fatalf("redirected request count = %d, want %d", got, requestsPerPolicy)
	}
}

func TestScenarioExecutorSameOriginPolicyBlocksCrossOriginBeforeTraffic(t *testing.T) {
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

	step := &scenario.CompiledStep{
		Name:             "same-origin",
		URL:              initialTarget.URL,
		Method:           http.MethodGet,
		ExpectedStatuses: []int{http.StatusOK},
		Timeout:          time.Second,
		RedirectPolicy:   core.RedirectPolicySameOrigin,
	}
	executor := scenario.NewScenarioExecutor(nil, nil, []string{"127.0.0.1"}, nil, clock.NewRealClock())
	result, err := executor.ExecuteStep(context.Background(), scenario.NewVUState(0, nil, nil), step)
	if err != nil {
		t.Fatalf("ExecuteStep() error = %v", err)
	}
	if !errors.Is(result.Result.Err, safety.ErrCrossOriginRedirectBlocked) {
		t.Fatalf("result error = %v, want ErrCrossOriginRedirectBlocked", result.Result.Err)
	}
	if got := redirectedRequestCount.Load(); got != 0 {
		t.Fatalf("cross-origin target received %d request(s), want 0", got)
	}
}
