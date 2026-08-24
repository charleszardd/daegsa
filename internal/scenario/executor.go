package scenario

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"

	"github.com/charleszardd/daegsa/internal/clock"
	"github.com/charleszardd/daegsa/internal/config"
	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/executor"
	"github.com/charleszardd/daegsa/internal/safety"
)

// ScenarioExecutor executes sequential scenario steps for virtual users (§4, §7, §8).
type ScenarioExecutor struct {
	scenario     *CompiledScenario
	transport    *http.Transport
	allowedHosts []string
	clock        clock.Clock
	classifier   core.OutcomeClassifier
	knownSecrets []string
}

// NewScenarioExecutor creates a configured ScenarioExecutor (§7, §8).
func NewScenarioExecutor(
	scenario *CompiledScenario,
	transport *http.Transport,
	allowedHosts []string,
	knownSecrets []string,
	clk clock.Clock,
) *ScenarioExecutor {
	if clk == nil {
		clk = clock.NewRealClock()
	}
	if transport == nil {
		transport = executor.NewSharedTransport(executor.DefaultTransportOptions())
	}

	return &ScenarioExecutor{
		scenario:     scenario,
		transport:    transport,
		allowedHosts: append([]string(nil), allowedHosts...),
		clock:        clk,
		classifier:   core.NewOutcomeClassifier(),
		knownSecrets: append([]string(nil), knownSecrets...),
	}
}

func validateScenarioTarget(targetURL *url.URL, allowedHosts []string) error {
	if targetURL == nil || (targetURL.Scheme != "http" && targetURL.Scheme != "https") || targetURL.Host == "" {
		return fmt.Errorf("scenario step target must be an absolute HTTP/HTTPS URL")
	}
	if !safety.IsHostAllowed(targetURL.Hostname(), allowedHosts) {
		return fmt.Errorf("%w: scenario step target host is not allowlisted", safety.ErrHostNotAllowed)
	}
	return nil
}

func scenarioRedirectChecker(redirectPolicy string, allowedHosts []string) func(*http.Request, []*http.Request) error {
	return func(req *http.Request, via []*http.Request) error {
		if len(via) == 0 {
			return fmt.Errorf("redirect refused without an originating request")
		}
		if len(via) >= safety.MaxRedirectHops {
			return fmt.Errorf("stopped after %d redirects", safety.MaxRedirectHops)
		}

		// Empty or unknown policies fail closed. Valid compiled configurations
		// always provide one of the explicit policies below.
		switch redirectPolicy {
		case core.RedirectPolicyNone:
			return http.ErrUseLastResponse
		case core.RedirectPolicySameOrigin:
			initialURL := via[0].URL
			if req.URL.Scheme != initialURL.Scheme || req.URL.Host != initialURL.Host {
				return fmt.Errorf("%w: blocked cross-origin scenario redirect", safety.ErrCrossOriginRedirectBlocked)
			}
		case core.RedirectPolicyAll:
			// Cross-origin redirects are permitted only after host authorization.
		default:
			return http.ErrUseLastResponse
		}

		if err := validateScenarioTarget(req.URL, allowedHosts); err != nil {
			return fmt.Errorf("scenario redirect refused: %w", err)
		}
		return nil
	}
}

// SetClock allows injecting a mock clock for deterministic testing.
func (e *ScenarioExecutor) SetClock(clk clock.Clock) {
	e.clock = clk
}

// Transport returns the underlying shared transport.
func (e *ScenarioExecutor) Transport() *http.Transport {
	return e.transport
}

