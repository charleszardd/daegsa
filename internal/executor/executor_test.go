package executor

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/charleszardd/daegsa/internal/auth"
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

	res200, err := exec200.ExecuteRequest(ctx, 0)
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

	res204, err := exec204.ExecuteRequest(ctx, 0)
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

	res404Unexp, err := exec404Unexp.ExecuteRequest(ctx, 0)
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

	res404Exp, err := exec404Exp.ExecuteRequest(ctx, 0)
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

	res500, err := exec500.ExecuteRequest(ctx, 0)
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

	res, err := exec.ExecuteRequest(ctx, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if res.Outcome != core.OutcomeSuccess {
		t.Errorf("expected OutcomeSuccess, got %s", res.Outcome)
	}

	// Invariant: ScheduledAt <= DispatchedAt <= HeadersReceivedAt <= BodyCompletedAt
	ts := res.Timestamps
	if ts.DispatchedAt.Before(ts.ScheduledAt) {
		t.Errorf("DispatchedAt before ScheduledAt: %v < %v", ts.DispatchedAt, ts.ScheduledAt)
	}
	if ts.HeadersReceivedAt.Before(ts.DispatchedAt) {
		t.Errorf("HeadersReceivedAt before DispatchedAt: %v < %v", ts.HeadersReceivedAt, ts.DispatchedAt)
	}
	if ts.BodyCompletedAt.Before(ts.HeadersReceivedAt) {
		t.Errorf("BodyCompletedAt before HeadersReceivedAt: %v < %v", ts.BodyCompletedAt, ts.HeadersReceivedAt)
	}

	// Latency measurements non-negative
	if res.Latency <= 0 {
		t.Errorf("expected positive latency, got %v", res.Latency)
	}
	if res.TTFB <= 0 {
		t.Errorf("expected positive TTFB, got %v", res.TTFB)
	}
	if res.TotalDuration <= 0 {
		t.Errorf("expected positive TotalDuration, got %v", res.TotalDuration)
	}
	if res.Latency < 40*time.Millisecond {
		t.Errorf("expected at least 40ms latency due to delay, got %v", res.Latency)
	}
}

