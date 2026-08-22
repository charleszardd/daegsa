package testtarget_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/charleszardd/daegsa/internal/clock"
	"github.com/charleszardd/daegsa/internal/testtarget"
)

func TestTargetServer_StatusCodes(t *testing.T) {
	ts := testtarget.NewServer()
	defer ts.Close()

	codes := []int{200, 204, 400, 404, 500, 503, 429}
	for _, code := range codes {
		// Test via query param
		resp, err := http.Get(ts.URL() + "/test?status=" + strconv.Itoa(code))
		if err != nil {
			t.Fatalf("GET /test?status=%d failed: %v", code, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != code {
			t.Errorf("GET /test?status=%d got status %d", code, resp.StatusCode)
		}

		// Test via header
		req, _ := http.NewRequest("GET", ts.URL()+"/test", nil)
		req.Header.Set("X-Target-Status", strconv.Itoa(code))
		resp, err = http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("GET /test with X-Target-Status=%d failed: %v", code, err)
		}
		_ = resp.Body.Close()
		if resp.StatusCode != code {
			t.Errorf("X-Target-Status=%d got status %d", code, resp.StatusCode)
		}
	}
}

func TestTargetServer_Delays(t *testing.T) {
	ts := testtarget.NewServer()
	defer ts.Close()

	start := time.Now()
	resp, err := http.Get(ts.URL() + "/test?delay=50ms")
	if err != nil {
		t.Fatalf("GET /test?delay=50ms failed: %v", err)
	}
	_ = resp.Body.Close()

	elapsed := time.Since(start)
	if elapsed < 40*time.Millisecond {
		t.Errorf("expected delay of at least 40ms, got %v", elapsed)
	}
}

func TestTargetServer_PayloadSizes(t *testing.T) {
	ts := testtarget.NewServer()
	defer ts.Close()

	sizes := []int64{0, 1024, 65536, 1048576}
	for _, size := range sizes {
		resp, err := http.Get(ts.URL() + "/test?bytes=" + strconv.FormatInt(size, 10))
		if err != nil {
			t.Fatalf("GET /test?bytes=%d failed: %v", size, err)
		}
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if err != nil {
			t.Fatalf("reading body of size %d failed: %v", size, err)
		}
		if int64(len(body)) != size {
			t.Errorf("expected body length %d, got %d", size, len(body))
		}
	}
}

func TestTargetServer_Redirects(t *testing.T) {
	ts := testtarget.NewServer()
	defer ts.Close()

	// Client without auto-redirects
	noRedirectClient := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}

	// Same-origin redirect
	resp, err := noRedirectClient.Get(ts.URL() + "/start?redirect_path=/target")
	if err != nil {
		t.Fatalf("GET /start?redirect_path=/target failed: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Errorf("redirect status = %d, want 302", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "/target" {
		t.Errorf("Location header = %q, want '/target'", loc)
	}

	// Cross-origin redirect
	resp, err = noRedirectClient.Get(ts.URL() + "/start?redirect_url=http://external.example.com/dest")
	if err != nil {
		t.Fatalf("GET /start?redirect_url failed: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusFound {
		t.Errorf("redirect status = %d, want 302", resp.StatusCode)
	}
	if loc := resp.Header.Get("Location"); loc != "http://external.example.com/dest" {
		t.Errorf("Location header = %q, want 'http://external.example.com/dest'", loc)
	}
}

func TestTargetServer_AbruptDrops(t *testing.T) {
	ts := testtarget.NewServer()
	defer ts.Close()

	// Immediate drop
	resp, err := http.Get(ts.URL() + "/test?drop=immediate")
	if err == nil {
		// If read succeeds, reading the body must fail
		_, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr == nil {
			t.Errorf("expected error on immediate drop, got nil")
		}
	}

	// Midway drop
	resp, err = http.Get(ts.URL() + "/test?drop=midway&after_bytes=50")
	if err == nil {
		buf := make([]byte, 1000)
		n, readErr := resp.Body.Read(buf)
		_ = resp.Body.Close()
		if readErr == nil && n >= 1000 {
			t.Errorf("expected EOF or abrupt disconnect on midway drop, read %d bytes", n)
		}
	}
}

func TestTargetServer_TimeoutHang(t *testing.T) {
	ts := testtarget.NewServer()
	defer ts.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	req, _ := http.NewRequestWithContext(ctx, "GET", ts.URL()+"/test?hang=true", nil)
	_, err := http.DefaultClient.Do(req)
	if err == nil {
		t.Fatalf("expected timeout error on hang request, got nil")
	}
}

func TestTargetServer_Cookies(t *testing.T) {
	ts := testtarget.NewServer()
	defer ts.Close()

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}

	// Set cookies
	setURL := ts.URL() + "/cookies/set?session_id=secret123&theme=dark"
	resp, err := client.Get(setURL)
	if err != nil {
		t.Fatalf("GET /cookies/set failed: %v", err)
	}
	_ = resp.Body.Close()

	// Inspect cookies
	inspectURL := ts.URL() + "/cookies/inspect"
	resp, err = client.Get(inspectURL)
	if err != nil {
		t.Fatalf("GET /cookies/inspect failed: %v", err)
	}
	defer resp.Body.Close()

	var cookies map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&cookies); err != nil {
		t.Fatalf("failed to decode cookies JSON: %v", err)
	}

	if cookies["session_id"] != "secret123" {
		t.Errorf("session_id cookie = %q, want 'secret123'", cookies["session_id"])
	}
	if cookies["theme"] != "dark" {
		t.Errorf("theme cookie = %q, want 'dark'", cookies["theme"])
	}
}

