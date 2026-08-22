package executor

import (
	"net/http"
	"testing"
)

func TestRateLimitStandardPrecedenceAndInvalidEvidence(t *testing.T) {
	headers := http.Header{}
	headers.Set("RateLimit-Limit", "invalid")
	headers.Set("X-RateLimit-Limit", "99")
	info := ExtractRateLimitInfo(headers)
	if info == nil || info.Limit != nil {
		t.Fatalf("standard invalid header must not fall back to legacy: %+v", info)
	}
	if len(info.HeaderObservations) != 1 || info.HeaderObservations[0].Valid {
		t.Fatalf("missing invalid parse evidence: %+v", info.HeaderObservations)
	}
}

func TestRateLimitPolicySanitizesControlCharactersAndBoundsLength(t *testing.T) {
	headers := http.Header{}
	headers.Set("RateLimit-Policy", "abc\r\n"+string(make([]byte, 200)))
	info := ExtractRateLimitInfo(headers)
	if info == nil || len(info.Policy) > maxRateLimitHeaderValueLength {
		t.Fatalf("unbounded policy: %q", info.Policy)
	}
	for _, character := range info.Policy {
		if character < 0x20 {
			t.Fatalf("control character retained: %q", info.Policy)
		}
	}
}

func TestRetryAfter_DeltaSecondsAndHTTPDate(t *testing.T) {
	// Delta seconds
	headers1 := http.Header{}
	headers1.Set("Retry-After", "120")
	info1 := ExtractRateLimitInfo(headers1)
	if info1 == nil || info1.RetryAfterSeconds == nil || *info1.RetryAfterSeconds != 120 {
		t.Fatalf("expected RetryAfterSeconds=120, got %+v", info1)
	}

	// HTTP Date
	headers2 := http.Header{}
	headers2.Set("Retry-After", "Wed, 21 Oct 2026 07:28:00 GMT")
	info2 := ExtractRateLimitInfo(headers2)
	if info2 == nil || info2.RetryAfterDate == nil {
		t.Fatalf("expected RetryAfterDate parsed, got %+v", info2)
	}
}

func TestRateLimitReset_EpochAndDeltaAndDate(t *testing.T) {
	// Reset delta seconds (< 1e9)
	headers1 := http.Header{}
	headers1.Set("RateLimit-Reset", "60")
	info1 := ExtractRateLimitInfo(headers1)
	if info1 == nil || info1.ResetSeconds == nil || *info1.ResetSeconds != 60 {
		t.Fatalf("expected ResetSeconds=60, got %+v", info1)
	}

	// Reset epoch seconds (> 1e9)
	headers2 := http.Header{}
	headers2.Set("RateLimit-Reset", "1750000000")
	info2 := ExtractRateLimitInfo(headers2)
	if info2 == nil || info2.ResetDate == nil {
		t.Fatalf("expected ResetDate parsed from epoch timestamp, got %+v", info2)
	}

	// Reset HTTP Date
	headers3 := http.Header{}
	headers3.Set("RateLimit-Reset", "Wed, 21 Oct 2026 07:28:00 GMT")
	info3 := ExtractRateLimitInfo(headers3)
	if info3 == nil || info3.ResetDate == nil {
		t.Fatalf("expected ResetDate parsed from HTTP Date, got %+v", info3)
	}
}

func TestRateLimitPrecedence_StandardWinsWhenValid(t *testing.T) {
	headers := http.Header{}
	headers.Set("RateLimit-Limit", "100")
	headers.Set("X-RateLimit-Limit", "50")
	headers.Set("RateLimit-Remaining", "25")
	headers.Set("X-RateLimit-Remaining", "10")
	info := ExtractRateLimitInfo(headers)
	if info == nil {
		t.Fatal("expected non-nil info")
	}
	if info.Limit == nil || *info.Limit != 100 {
		t.Errorf("Limit = %v, want 100", info.Limit)
	}
	if info.Remaining == nil || *info.Remaining != 25 {
		t.Errorf("Remaining = %v, want 25", info.Remaining)
	}
}

func TestExtractRateLimitInfo_NilAndEmpty(t *testing.T) {
	if ExtractRateLimitInfo(nil) != nil {
		t.Error("expected nil on nil headers")
	}
	if ExtractRateLimitInfo(http.Header{}) != nil {
		t.Error("expected nil on empty headers")
	}
}