// Mode 3: Payload Streaming & Capping
func TestExecutor_Mode3_PayloadStreamingAndCapping(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	ctx := context.Background()

	// Full read: 10 KiB response with 1 MiB limit
	pFull := helperBuildPlan(t, server.URL()+"?bytes=10240", "GET", "", []int{200}, 1048576, 0)
	execFull, _ := NewHTTPExecutor(pFull)
	defer execFull.Close()

	resFull, err := execFull.ExecuteRequest(ctx, 0)
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

	resCapped, err := execCapped.ExecuteRequest(ctx, 0)
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
	pSameOrigin := helperBuildPlan(t, server.URL()+"?redirect_path=/dest&hops=3", "GET", core.RedirectPolicySameOrigin, []int{200}, 0, 0)
	execSameOrigin, _ := NewHTTPExecutor(pSameOrigin)
	defer execSameOrigin.Close()

	resSameOrigin, err := execSameOrigin.ExecuteRequest(ctx, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resSameOrigin.Outcome != core.OutcomeSuccess {
		t.Errorf("expected OutcomeSuccess for same-origin redirect, got %s", resSameOrigin.Outcome)
	}

	// 2. Cross-origin redirect blocked by same-origin policy
	pCrossBlocked := helperBuildPlan(t, server.URL()+"?redirect_url=http://example.com/external", "GET", core.RedirectPolicySameOrigin, []int{200}, 0, 0)
	execCrossBlocked, _ := NewHTTPExecutor(pCrossBlocked)
	defer execCrossBlocked.Close()

	resCrossBlocked, err := execCrossBlocked.ExecuteRequest(ctx, 0)
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
	pCrossDisallowed := helperBuildPlan(t, server.URL()+"?redirect_url=http://unauthorized.example.com/external", "GET", core.RedirectPolicyAll, []int{200}, 0, 0)
	pCrossDisallowed.AllowedHosts = []string{"127.0.0.1", "localhost"}
	execCrossDisallowed, _ := NewHTTPExecutor(pCrossDisallowed)
	defer execCrossDisallowed.Close()

	resCrossDisallowed, _ := execCrossDisallowed.ExecuteRequest(ctx, 0)
	if !errors.Is(resCrossDisallowed.Err, safety.ErrHostNotAllowed) {
		t.Errorf("expected ErrHostNotAllowed for unauthorized redirect target, got %v", resCrossDisallowed.Err)
	}

	// 4. Redirect policy "none"
	pNone := helperBuildPlan(t, server.URL()+"?redirect_path=/dest&hops=1", "GET", core.RedirectPolicyNone, []int{302}, 0, 0)
	execNone, _ := NewHTTPExecutor(pNone)
	defer execNone.Close()

	resNone, err := execNone.ExecuteRequest(ctx, 0)
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

	resImmediate, _ := execImmediate.ExecuteRequest(ctx, 0)
	if !resImmediate.Outcome.IsTransportFailure() {
		t.Errorf("expected transport failure outcome for immediate drop, got %s", resImmediate.Outcome)
	}

	// 2. Midway hijack/close
	pMidway := helperBuildPlan(t, server.URL()+"?drop=midway&after_bytes=50", "GET", "", []int{200}, 0, 0)
	execMidway, _ := NewHTTPExecutor(pMidway)
	defer execMidway.Close()

	resMidway, _ := execMidway.ExecuteRequest(ctx, 0)
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

	resHang, err := execHang.ExecuteRequest(ctx, 0)
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

	resCookie, err := execCookie.ExecuteRequest(ctx, 0)
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

	res, err := execRateLimit.ExecuteRequest(ctx, 0)
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
		res, err := exec.ExecuteRequest(ctx, 0)
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

	res, err := exec.ExecuteRequest(ctx, 0)
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

	res, err := exec.ExecuteRequest(ctx, 0)
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
	p := helperBuildPlan(t, server.URL()+"?redirect_path=/dest&hops=15", "GET", core.RedirectPolicySameOrigin, []int{200}, 0, 0)
	exec, _ := NewHTTPExecutor(p)
	defer exec.Close()

	res, err := exec.ExecuteRequest(ctx, 0)
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

// Authentication & Secret Contract Tests against testtarget
func TestExecutor_Auth_Bearer(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	ctx := context.Background()

	// Valid Bearer
	pValid := helperBuildPlan(t, server.URL()+"/auth/bearer", "GET", "", []int{200}, 0, 0)
	authnValid, _ := auth.NewAuthenticator(&config.AuthConfig{
		Type:  auth.AuthTypeBearer,
		Token: "valid-bearer-token",
	})
	pValid.Authenticator = authnValid
	pValid.KnownSecrets = []string{"valid-bearer-token"}

	execValid, _ := NewHTTPExecutor(pValid)
	defer execValid.Close()

	resValid, err := execValid.ExecuteRequest(ctx, 0)
	if err != nil {
		t.Fatalf("unexpected execution error: %v", err)
	}
	if resValid.Outcome != core.OutcomeSuccess {
		t.Errorf("expected OutcomeSuccess for valid bearer, got %s (status %d)", resValid.Outcome, resValid.StatusCode)
	}

	// Invalid Bearer (none configured)
	pInvalid := helperBuildPlan(t, server.URL()+"/auth/bearer", "GET", "", []int{200}, 0, 0)
	execInvalid, _ := NewHTTPExecutor(pInvalid)
	defer execInvalid.Close()

	resInvalid, err := execInvalid.ExecuteRequest(ctx, 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resInvalid.Outcome != core.OutcomeUnexpectedStatus || resInvalid.StatusCode != 401 {
		t.Errorf("expected 401 Unauthorized, got outcome %s, status %d", resInvalid.Outcome, resInvalid.StatusCode)
	}
}

func TestExecutor_Auth_CustomHeader(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	ctx := context.Background()
	p := helperBuildPlan(t, server.URL()+"/auth/header", "GET", "", []int{200}, 0, 0)
	authn, _ := auth.NewAuthenticator(&config.AuthConfig{
		Type:       auth.AuthTypeCustomHeader,
		Token:      "live-api-key-999",
		HeaderName: "X-API-Key",
	})
	p.Authenticator = authn
	p.KnownSecrets = []string{"live-api-key-999"}

	exec, _ := NewHTTPExecutor(p)
	defer exec.Close()

	res, err := exec.ExecuteRequest(ctx, 0)
	if err != nil {
		t.Fatalf("unexpected execution error: %v", err)
	}
	if res.Outcome != core.OutcomeSuccess {
		t.Errorf("expected OutcomeSuccess for custom header auth, got %s (status %d)", res.Outcome, res.StatusCode)
	}
}

func TestExecutor_Auth_Basic(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	ctx := context.Background()
	p := helperBuildPlan(t, server.URL()+"/auth/basic", "GET", "", []int{200}, 0, 0)
	authn, _ := auth.NewAuthenticator(&config.AuthConfig{
		Type:     auth.AuthTypeBasic,
		Username: "admin_user",
		Password: "secret_pass_123",
	})
	p.Authenticator = authn
	p.KnownSecrets = []string{"secret_pass_123"}

	exec, _ := NewHTTPExecutor(p)
	defer exec.Close()

	res, err := exec.ExecuteRequest(ctx, 0)
	if err != nil {
		t.Fatalf("unexpected execution error: %v", err)
	}
	if res.Outcome != core.OutcomeSuccess {
		t.Errorf("expected OutcomeSuccess for basic auth, got %s (status %d)", res.Outcome, res.StatusCode)
	}
}

func TestExecutor_Auth_TokenPool(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	ctx := context.Background()
	tokens := []string{"tok-A", "tok-B", "tok-C"}
	p := helperBuildPlan(t, server.URL()+"/auth/token-pool", "GET", "", []int{200}, 0, 0)
	authn, _ := auth.NewAuthenticator(&config.AuthConfig{
		Type:      auth.AuthTypeTokenPool,
		TokenPool: tokens,
	})
	p.Authenticator = authn
	p.KnownSecrets = tokens

	exec, _ := NewHTTPExecutor(p)
	defer exec.Close()

	// Worker 0 -> tok-A, Worker 1 -> tok-B, Worker 2 -> tok-C, Worker 3 -> tok-A
	for workerID := 0; workerID < 4; workerID++ {
		res, err := exec.ExecuteRequest(ctx, workerID)
		if err != nil {
			t.Fatalf("worker %d request failed: %v", workerID, err)
		}
		if res.Outcome != core.OutcomeSuccess {
			t.Errorf("worker %d failed with outcome %s", workerID, res.Outcome)
		}
	}

	recorded := server.RecordedRequests()
	if len(recorded) != 4 {
		t.Fatalf("expected 4 recorded requests, got %d", len(recorded))
	}
	if recorded[0].Header.Get("Authorization") != "Bearer tok-A" {
		t.Errorf("worker 0 token = %q, want 'Bearer tok-A'", recorded[0].Header.Get("Authorization"))
	}
	if recorded[1].Header.Get("Authorization") != "Bearer tok-B" {
		t.Errorf("worker 1 token = %q, want 'Bearer tok-B'", recorded[1].Header.Get("Authorization"))
	}
	if recorded[2].Header.Get("Authorization") != "Bearer tok-C" {
		t.Errorf("worker 2 token = %q, want 'Bearer tok-C'", recorded[2].Header.Get("Authorization"))
	}
	if recorded[3].Header.Get("Authorization") != "Bearer tok-A" {
		t.Errorf("worker 3 token = %q, want 'Bearer tok-A'", recorded[3].Header.Get("Authorization"))
	}
}

func TestExecutor_Auth_CookieJar_Isolation(t *testing.T) {
	server := testtarget.NewServer()
	defer server.Close()

	ctx := context.Background()
	p := helperBuildPlan(t, server.URL(), "GET", "", []int{200}, 0, 0)
	jarMgr, _ := auth.NewVUJarManager(true, 2)
	p.JarManager = jarMgr
	p.CookieJarEnabled = true

	exec, _ := NewHTTPExecutor(p)
	defer exec.Close()

	// VU 0 sets cookie session=VU0
	p.TargetURL, _ = url.Parse(server.URL() + "/cookies/set?session=VU0")
	_, err := exec.ExecuteRequest(ctx, 0)
	if err != nil {
		t.Fatalf("VU 0 set cookie failed: %v", err)
	}

	// VU 1 sets cookie session=VU1
	p.TargetURL, _ = url.Parse(server.URL() + "/cookies/set?session=VU1")
	_, err = exec.ExecuteRequest(ctx, 1)
	if err != nil {
		t.Fatalf("VU 1 set cookie failed: %v", err)
	}

	// VU 0 inspects cookies -> must have session=VU0
	p.TargetURL, _ = url.Parse(server.URL() + "/cookies/inspect")
	_, err = exec.ExecuteRequest(ctx, 0)
	if err != nil {
		t.Fatalf("VU 0 inspect cookie failed: %v", err)
	}

	// VU 1 inspects cookies -> must have session=VU1
	_, err = exec.ExecuteRequest(ctx, 1)
	if err != nil {
		t.Fatalf("VU 1 inspect cookie failed: %v", err)
	}

	recorded := server.RecordedRequests()
	// Recorded: 0: set VU0, 1: set VU1, 2: inspect VU0, 3: inspect VU1
	if len(recorded) != 4 {
		t.Fatalf("expected 4 recorded requests, got %d", len(recorded))
	}

	req2Cookie := recorded[2].Header.Get("Cookie")
	if !strings.Contains(req2Cookie, "session=VU0") || strings.Contains(req2Cookie, "session=VU1") {
		t.Errorf("VU 0 cookie isolation failed: got header %q", req2Cookie)
	}

	req3Cookie := recorded[3].Header.Get("Cookie")
	if !strings.Contains(req3Cookie, "session=VU1") || strings.Contains(req3Cookie, "session=VU0") {
		t.Errorf("VU 1 cookie isolation failed: got header %q", req3Cookie)
	}
}

func TestExecutor_Auth_ErrorScrubbing(t *testing.T) {
	// Connect to non-existent port with known secret
	ctx := context.Background()
	secret := "SECRET_TOKEN_EMBEDDED_999"
	p := helperBuildPlan(t, "http://127.0.0.1:59999/unreachable?token="+secret, "GET", "", []int{200}, 0, 100*time.Millisecond)
	p.KnownSecrets = []string{secret}

	exec, _ := NewHTTPExecutor(p)
	defer exec.Close()

	res, _ := exec.ExecuteRequest(ctx, 0)
	if res == nil || res.Err == nil {
		t.Fatalf("expected failed result with error")
	}

	errMsg := res.Err.Error()
	if strings.Contains(errMsg, secret) {
		t.Errorf("CRITICAL SECURITY VIOLATION: raw secret %q leaked in executor error: %q", secret, errMsg)
	}
}
