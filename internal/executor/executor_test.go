package executor

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"testing"
	"time"

	"github.com/charleszardd/daegsa/internal/config"
	"github.com/charleszardd/daegsa/internal/core"
	"github.com/charleszardd/daegsa/internal/plan"
	"github.com/charleszardd/daegsa/internal/safety"
	"github.com/charleszardd/daegsa/internal/testtarget"
)

func helperBuildPlan(t *testing.T, rawURL, method, redirectPolicy string, expectedStatuses []int, bodyLimit int64, timeout time.Duration) *plan.Plan {
	t.Helper()
	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		t.Fatalf("invalid URL %q: %v", rawURL, err)
	}

	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	if bodyLimit <= 0 {
		bodyLimit = 1048576 // 1 MiB
	}
	if redirectPolicy == "" {
		redirectPolicy = "same-origin"
	}
	if len(expectedStatuses) == 0 {
		expectedStatuses = []int{200}
	}

	return &plan.Plan{
		Name:              "test-plan",
		SchemaVersion:     1,
		Fingerprint:       "abcdef1234567890",
		TargetURL:         parsedURL,
		Method:            method,
		Headers:           make(map[string][]string),
		ExpectedStatuses:  expectedStatuses,
		RequestTimeout:    timeout,
		ResponseBodyLimit: bodyLimit,
		RedirectPolicy:    redirectPolicy,
		Model:             core.WorkloadModelOpen,
		Rate:              10,
		TimeUnit:          time.Second,
		MaxInFlight:       20,
		Duration:          5 * time.Second,
		GracefulStop:      2 * time.Second,
		AllowedHosts:      []string{"127.0.0.1", "localhost"},
	}
}

