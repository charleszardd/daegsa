package executor

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RateLimitInfo captures parsed rate-limiting and throttling metadata from HTTP response headers (§9, §14).
type RateLimitInfo struct {
	RetryAfterSeconds *int64     `json:"retry_after_seconds,omitempty"`
	RetryAfterDate    *time.Time `json:"retry_after_date,omitempty"`
	Limit             *int64     `json:"limit,omitempty"`
	Remaining         *int64     `json:"remaining,omitempty"`
	ResetSeconds      *int64     `json:"reset_seconds,omitempty"`
	ResetDate         *time.Time `json:"reset_date,omitempty"`
	Policy            string     `json:"policy,omitempty"`
}

// ExtractRateLimitInfo inspects HTTP response headers and extracts standardized rate-limit metadata (§9, §14).
func ExtractRateLimitInfo(headers http.Header) *RateLimitInfo {
	if headers == nil {
		return nil
	}

	var info RateLimitInfo
	hasAny := false

	// 1. Retry-After header
	if retryAfterStr := strings.TrimSpace(headers.Get("Retry-After")); retryAfterStr != "" {
		if secs, err := strconv.ParseInt(retryAfterStr, 10, 64); err == nil {
			info.RetryAfterSeconds = &secs
			hasAny = true
		} else if parsedTime, err := http.ParseTime(retryAfterStr); err == nil {
			info.RetryAfterDate = &parsedTime
			hasAny = true
		}
	}

	// 2. Limit header (RateLimit-Limit or X-RateLimit-Limit)
	limitStr := strings.TrimSpace(headers.Get("RateLimit-Limit"))
	if limitStr == "" {
		limitStr = strings.TrimSpace(headers.Get("X-RateLimit-Limit"))
	}
	if limitStr != "" {
		if val, err := parseRateLimitNumeric(limitStr); err == nil {
			info.Limit = &val
			hasAny = true
		}
	}

	// 3. Remaining header (RateLimit-Remaining or X-RateLimit-Remaining)
	remStr := strings.TrimSpace(headers.Get("RateLimit-Remaining"))
	if remStr == "" {
		remStr = strings.TrimSpace(headers.Get("X-RateLimit-Remaining"))
	}
	if remStr != "" {
		if val, err := parseRateLimitNumeric(remStr); err == nil {
			info.Remaining = &val
			hasAny = true
		}
	}

	// 4. Reset header (RateLimit-Reset or X-RateLimit-Reset)
	resetStr := strings.TrimSpace(headers.Get("RateLimit-Reset"))
	if resetStr == "" {
		resetStr = strings.TrimSpace(headers.Get("X-RateLimit-Reset"))
	}
	if resetStr != "" {
		if val, err := parseRateLimitNumeric(resetStr); err == nil {
			// If unix epoch timestamp (e.g. > 1,000,000,000)
			if val > 1000000000 {
				t := time.Unix(val, 0).UTC()
				info.ResetDate = &t
			} else {
				info.ResetSeconds = &val
			}
			hasAny = true
		} else if parsedTime, err := http.ParseTime(resetStr); err == nil {
			info.ResetDate = &parsedTime
			hasAny = true
		}
	}

	// 5. Policy header (RateLimit-Policy)
	if policy := strings.TrimSpace(headers.Get("RateLimit-Policy")); policy != "" {
		info.Policy = policy
		hasAny = true
	}

	if !hasAny {
		return nil
	}

	return &info
}

func parseRateLimitNumeric(s string) (int64, error) {
	// If header contains multiple parameters like "100, 100;w=60", extract first token
	if idx := strings.IndexAny(s, ",;"); idx != -1 {
		s = strings.TrimSpace(s[:idx])
	}
	// Try float first to handle formatted rates like "100.0"
	if f, err := strconv.ParseFloat(s, 64); err == nil {
		return int64(f), nil
	}
	return strconv.ParseInt(s, 10, 64)
}