// ExecuteStep executes a single compiled scenario step for the given VUState (§7, §8, §11).
func (e *ScenarioExecutor) ExecuteStep(ctx context.Context, state *VUState, step *CompiledStep) (*StepResult, error) {
	scheduledAt := e.clock.Now()
	dispatchedAt := e.clock.Now()

	// 1. Variable substitution across URL, Headers, and Body
	subURL, urlErr := SubstituteVariables(step.URL, state.Variables)
	if urlErr != nil {
		completedAt := e.clock.Now()
		outcome := e.classifier.Classify(core.ClassifyInput{
			RequestBuildErr: true,
			Err:             urlErr,
		})
		return &StepResult{
			StepName: step.Name,
			Result: &executor.Result{
				Outcome:       outcome,
				Timestamps:    core.RequestTimestamps{ScheduledAt: scheduledAt, DispatchedAt: dispatchedAt, HeadersReceivedAt: completedAt, BodyCompletedAt: completedAt},
				Latency:       completedAt.Sub(dispatchedAt),
				TotalDuration: completedAt.Sub(scheduledAt),
				Err:           urlErr,
			},
			ExtractErr: urlErr,
			Succeeded:  false,
		}, nil
	}

	parsedURL, parseErr := url.Parse(subURL)
	if parseErr != nil {
		completedAt := e.clock.Now()
		outcome := e.classifier.Classify(core.ClassifyInput{
			RequestBuildErr: true,
			Err:             parseErr,
		})
		return &StepResult{
			StepName: step.Name,
			Result: &executor.Result{
				Outcome:       outcome,
				Timestamps:    core.RequestTimestamps{ScheduledAt: scheduledAt, DispatchedAt: dispatchedAt, HeadersReceivedAt: completedAt, BodyCompletedAt: completedAt},
				Latency:       completedAt.Sub(dispatchedAt),
				TotalDuration: completedAt.Sub(scheduledAt),
				Err:           parseErr,
			},
			ExtractErr: parseErr,
			Succeeded:  false,
		}, nil
	}
	if targetErr := validateScenarioTarget(parsedURL, e.allowedHosts); targetErr != nil {
		completedAt := e.clock.Now()
		scrubbedErr := config.RedactError(targetErr, e.knownSecrets)
		outcome := e.classifier.Classify(core.ClassifyInput{
			RequestBuildErr: true,
			Err:             scrubbedErr,
		})
		return &StepResult{
			StepName: step.Name,
			Result: &executor.Result{
				Outcome:       outcome,
				Timestamps:    core.RequestTimestamps{ScheduledAt: scheduledAt, DispatchedAt: dispatchedAt, HeadersReceivedAt: completedAt, BodyCompletedAt: completedAt},
				Latency:       completedAt.Sub(dispatchedAt),
				TotalDuration: completedAt.Sub(scheduledAt),
				Err:           scrubbedErr,
			},
			ExtractErr: scrubbedErr,
			Succeeded:  false,
		}, nil
	}

	var bodyBytes []byte
	if step.Body != "" {
		subBody, bodySubErr := SubstituteVariables(step.Body, state.Variables)
		if bodySubErr != nil {
			completedAt := e.clock.Now()
			outcome := e.classifier.Classify(core.ClassifyInput{
				RequestBuildErr: true,
				Err:             bodySubErr,
			})
			return &StepResult{
				StepName: step.Name,
				Result: &executor.Result{
					Outcome:       outcome,
					Timestamps:    core.RequestTimestamps{ScheduledAt: scheduledAt, DispatchedAt: dispatchedAt, HeadersReceivedAt: completedAt, BodyCompletedAt: completedAt},
					Latency:       completedAt.Sub(dispatchedAt),
					TotalDuration: completedAt.Sub(scheduledAt),
					Err:           bodySubErr,
				},
				ExtractErr: bodySubErr,
				Succeeded:  false,
			}, nil
		}
		bodyBytes = []byte(subBody)
	}

	// 2. Build HTTP request
	timeout := step.Timeout
	if timeout <= 0 {
		timeout = config.DefaultRequestTimeout
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var bodyReader io.Reader
	if len(bodyBytes) > 0 {
		bodyReader = bytes.NewReader(bodyBytes)
	}

	req, err := http.NewRequestWithContext(reqCtx, step.Method, parsedURL.String(), bodyReader)
	if err != nil {
		completedAt := e.clock.Now()
		outcome := e.classifier.Classify(core.ClassifyInput{
			RequestBuildErr: true,
			Err:             err,
		})
		return &StepResult{
			StepName: step.Name,
			Result: &executor.Result{
				Outcome:       outcome,
				Timestamps:    core.RequestTimestamps{ScheduledAt: scheduledAt, DispatchedAt: dispatchedAt, HeadersReceivedAt: completedAt, BodyCompletedAt: completedAt},
				Latency:       completedAt.Sub(dispatchedAt),
				TotalDuration: completedAt.Sub(scheduledAt),
				Err:           err,
			},
			ExtractErr: err,
			Succeeded:  false,
		}, nil
	}

	// Apply headers with variable substitution
	for k, vals := range step.Headers {
		for _, val := range vals {
			subHdrVal, hdrErr := SubstituteVariables(val, state.Variables)
			if hdrErr == nil {
				req.Header.Add(k, subHdrVal)
			} else {
				req.Header.Add(k, val)
			}
		}
	}
	if hostVal := req.Header.Get("Host"); hostVal != "" {
		req.Host = hostVal
	}

	// 3. Execute HTTP request with isolated VU CookieJar
	client := &http.Client{
		Transport:     e.transport,
		CheckRedirect: scenarioRedirectChecker(step.RedirectPolicy, e.allowedHosts),
	}
	if state != nil && state.CookieJar != nil {
		client.Jar = state.CookieJar
	}

	bytesSent := int64(len(step.Method) + len(parsedURL.RequestURI()) + len(" HTTP/1.1\r\n") + len(bodyBytes))

	resp, doErr := client.Do(req)
	headersReceivedAt := e.clock.Now()

	if doErr != nil {
		bodyCompletedAt := e.clock.Now()
		isCanceled := errors.Is(ctx.Err(), context.Canceled)
		scrubbedErr := config.RedactError(doErr, e.knownSecrets)
		outcome := e.classifier.Classify(core.ClassifyInput{
			Err:      scrubbedErr,
			Canceled: isCanceled,
		})
		return &StepResult{
			StepName: step.Name,
			Result: &executor.Result{
				Outcome:       outcome,
				StatusCode:    0,
				Timestamps:    core.RequestTimestamps{ScheduledAt: scheduledAt, DispatchedAt: dispatchedAt, HeadersReceivedAt: headersReceivedAt, BodyCompletedAt: bodyCompletedAt},
				Latency:       bodyCompletedAt.Sub(dispatchedAt),
				TTFB:          headersReceivedAt.Sub(dispatchedAt),
				TotalDuration: bodyCompletedAt.Sub(scheduledAt),
				BytesSent:     bytesSent,
				BytesReceived: 0,
				Err:           scrubbedErr,
			},
			ExtractErr: scrubbedErr,
			Succeeded:  false,
		}, nil
	}

	// 4. Read and safely drain response body
	respBody, bytesRecv, truncated, bodyErr := executor.ReadAndDrainResponseBody(resp, step.ResponseBodyLimit)
	bodyCompletedAt := e.clock.Now()

	rateLimitInfo := executor.ExtractRateLimitInfo(resp.Header)
	scrubbedBodyErr := config.RedactError(bodyErr, e.knownSecrets)
	outcome := e.classifier.Classify(core.ClassifyInput{
		StatusCode:       resp.StatusCode,
		ExpectedStatuses: step.ExpectedStatuses,
		ResponseBodyErr:  scrubbedBodyErr != nil,
		Err:              scrubbedBodyErr,
	})

	ttfb := headersReceivedAt.Sub(dispatchedAt)
	if ttfb < 0 {
		ttfb = 0
	}
	latency := bodyCompletedAt.Sub(dispatchedAt)
	if latency < 0 {
		latency = 0
	}
	totalDuration := bodyCompletedAt.Sub(scheduledAt)
	if totalDuration < 0 {
		totalDuration = 0
	}

	res := &executor.Result{
		Outcome:       outcome,
		StatusCode:    resp.StatusCode,
		Protocol:      resp.Proto,
		Timestamps:    core.RequestTimestamps{ScheduledAt: scheduledAt, DispatchedAt: dispatchedAt, HeadersReceivedAt: headersReceivedAt, BodyCompletedAt: bodyCompletedAt},
		Latency:       latency,
		TTFB:          ttfb,
		TotalDuration: totalDuration,
		BytesSent:     bytesSent,
		BytesReceived: bytesRecv,
		Truncated:     truncated,
		RateLimitInfo: rateLimitInfo,
		Err:           scrubbedBodyErr,
	}

	// Check if step succeeded HTTP-wise
	succeeded := outcome.IsSuccess()

	// 5. Response Extraction
	var extractErr error
	if succeeded && len(step.ExtractRules) > 0 {
		extractErr = ExtractAll(resp, respBody, step.ExtractRules, state)
		if extractErr != nil {
			succeeded = false
			res.Outcome = core.OutcomeUnexpectedStatus
			res.Err = extractErr
		}
	}

	return &StepResult{
		StepName:   step.Name,
		Result:     res,
		ExtractErr: extractErr,
		Succeeded:  succeeded,
	}, nil
}

// ExecuteIteration executes all scenario steps sequentially for the given VU (§7).
// Returns whether the entire iteration succeeded and any fatal scheduler error (e.g. ErrAbortVU).
func (e *ScenarioExecutor) ExecuteIteration(
	ctx context.Context,
	state *VUState,
	onStepDone func(stepResult *StepResult),
) (bool, error) {
	if e.scenario == nil || len(e.scenario.Steps) == 0 {
		return true, nil
	}

	state.ResetIteration()
	iterationSuccess := true

	for stepIdx, step := range e.scenario.Steps {
		// Check context cancellation before executing step
		if ctx.Err() != nil {
			return false, ctx.Err()
		}

		stepRes, err := e.ExecuteStep(ctx, state, step)
		if err != nil {
			return false, err
		}
		stepRes.StepIndex = stepIdx

		if onStepDone != nil {
			onStepDone(stepRes)
		}

		if !stepRes.Succeeded {
			iterationSuccess = false
			switch step.OnFailure {
			case OnFailureStop:
				// Stop current iteration and return
				return false, nil

			case OnFailureAbortVU:
				stepRes.AbortedVU = true
				return false, ErrAbortVU

			case OnFailureContinue:
				// Continue to next step in iteration
			}
		}

		// Apply per-step think time between steps if step succeeded
		if stepRes.Succeeded && step.ThinkTime > 0 && stepIdx < len(e.scenario.Steps)-1 {
			thinkTimer := e.clock.NewTimer(step.ThinkTime)
			select {
			case <-thinkTimer.C():
			case <-ctx.Done():
				thinkTimer.Stop()
				return false, ctx.Err()
			}
		}
	}

	return iterationSuccess, nil
}
