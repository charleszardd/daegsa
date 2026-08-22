package executor

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/charleszardd/daegsa/internal/clock"
	"github.com/charleszardd/daegsa/internal/config"
	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/plan"
	"github.com/charleszardd/daegsa/internal/safety"
)

// HTTPExecutor executes requests against a target according to an immutable plan (§8, §9).
type HTTPExecutor struct {
	client        *http.Client
	transport     *http.Transport
	classifier    core.OutcomeClassifier
	plan          *plan.Plan
	clock         clock.Clock
	checkRedirect func(req *http.Request, via []*http.Request) error
	workerClients []*http.Client
}

// NewHTTPExecutor instantiates a new HTTPExecutor with a shared tuned transport (§8).
func NewHTTPExecutor(p *plan.Plan) (*HTTPExecutor, error) {
	if p == nil {
		return nil, fmt.Errorf("plan cannot be nil")
	}

	transport := NewSharedTransport(DefaultTransportOptions())

	checkRedirect := func(req *http.Request, via []*http.Request) error {
		if len(via) >= safety.MaxRedirectHops {
			return fmt.Errorf("stopped after %d redirects", safety.MaxRedirectHops)
		}

		switch p.RedirectPolicy {
		case core.RedirectPolicyNone:
			return http.ErrUseLastResponse

		case core.RedirectPolicySameOrigin:
			initial := via[0].URL
			if req.URL.Scheme != initial.Scheme || req.URL.Host != initial.Host {
				return fmt.Errorf("%w: blocked redirect from %s to %s",
					safety.ErrCrossOriginRedirectBlocked, initial.Host, req.URL.Host)
			}

		case core.RedirectPolicyAll:
			targetHost := req.URL.Hostname()
			if targetHost == "" {
				targetHost = req.URL.Host
			}
			if !safety.IsHostAllowed(targetHost, p.AllowedHosts) {
				return fmt.Errorf("%w: redirect target host %q is not in allowed_hosts %v",
					safety.ErrHostNotAllowed, targetHost, p.AllowedHosts)
			}
		}

		return nil
	}

	client := &http.Client{
		Transport:     transport,
		CheckRedirect: checkRedirect,
	}

	var workerClients []*http.Client
	if p.CookieJarEnabled && p.JarManager != nil {
		workerCount := int(p.Users)
		if p.Model == core.WorkloadModelOpen {
			workerCount = int(p.MaxInFlight)
		}
		workerClients = make([]*http.Client, workerCount)
		for workerID := range workerClients {
			workerClients[workerID] = &http.Client{
				Transport:     transport,
				CheckRedirect: checkRedirect,
				Jar:           p.JarManager.GetJar(workerID),
			}
		}
	}

	return &HTTPExecutor{
		client:        client,
		transport:     transport,
		classifier:    core.NewOutcomeClassifier(),
		plan:          p,
		clock:         clock.NewRealClock(),
		checkRedirect: checkRedirect,
		workerClients: workerClients,
	}, nil
}

// SetClock allows injecting a custom clock for deterministic testing.
func (e *HTTPExecutor) SetClock(c clock.Clock) {
	e.clock = c
}

// SetTransport allows injecting a custom transport for testing.
func (e *HTTPExecutor) SetTransport(t *http.Transport) {
	e.transport = t
	e.client.Transport = t
	for _, client := range e.workerClients {
		client.Transport = t
	}
}

// ExecuteRequest executes a single HTTP request according to the plan for the specified workerID/VU,
// capturing precise boundary timestamps, latencies, and classifying the terminal outcome (§8, §9, §11).
func (e *HTTPExecutor) ExecuteRequest(ctx context.Context, workerID int) (*Result, error) {
	scheduledAt := e.clock.Now()
	dispatchedAt := e.clock.Now()

	// Apply per-request timeout
	timeout := e.plan.RequestTimeout
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	reqCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	req, bytesSent, reqErr := BuildHTTPRequest(reqCtx, e.plan)
	if reqErr != nil {
		completedAt := e.clock.Now()
		scrubbedErr := config.RedactError(reqErr, e.plan.KnownSecrets)
		outcome := e.classifier.Classify(core.ClassifyInput{
			RequestBuildErr: true,
			Err:             scrubbedErr,
		})
		return &Result{
			Outcome:       outcome,
			Timestamps:    core.RequestTimestamps{ScheduledAt: scheduledAt, DispatchedAt: dispatchedAt, HeadersReceivedAt: completedAt, BodyCompletedAt: completedAt},
			Latency:       completedAt.Sub(dispatchedAt),
			TTFB:          0,
			TotalDuration: completedAt.Sub(scheduledAt),
			BytesSent:     0,
			BytesReceived: 0,
			Err:           scrubbedErr,
		}, nil
	}

	// Inject authentication credentials based on workerID
	if e.plan.Authenticator != nil {
		e.plan.Authenticator.AuthenticateRequest(req, workerID)
		bytesSent = estimateRequestBytes(req, int64(len(e.plan.Body)))
	}

	// Select the bounded per-worker client when cookie isolation is enabled.
	client := e.client
	if workerID >= 0 && workerID < len(e.workerClients) {
		client = e.workerClients[workerID]
	}

	resp, doErr := client.Do(req)
	headersReceivedAt := e.clock.Now()

	if doErr != nil {
		bodyCompletedAt := e.clock.Now()
		isCanceled := errors.Is(ctx.Err(), context.Canceled)
		scrubbedErr := config.RedactError(doErr, e.plan.KnownSecrets)
		outcome := e.classifier.Classify(core.ClassifyInput{
			Err:      scrubbedErr,
			Canceled: isCanceled,
		})
		return &Result{
			Outcome:       outcome,
			StatusCode:    0,
			Timestamps:    core.RequestTimestamps{ScheduledAt: scheduledAt, DispatchedAt: dispatchedAt, HeadersReceivedAt: headersReceivedAt, BodyCompletedAt: bodyCompletedAt},
			Latency:       bodyCompletedAt.Sub(dispatchedAt),
			TTFB:          headersReceivedAt.Sub(dispatchedAt),
			TotalDuration: bodyCompletedAt.Sub(scheduledAt),
			BytesSent:     bytesSent,
			BytesReceived: 0,
			Err:           scrubbedErr,
		}, nil
	}

	// Read and drain response body
	_, bytesRecv, truncated, bodyErr := ReadAndDrainResponseBody(resp, e.plan.ResponseBodyLimit)
	bodyCompletedAt := e.clock.Now()

	rateLimitInfo := ExtractRateLimitInfo(resp.Header)
	scrubbedBodyErr := config.RedactError(bodyErr, e.plan.KnownSecrets)
	outcome := e.classifier.Classify(core.ClassifyInput{
		StatusCode:       resp.StatusCode,
		ExpectedStatuses: e.plan.ExpectedStatuses,
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

	return &Result{
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
	}, nil
}

// Transport returns the underlying shared HTTP transport (§8).
func (e *HTTPExecutor) Transport() *http.Transport {
	return e.transport
}

// Close releases pooled idle TCP connections and cleans up executor resources (§8).
func (e *HTTPExecutor) Close() {
	if e.transport != nil {
		e.transport.CloseIdleConnections()
	}
}
