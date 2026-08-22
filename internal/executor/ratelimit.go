package executor

import (
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode"
)

const (
	maxRateLimitHeaderValueLength = 128
	rateLimitEpochThreshold       = 1000000000
)

type HeaderParseObservation struct {
	Name    string
	Present bool
	Valid   bool
	Value   string
}

// RateLimitInfo captures parsed rate-limiting metadata and bounded parse evidence.
type RateLimitInfo struct {
	RetryAfterSeconds  *int64                   `json:"retry_after_seconds,omitempty"`
	RetryAfterDate     *time.Time               `json:"retry_after_date,omitempty"`
	Limit              *int64                   `json:"limit,omitempty"`
	Remaining          *int64                   `json:"remaining,omitempty"`
	ResetSeconds       *int64                   `json:"reset_seconds,omitempty"`
	ResetDate          *time.Time               `json:"reset_date,omitempty"`
	Policy             string                   `json:"policy,omitempty"`
	HeaderObservations []HeaderParseObservation `json:"-"`
}

// ExtractRateLimitInfo applies standard-over-legacy precedence and retains
// parse-validity evidence without retaining unbounded response data.
func ExtractRateLimitInfo(headers http.Header) *RateLimitInfo {
	if headers == nil {
		return nil
	}
	info := &RateLimitInfo{}

	retryAfter := strings.TrimSpace(headers.Get("Retry-After"))
	if retryAfter != "" {
		observation := HeaderParseObservation{Name: "retry_after", Present: true}
		if seconds, err := strconv.ParseInt(retryAfter, 10, 64); err == nil && seconds >= 0 {
			info.RetryAfterSeconds = &seconds
			observation.Valid = true
			observation.Value = strconv.FormatInt(seconds, 10)
		} else if parsedTime, err := http.ParseTime(retryAfter); err == nil {
			info.RetryAfterDate = &parsedTime
			observation.Valid = true
			observation.Value = parsedTime.UTC().Format(http.TimeFormat)
		}
		info.HeaderObservations = append(info.HeaderObservations, observation)
	}

	limitRaw, limitPresent := preferredHeader(headers, "RateLimit-Limit", "X-RateLimit-Limit")
	if limitPresent {
		info.Limit = parseObservedNumeric(info, "limit", limitRaw)
	}
	remainingRaw, remainingPresent := preferredHeader(headers, "RateLimit-Remaining", "X-RateLimit-Remaining")
	if remainingPresent {
		info.Remaining = parseObservedNumeric(info, "remaining", remainingRaw)
	}
	resetRaw, resetPresent := preferredHeader(headers, "RateLimit-Reset", "X-RateLimit-Reset")
	if resetPresent {
		observation := HeaderParseObservation{Name: "reset", Present: true}
		if value, err := parseRateLimitNumeric(resetRaw); err == nil && value >= 0 {
			observation.Valid = true
			if value > rateLimitEpochThreshold {
				parsedTime := time.Unix(value, 0).UTC()
				info.ResetDate = &parsedTime
				observation.Value = parsedTime.Format(http.TimeFormat)
			} else {
				info.ResetSeconds = &value
				observation.Value = strconv.FormatInt(value, 10)
			}
		} else if parsedTime, err := http.ParseTime(resetRaw); err == nil {
			info.ResetDate = &parsedTime
			observation.Valid = true
			observation.Value = parsedTime.UTC().Format(http.TimeFormat)
		}
		info.HeaderObservations = append(info.HeaderObservations, observation)
	}
	if policy := strings.TrimSpace(headers.Get("RateLimit-Policy")); policy != "" {
		info.Policy = sanitizeHeaderValue(policy)
		info.HeaderObservations = append(info.HeaderObservations, HeaderParseObservation{Name: "policy", Present: true, Valid: true, Value: info.Policy})
	}
	if len(info.HeaderObservations) == 0 {
		return nil
	}
	return info
}

func preferredHeader(headers http.Header, standardName, legacyName string) (string, bool) {
	if values, exists := headers[http.CanonicalHeaderKey(standardName)]; exists && len(values) > 0 {
		return strings.TrimSpace(values[0]), true
	}
	if values, exists := headers[http.CanonicalHeaderKey(legacyName)]; exists && len(values) > 0 {
		return strings.TrimSpace(values[0]), true
	}
	return "", false
}

func parseObservedNumeric(info *RateLimitInfo, name, raw string) *int64 {
	observation := HeaderParseObservation{Name: name, Present: true}
	value, err := parseRateLimitNumeric(raw)
	if err == nil && value >= 0 {
		observation.Valid = true
		observation.Value = strconv.FormatInt(value, 10)
	}
	info.HeaderObservations = append(info.HeaderObservations, observation)
	if !observation.Valid {
		return nil
	}
	return &value
}

func sanitizeHeaderValue(value string) string {
	value = strings.Map(func(character rune) rune {
		if unicode.IsControl(character) {
			return ' '
		}
		return character
	}, value)
	value = strings.Join(strings.Fields(value), " ")
	if len(value) > maxRateLimitHeaderValueLength {
		value = value[:maxRateLimitHeaderValueLength]
	}
	return value
}

func parseRateLimitNumeric(value string) (int64, error) {
	if index := strings.IndexAny(value, ",;"); index != -1 {
		value = strings.TrimSpace(value[:index])
	}
	if parsed, err := strconv.ParseFloat(value, 64); err == nil {
		return int64(parsed), nil
	}
	return strconv.ParseInt(value, 10, 64)
}