// Mode 1: Status Codes
func TestExecutor_Mode1_StatusCodes(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	ctx := context.Background()

	// 1. 200 OK
	p200 := helperBuildPlan(t, server.URL()+"?status=200", "GET", "", []int{200}, 0, 0)
	exec200, _ := NewHTTPExecutor(p200)
	defer exec200.Close()

	res200, err := exec200.ExecuteRequest(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res200.Outcome != core.OutcomeSuccess {
		t.Errorf("expected OutcomeSuccess, got %s", res200.Outcome)
	}
	if res200.StatusCode != 200 {
		t.Errorf("expected status 200, got %d", res200.StatusCode)
	}

	// 2. 204 No Content
	p204 := helperBuildPlan(t, server.URL()+"?status=204", "GET", "", []int{204}, 0, 0)
	exec204, _ := NewHTTPExecutor(p204)
	defer exec204.Close()

	res204, err := exec204.ExecuteRequest(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res204.Outcome != core.OutcomeSuccess {
		t.Errorf("expected OutcomeSuccess, got %s", res204.Outcome)
	}
	if res204.StatusCode != 204 {
		t.Errorf("expected status 204, got %d", res204.StatusCode)
	}

	// 3. 404 Not Found (unexpected)
	p404Unexp := helperBuildPlan(t, server.URL()+"?status=404", "GET", "", []int{200}, 0, 0)
	exec404Unexp, _ := NewHTTPExecutor(p404Unexp)
	defer exec404Unexp.Close()

	res404Unexp, err := exec404Unexp.ExecuteRequest(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res404Unexp.Outcome != core.OutcomeUnexpectedStatus {
		t.Errorf("expected OutcomeUnexpectedStatus, got %s", res404Unexp.Outcome)
	}

	// 4. 404 Not Found (expected)
	p404Exp := helperBuildPlan(t, server.URL()+"?status=404", "GET", "", []int{404}, 0, 0)
	exec404Exp, _ := NewHTTPExecutor(p404Exp)
	defer exec404Exp.Close()

	res404Exp, err := exec404Exp.ExecuteRequest(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res404Exp.Outcome != core.OutcomeSuccess {
		t.Errorf("expected OutcomeSuccess when 404 is expected, got %s", res404Exp.Outcome)
	}

	// 5. 500 Internal Server Error
	p500 := helperBuildPlan(t, server.URL()+"?status=500", "GET", "", []int{200}, 0, 0)
	exec500, _ := NewHTTPExecutor(p500)
	defer exec500.Close()

	res500, err := exec500.ExecuteRequest(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res500.Outcome != core.OutcomeUnexpectedStatus {
		t.Errorf("expected OutcomeUnexpectedStatus for 500, got %s", res500.Outcome)
	}
}

// Mode 2: Delays & Timestamps
func TestExecutor_Mode2_DelaysAndTimestamps(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	ctx := context.Background()
	delay := 50 * time.Millisecond
	p := helperBuildPlan(t, fmt.Sprintf("%s?delay=%v", server.URL(), delay), "GET", "", []int{200}, 0, 0)
	exec, _ := NewHTTPExecutor(p)
	defer exec.Close()

	res, err := exec.ExecuteRequest(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Outcome != core.OutcomeSuccess {
		t.Errorf("expected OutcomeSuccess, got %s", res.Outcome)
	}

	// Invariant: ScheduledAt <= DispatchedAt <= HeadersReceivedAt <= BodyCompletedAt
	ts := res.Timestamps
	if ts.DispatchedAt.Before(ts.ScheduledAt) {
		t.Errorf("invariant violated: DispatchedAt %v is before ScheduledAt %v", ts.DispatchedAt, ts.ScheduledAt)
	}
	if ts.HeadersReceivedAt.Before(ts.DispatchedAt) {
		t.Errorf("invariant violated: HeadersReceivedAt %v is before DispatchedAt %v", ts.HeadersReceivedAt, ts.DispatchedAt)
	}
	if ts.BodyCompletedAt.Before(ts.HeadersReceivedAt) {
		t.Errorf("invariant violated: BodyCompletedAt %v is before HeadersReceivedAt %v", ts.BodyCompletedAt, ts.HeadersReceivedAt)
	}

	// TTFB and Latency should be at least 40ms (accounting for scheduling jitter)
	if res.TTFB < 40*time.Millisecond {
		t.Errorf("expected TTFB >= 40ms, got %v", res.TTFB)
	}
	if res.Latency < 40*time.Millisecond {
		t.Errorf("expected Latency >= 40ms, got %v", res.Latency)
	}
}

// Mode 3: Payload Streaming & Body Capping
func TestExecutor_Mode3_PayloadStreamingAndCapping(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	ctx := context.Background()

	// Full read: 10 KiB response with 1 MiB limit
	pFull := helperBuildPlan(t, server.URL()+"?bytes=10240", "GET", "", []int{200}, 1048576, 0)
	execFull, _ := NewHTTPExecutor(pFull)
	defer execFull.Close()

	resFull, err := execFull.ExecuteRequest(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resFull.Outcome != core.OutcomeSuccess {
		t.Errorf("expected OutcomeSuccess, got %s", resFull.Outcome)
	}
	if resFull.BytesReceived < 10240 {
		t.Errorf("expected at least 10240 bytes received, got %d", resFull.BytesReceived)
	}
	if resFull.Truncated {
		t.Errorf("expected response not to be truncated")
	}

	// Capped read: 10 KiB response with 500 byte limit
	pCapped := helperBuildPlan(t, server.URL()+"?bytes=10240", "GET", "", []int{200}, 500, 0)
	execCapped, _ := NewHTTPExecutor(pCapped)
	defer execCapped.Close()

	resCapped, err := execCapped.ExecuteRequest(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resCapped.Outcome != core.OutcomeSuccess {
		t.Errorf("expected OutcomeSuccess, got %s", resCapped.Outcome)
	}
	if !resCapped.Truncated {
		t.Errorf("expected response to be marked truncated")
	}
}

// Mode 4: Redirects
func TestExecutor_Mode4_Redirects(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	ctx := context.Background()

	// 1. Same-origin multi-hop redirect (3 hops)
	pSameOrigin := helperBuildPlan(t, server.URL()+"?redirect_path=/dest&hops=3", "GET", config.RedirectPolicySameOrigin, []int{200}, 0, 0)
	execSameOrigin, _ := NewHTTPExecutor(pSameOrigin)
	defer execSameOrigin.Close()

	resSameOrigin, err := execSameOrigin.ExecuteRequest(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resSameOrigin.Outcome != core.OutcomeSuccess {
		t.Errorf("expected OutcomeSuccess for same-origin redirect, got %s", resSameOrigin.Outcome)
	}

	// 2. Cross-origin redirect blocked by same-origin policy
	pCrossBlocked := helperBuildPlan(t, server.URL()+"?redirect_url=http://example.com/external", "GET", config.RedirectPolicySameOrigin, []int{200}, 0, 0)
	execCrossBlocked, _ := NewHTTPExecutor(pCrossBlocked)
	defer execCrossBlocked.Close()

	resCrossBlocked, err := execCrossBlocked.ExecuteRequest(ctx)
	if err != nil {
		t.Fatalf("unexpected execution error: %v", err)
	}
	if resCrossBlocked.Outcome == core.OutcomeSuccess {
		t.Errorf("expected cross-origin redirect to fail, but succeeded")
	}
	if !errors.Is(resCrossBlocked.Err, safety.ErrCrossOriginRedirectBlocked) {
		t.Errorf("expected ErrCrossOriginRedirectBlocked, got %v", resCrossBlocked.Err)
	}

	// 3. Cross-origin redirect with redirects: all and disallowed host
	pCrossDisallowed := helperBuildPlan(t, server.URL()+"?redirect_url=http://unauthorized.example.com/external", "GET", config.RedirectPolicyAll, []int{200}, 0, 0)
	pCrossDisallowed.AllowedHosts = []string{"127.0.0.1", "localhost"}
	execCrossDisallowed, _ := NewHTTPExecutor(pCrossDisallowed)
	defer execCrossDisallowed.Close()

	resCrossDisallowed, _ := execCrossDisallowed.ExecuteRequest(ctx)
	if !errors.Is(resCrossDisallowed.Err, safety.ErrHostNotAllowed) {
		t.Errorf("expected ErrHostNotAllowed for unauthorized redirect target, got %v", resCrossDisallowed.Err)
	}

	// 4. Redirect policy "none"
	pNone := helperBuildPlan(t, server.URL()+"?redirect_path=/dest&hops=1", "GET", config.RedirectPolicyNone, []int{302}, 0, 0)
	execNone, _ := NewHTTPExecutor(pNone)
	defer execNone.Close()

	resNone, err := execNone.ExecuteRequest(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resNone.StatusCode != 302 {
		t.Errorf("expected status 302 with redirect policy 'none', got %d", resNone.StatusCode)
	}
	if resNone.Outcome != core.OutcomeSuccess {
		t.Errorf("expected OutcomeSuccess (302 expected), got %s", resNone.Outcome)
	}
}

// Mode 5: Abrupt TCP Disconnects
func TestExecutor_Mode5_TCPDisconnects(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	ctx := context.Background()

	// 1. Immediate hijack/close
	pImmediate := helperBuildPlan(t, server.URL()+"?drop=immediate", "GET", "", []int{200}, 0, 0)
	execImmediate, _ := NewHTTPExecutor(pImmediate)
	defer execImmediate.Close()

	resImmediate, _ := execImmediate.ExecuteRequest(ctx)
	if !resImmediate.Outcome.IsTransportFailure() {
		t.Errorf("expected transport failure outcome for immediate drop, got %s", resImmediate.Outcome)
	}

	// 2. Midway hijack/close
	pMidway := helperBuildPlan(t, server.URL()+"?drop=midway&after_bytes=50", "GET", "", []int{200}, 0, 0)
	execMidway, _ := NewHTTPExecutor(pMidway)
	defer execMidway.Close()

	resMidway, _ := execMidway.ExecuteRequest(ctx)
	if resMidway.Outcome != core.OutcomeResponseBodyError && !resMidway.Outcome.IsTransportFailure() {
		t.Errorf("expected OutcomeResponseBodyError or transport failure for midway drop, got %s", resMidway.Outcome)
	}
}

// Mode 6: Timeout Hangs
func TestExecutor_Mode6_TimeoutHangs(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	ctx := context.Background()
	// 50ms request timeout against a hanging server
	pHang := helperBuildPlan(t, server.URL()+"?hang=true", "GET", "", []int{200}, 0, 50*time.Millisecond)
	execHang, _ := NewHTTPExecutor(pHang)
	defer execHang.Close()

	resHang, err := execHang.ExecuteRequest(ctx)
	if err != nil {
		t.Fatalf("unexpected execution error: %v", err)
	}
	if resHang.Outcome != core.OutcomeTimeout {
		t.Errorf("expected OutcomeTimeout for hanging target, got %s", resHang.Outcome)
	}
}

// Mode 7: Cookies
func TestExecutor_Mode7_Cookies(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	ctx := context.Background()
	pCookie := helperBuildPlan(t, server.URL()+"/cookies/set?user_session=abc12345", "GET", "", []int{200}, 0, 0)
	execCookie, _ := NewHTTPExecutor(pCookie)
	defer execCookie.Close()

	resCookie, err := execCookie.ExecuteRequest(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resCookie.Outcome != core.OutcomeSuccess {
		t.Errorf("expected OutcomeSuccess, got %s", resCookie.Outcome)
	}
}

// Mode 8: 429 Rate Limiting & Header Parsing
func TestExecutor_Mode8_RateLimiting(t *testing.T) {
	rl := testtarget.NewRateLimiter(testtarget.RateLimiterConfig{
		RequestsPerWindow: 0,
		Window:            60 * time.Second,
		HeaderStyle:       testtarget.RateLimitHeaderStyleAll,
	})

	server := testtarget.NewServer(testtarget.WithRateLimiter(rl))
	defer server.Close()

	ctx := context.Background()
	pRateLimit := helperBuildPlan(t, server.URL(), "GET", "", []int{200}, 0, 0)
	execRateLimit, _ := NewHTTPExecutor(pRateLimit)
	defer execRateLimit.Close()

	res, err := execRateLimit.ExecuteRequest(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.StatusCode != 429 {
		t.Errorf("expected status 429, got %d", res.StatusCode)
	}
	if res.Outcome != core.OutcomeRateLimited {
		t.Errorf("expected OutcomeRateLimited, got %s", res.Outcome)
	}
	if res.RateLimitInfo == nil {
		t.Fatalf("expected RateLimitInfo extracted, got nil")
	}
	if res.RateLimitInfo.RetryAfterSeconds == nil || *res.RateLimitInfo.RetryAfterSeconds <= 0 {
		t.Errorf("expected positive RetryAfterSeconds, got %v", res.RateLimitInfo.RetryAfterSeconds)
	}
	if res.RateLimitInfo.Limit == nil || *res.RateLimitInfo.Limit != 0 {
		t.Errorf("expected Limit 0, got %v", res.RateLimitInfo.Limit)
	}
	if res.RateLimitInfo.Remaining == nil || *res.RateLimitInfo.Remaining != 0 {
		t.Errorf("expected Remaining 0, got %v", res.RateLimitInfo.Remaining)
	}
	if res.RateLimitInfo.Policy != "0;w=60" {
		t.Errorf("expected Policy '0;w=60', got %q", res.RateLimitInfo.Policy)
	}
}

// Transport Keep-Alive Connection Reuse
func TestExecutor_KeepAliveConnectionReuse(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	ctx := context.Background()
	p := helperBuildPlan(t, server.URL(), "GET", "", []int{200}, 0, 0)
	exec, _ := NewHTTPExecutor(p)
	defer exec.Close()

	// Execute consecutive requests
	for i := 0; i < 5; i++ {
		res, err := exec.ExecuteRequest(ctx)
		if err != nil {
			t.Fatalf("request %d failed: %v", i, err)
		}
		if res.Outcome != core.OutcomeSuccess {
			t.Errorf("request %d failed with outcome %s", i, res.Outcome)
		}
	}

	// Verify all 5 requests were received by the server
	recorded := server.RecordedRequests()
	if len(recorded) != 5 {
		t.Errorf("expected 5 recorded requests, got %d", len(recorded))
	}
}

func TestExecutor_ContextCancellation(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Pre-cancel context

	p := helperBuildPlan(t, server.URL(), "GET", "", []int{200}, 0, 0)
	exec, _ := NewHTTPExecutor(p)
	defer exec.Close()

	res, err := exec.ExecuteRequest(ctx)
	if err != nil {
		t.Fatalf("unexpected execution error: %v", err)
	}
	if res.Outcome != core.OutcomeCanceled {
		t.Errorf("expected OutcomeCanceled, got %s", res.Outcome)
	}
}

func TestExecutor_CustomHeadersAndHostOverride(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	ctx := context.Background()
	p := helperBuildPlan(t, server.URL()+"/items", "GET", "", []int{200}, 0, 0)
	p.Headers.Set("X-Custom-Test-Header", "daegsa-test-value")
	p.Headers.Set("Host", "custom.host.override")

	// Verify BuildHTTPRequest sets req.Host explicitly
	req, bytesSent, reqErr := BuildHTTPRequest(ctx, p)
	if reqErr != nil {
		t.Fatalf("BuildHTTPRequest failed: %v", reqErr)
	}
	if req.Host != "custom.host.override" {
		t.Errorf("expected req.Host override 'custom.host.override', got %q", req.Host)
	}
	if bytesSent <= 0 {
		t.Errorf("expected positive estimated bytes sent, got %d", bytesSent)
	}

	exec, _ := NewHTTPExecutor(p)
	defer exec.Close()

	res, err := exec.ExecuteRequest(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Outcome != core.OutcomeSuccess {
		t.Errorf("expected OutcomeSuccess, got %s", res.Outcome)
	}

	recorded := server.RecordedRequests()
	if len(recorded) != 1 {
		t.Fatalf("expected 1 recorded request, got %d", len(recorded))
	}
	if recorded[0].Header.Get("X-Custom-Test-Header") != "daegsa-test-value" {
		t.Errorf("expected custom header received, got %q", recorded[0].Header.Get("X-Custom-Test-Header"))
	}
}

func TestExecutor_Redirect_LoopExceeded(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	ctx := context.Background()
	// 15 hops exceeds MaxRedirectHops (10)
	p := helperBuildPlan(t, server.URL()+"?redirect_path=/dest&hops=15", "GET", config.RedirectPolicySameOrigin, []int{200}, 0, 0)
	exec, _ := NewHTTPExecutor(p)
	defer exec.Close()

	res, err := exec.ExecuteRequest(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.Outcome == core.OutcomeSuccess {
		t.Errorf("expected redirect loop failure, got success")
	}
}

func TestExtractRateLimitInfo_Variants(t *testing.T) {
	// 1. Nil headers
	if info := ExtractRateLimitInfo(nil); info != nil {
		t.Errorf("expected nil for nil headers, got %v", info)
	}

	// 2. HTTP-Date in Retry-After
	hDate := http.Header{}
	hDate.Set("Retry-After", "Sat, 22 Aug 2026 15:30:00 GMT")
	infoDate := ExtractRateLimitInfo(hDate)
	if infoDate == nil || infoDate.RetryAfterDate == nil {
		t.Fatalf("expected RetryAfterDate extracted, got %v", infoDate)
	}

	// 3. RateLimit-Reset with Unix Epoch (>1000000000)
	hEpoch := http.Header{}
	hEpoch.Set("RateLimit-Reset", "1787326200")
	infoEpoch := ExtractRateLimitInfo(hEpoch)
	if infoEpoch == nil || infoEpoch.ResetDate == nil {
		t.Fatalf("expected ResetDate extracted for epoch, got %v", infoEpoch)
	}

	// 4. RateLimit with decimal / float formatting
	hFloat := http.Header{}
	hFloat.Set("RateLimit-Remaining", "99.0")
	infoFloat := ExtractRateLimitInfo(hFloat)
	if infoFloat == nil || infoFloat.Remaining == nil || *infoFloat.Remaining != 99 {
		t.Fatalf("expected Remaining=99 for float rate limit, got %v", infoFloat)
	}
}