func TestTargetServer_RateLimiting(t *testing.T) {
	mockClock := clock.NewControllableClock(time.Date(2026, 8, 22, 12, 0, 0, 0, time.UTC))
	rl := testtarget.NewRateLimiter(testtarget.RateLimiterConfig{
		RequestsPerWindow: 2,
		Window:            10 * time.Second,
		HeaderStyle:       testtarget.RateLimitHeaderStyleAll,
		Clock:             mockClock,
	})

	ts := testtarget.NewServer(
		testtarget.WithRateLimiter(rl),
		testtarget.WithClock(mockClock),
	)
	defer ts.Close()

	// 1st request: allowed
	resp, err := http.Get(ts.URL() + "/api/resource")
	if err != nil {
		t.Fatalf("1st request failed: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("1st request status = %d, want 200", resp.StatusCode)
	}

	// 2nd request: allowed
	resp, err = http.Get(ts.URL() + "/api/resource")
	if err != nil {
		t.Fatalf("2nd request failed: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("2nd request status = %d, want 200", resp.StatusCode)
	}

	// 3rd request: rate-limited (429)
	resp, err = http.Get(ts.URL() + "/api/resource")
	if err != nil {
		t.Fatalf("3rd request failed: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("3rd request status = %d, want 429", resp.StatusCode)
	}

	// Check rate limit headers
	if retryAfter := resp.Header.Get("Retry-After"); retryAfter == "" {
		t.Errorf("missing Retry-After header on 429 response")
	}
	if limit := resp.Header.Get("RateLimit-Limit"); limit != "2" {
		t.Errorf("RateLimit-Limit = %q, want '2'", limit)
	}
	if remaining := resp.Header.Get("RateLimit-Remaining"); remaining != "0" {
		t.Errorf("RateLimit-Remaining = %q, want '0'", remaining)
	}
	if xLimit := resp.Header.Get("X-RateLimit-Limit"); xLimit != "2" {
		t.Errorf("X-RateLimit-Limit = %q, want '2'", xLimit)
	}

	// Advance virtual time past the rate limit window
	mockClock.Advance(11 * time.Second)

	// 4th request: quota reset, allowed again
	resp, err = http.Get(ts.URL() + "/api/resource")
	if err != nil {
		t.Fatalf("4th request failed: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("4th request status after window reset = %d, want 200", resp.StatusCode)
	}

	// Verify request recording
	recorded := ts.RecordedRequests()
	if len(recorded) != 4 {
		t.Errorf("recorded requests count = %d, want 4", len(recorded))
	}
}

func TestTargetServer_RecordedRequests(t *testing.T) {
	ts := testtarget.NewServer()
	defer ts.Close()

	req, _ := http.NewRequest("POST", ts.URL()+"/items?foo=bar", nil)
	req.Header.Set("X-Custom-Header", "test-val")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("POST request failed: %v", err)
	}
	_ = resp.Body.Close()

	recorded := ts.RecordedRequests()
	if len(recorded) != 1 {
		t.Fatalf("recorded requests count = %d, want 1", len(recorded))
	}

	r := recorded[0]
	if r.Method != "POST" {
		t.Errorf("recorded Method = %q, want 'POST'", r.Method)
	}
	if r.Path != "/items" {
		t.Errorf("recorded Path = %q, want '/items'", r.Path)
	}
	if r.Header.Get("X-Custom-Header") != "test-val" {
		t.Errorf("recorded header = %q, want 'test-val'", r.Header.Get("X-Custom-Header"))
	}
}

func TestTargetServer_AuthEndpoints(t *testing.T) {
	ts := testtarget.NewServer()
	defer ts.Close()

	client := http.DefaultClient

	// 1. Bearer endpoint
	// Without token -> 401
	resp, err := client.Get(ts.URL() + "/auth/bearer")
	if err != nil {
		t.Fatalf("GET /auth/bearer failed: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 without bearer token, got %d", resp.StatusCode)
	}

	// With token -> 200
	reqBearer, _ := http.NewRequest("GET", ts.URL()+"/auth/bearer", nil)
	reqBearer.Header.Set("Authorization", "Bearer valid-token-123")
	resp, err = client.Do(reqBearer)
	if err != nil {
		t.Fatalf("GET /auth/bearer with valid token failed: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 with bearer token, got %d", resp.StatusCode)
	}

	// 2. Custom Header endpoint
	// Without header -> 401
	resp, err = client.Get(ts.URL() + "/auth/header")
	if err != nil {
		t.Fatalf("GET /auth/header failed: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 without custom header, got %d", resp.StatusCode)
	}

	// With header -> 200
	reqHeader, _ := http.NewRequest("GET", ts.URL()+"/auth/header", nil)
	reqHeader.Header.Set("X-API-Key", "apikey_999")
	resp, err = client.Do(reqHeader)
	if err != nil {
		t.Fatalf("GET /auth/header with header failed: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 with custom header, got %d", resp.StatusCode)
	}

	// 3. Basic auth endpoint
	// Without basic auth -> 401
	resp, err = client.Get(ts.URL() + "/auth/basic")
	if err != nil {
		t.Fatalf("GET /auth/basic failed: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 without basic auth, got %d", resp.StatusCode)
	}

	// With basic auth -> 200
	reqBasic, _ := http.NewRequest("GET", ts.URL()+"/auth/basic", nil)
	reqBasic.SetBasicAuth("admin", "pass")
	resp, err = client.Do(reqBasic)
	if err != nil {
		t.Fatalf("GET /auth/basic with credentials failed: %v", err)
	}
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 with basic auth, got %d", resp.StatusCode)
	}

	// 4. Token pool endpoint
	reqPool, _ := http.NewRequest("GET", ts.URL()+"/auth/token-pool", nil)
	reqPool.Header.Set("Authorization", "Bearer pool-tok-1")
	resp, err = client.Do(reqPool)
	if err != nil {
		t.Fatalf("GET /auth/token-pool failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 for token pool endpoint, got %d", resp.StatusCode)
	}
	if strings.Contains(string(body), "pool-tok-1") {
		t.Errorf("token-pool response leaked the raw token: %s", string(body))
	}
	if !strings.Contains(string(body), "token_hash") {
		t.Errorf("expected response to contain a non-secret token_hash, got %s", string(body))
	}
}

func TestTargetServer_ScenarioEndpoints(t *testing.T) {
	ts := testtarget.NewServer()
	defer ts.Close()

	jar, _ := cookiejar.New(nil)
	client := &http.Client{Jar: jar}

	// 1. POST /auth/login
	loginResp, err := client.Post(ts.URL()+"/auth/login", "application/json", strings.NewReader(`{"user":"test"}`))
	if err != nil {
		t.Fatalf("POST /auth/login failed: %v", err)
	}
	defer loginResp.Body.Close()

	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from /auth/login, got %d", loginResp.StatusCode)
	}

	var loginData map[string]interface{}
	if err := json.NewDecoder(loginResp.Body).Decode(&loginData); err != nil {
		t.Fatalf("failed to decode login JSON: %v", err)
	}
	if loginData["token"] != "tok_jwt_scenario_abc123" {
		t.Errorf("expected token 'tok_jwt_scenario_abc123', got %v", loginData["token"])
	}

	// 2. GET /api/items with session cookie in jar
	itemsResp, err := client.Get(ts.URL() + "/api/items")
	if err != nil {
		t.Fatalf("GET /api/items failed: %v", err)
	}
	defer itemsResp.Body.Close()

	if itemsResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from /api/items, got %d", itemsResp.StatusCode)
	}

	// 3. POST /api/logout
	logoutResp, err := client.Post(ts.URL()+"/api/logout", "application/json", nil)
	if err != nil {
		t.Fatalf("POST /api/logout failed: %v", err)
	}
	defer logoutResp.Body.Close()
	if logoutResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 from /api/logout, got %d", logoutResp.StatusCode)
	}

	// 4. GET /scenario/fail-step
	failResp, err := client.Get(ts.URL() + "/scenario/fail-step")
	if err != nil {
		t.Fatalf("GET /scenario/fail-step failed: %v", err)
	}
	defer failResp.Body.Close()
	if failResp.StatusCode != http.StatusInternalServerError {
		t.Errorf("expected 500 from /scenario/fail-step, got %d", failResp.StatusCode)
	}

	// 5. GET /scenario/dynamic
	dynResp, err := client.Get(ts.URL() + "/scenario/dynamic?user_id=42&cat=books")
	if err != nil {
		t.Fatalf("GET /scenario/dynamic failed: %v", err)
	}
	defer dynResp.Body.Close()
	if dynResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 from /scenario/dynamic, got %d", dynResp.StatusCode)
	}
}
