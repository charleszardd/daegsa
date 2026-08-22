package testtarget

import (
	"fmt"
	"math"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/charleszardd/daegsa/internal/clock"
)

// RateLimitHeaderStyle defines the format of rate limit headers returned by the server (§14).
type RateLimitHeaderStyle int

const (
	// RateLimitHeaderStyleSeconds emits Retry-After in integer delta-seconds (e.g. "Retry-After: 15").
	RateLimitHeaderStyleSeconds RateLimitHeaderStyle = iota

	// RateLimitHeaderStyleDate emits Retry-After in HTTP-Date format (e.g. "Retry-After: Sat, 22 Aug 2026 15:30:00 GMT").
	RateLimitHeaderStyleDate

	// RateLimitHeaderStyleDraft emits standard IETF Draft headers (RateLimit-Limit, RateLimit-Remaining, RateLimit-Reset, RateLimit-Policy).
	RateLimitHeaderStyleDraft

	// RateLimitHeaderStyleLegacy emits X-RateLimit-* headers (X-RateLimit-Limit, X-RateLimit-Remaining, X-RateLimit-Reset).
	RateLimitHeaderStyleLegacy

	// RateLimitHeaderStyleAll emits all rate-limiting header variants simultaneously.
	RateLimitHeaderStyleAll
)

// RateLimiterConfig defines the parameters for the rate limiter.
type RateLimiterConfig struct {
	RequestsPerWindow int
	Window            time.Duration
	HeaderStyle       RateLimitHeaderStyle
	Clock             clock.Clock
}

// RateLimiter simulates server-side rate-limiting and generates standard headers (§14).
type RateLimiter struct {
	mu          sync.Mutex
	limit       int
	window      time.Duration
	headerStyle RateLimitHeaderStyle
	clock       clock.Clock
	count       int
	resetAt     time.Time
}

// NewRateLimiter creates a new RateLimiter instance.
func NewRateLimiter(cfg RateLimiterConfig) *RateLimiter {
	c := cfg.Clock
	if c == nil {
		c = clock.NewRealClock()
	}
	return &RateLimiter{
		limit:       cfg.RequestsPerWindow,
		window:      cfg.Window,
		headerStyle: cfg.HeaderStyle,
		clock:       c,
		resetAt:     c.Now().Add(cfg.Window),
	}
}

// Check evaluates an incoming request. Returns allowed (true) or rate-limited (false)
// along with the rate-limiting response headers that should be attached to the response.
func (rl *RateLimiter) Check() (allowed bool, headers http.Header) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := rl.clock.Now()
	if now.After(rl.resetAt) || now.Equal(rl.resetAt) {
		rl.count = 0
		rl.resetAt = now.Add(rl.window)
	}

	remaining := rl.limit - rl.count
	if remaining < 0 {
		remaining = 0
	}

	headers = make(http.Header)
	resetSeconds := int(math.Ceil(rl.resetAt.Sub(now).Seconds()))
	if resetSeconds < 1 {
		resetSeconds = 1
	}

	if rl.count >= rl.limit {
		// Rate limit exceeded: apply 429 headers
		rl.populateHeaders(headers, 0, resetSeconds, rl.resetAt)
		return false, headers
	}

	rl.count++
	remaining = rl.limit - rl.count
	rl.populateHeaders(headers, remaining, resetSeconds, rl.resetAt)
	return true, headers
}

func (rl *RateLimiter) populateHeaders(h http.Header, remaining, resetSeconds int, resetAt time.Time) {
	switch rl.headerStyle {
	case RateLimitHeaderStyleSeconds:
		if remaining == 0 {
			h.Set("Retry-After", strconv.Itoa(resetSeconds))
		}
	case RateLimitHeaderStyleDate:
		if remaining == 0 {
			h.Set("Retry-After", resetAt.UTC().Format(http.TimeFormat))
		}
	case RateLimitHeaderStyleDraft:
		h.Set("RateLimit-Limit", strconv.Itoa(rl.limit))
		h.Set("RateLimit-Remaining", strconv.Itoa(remaining))
		h.Set("RateLimit-Reset", strconv.Itoa(resetSeconds))
		h.Set("RateLimit-Policy", fmt.Sprintf("%d;w=%d", rl.limit, int(rl.window.Seconds())))
		if remaining == 0 {
			h.Set("Retry-After", strconv.Itoa(resetSeconds))
		}
	case RateLimitHeaderStyleLegacy:
		h.Set("X-RateLimit-Limit", strconv.Itoa(rl.limit))
		h.Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		h.Set("X-RateLimit-Reset", strconv.FormatInt(resetAt.Unix(), 10))
		if remaining == 0 {
			h.Set("Retry-After", strconv.Itoa(resetSeconds))
		}
	case RateLimitHeaderStyleAll:
		h.Set("Retry-After", strconv.Itoa(resetSeconds))
		h.Set("RateLimit-Limit", strconv.Itoa(rl.limit))
		h.Set("RateLimit-Remaining", strconv.Itoa(remaining))
		h.Set("RateLimit-Reset", strconv.Itoa(resetSeconds))
		h.Set("RateLimit-Policy", fmt.Sprintf("%d;w=%d", rl.limit, int(rl.window.Seconds())))
		h.Set("X-RateLimit-Limit", strconv.Itoa(rl.limit))
		h.Set("X-RateLimit-Remaining", strconv.Itoa(remaining))
		h.Set("X-RateLimit-Reset", strconv.FormatInt(resetAt.Unix(), 10))
	}
}
