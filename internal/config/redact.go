package config

import (
	"net/http"
	"net/url"
	"strings"
)

// RedactedPlaceholder is the canonical masked value for sensitive credentials (§6, §11).
const RedactedPlaceholder = "[REDACTED]"

// Standard sensitive header keys (case-insensitive checks).
var sensitiveHeaderSubstrings = []string{
	"auth",
	"token",
	"secret",
	"cookie",
	"key",
	"password",
}

// Standard sensitive query parameter substrings.
var sensitiveQueryParamSubstrings = []string{
	"token",
	"secret",
	"key",
	"auth",
	"password",
	"signature",
	"apikey",
	"api_key",
	"access_token",
}

// IsSensitiveHeader reports whether a header key is considered sensitive.
func IsSensitiveHeader(headerName string) bool {
	lower := strings.ToLower(strings.TrimSpace(headerName))
	for _, sub := range sensitiveHeaderSubstrings {
		if strings.Contains(lower, sub) {
			return true
		}
	}
	return false
}

// IsSensitiveQueryParam reports whether a query parameter name is considered sensitive.
func IsSensitiveQueryParam(paramName string) bool {
	lower := strings.ToLower(strings.TrimSpace(paramName))
	for _, sub := range sensitiveQueryParamSubstrings {
		if strings.Contains(lower, sub) {
			return true
		}
	}
	return false
}

// RedactHeaders returns a copy of headers where sensitive header values are replaced by [REDACTED].
func RedactHeaders(headers map[string]string) map[string]string {
	if headers == nil {
		return nil
	}
	redacted := make(map[string]string, len(headers))
	for k, v := range headers {
		if IsSensitiveHeader(k) {
			redacted[k] = RedactedPlaceholder
		} else {
			redacted[k] = v
		}
	}
	return redacted
}

// RedactHTTPHeaders returns a cloned http.Header where sensitive header values are replaced by [REDACTED].
func RedactHTTPHeaders(headers http.Header) http.Header {
	if headers == nil {
		return nil
	}
	redacted := make(http.Header, len(headers))
	for k, vals := range headers {
		if IsSensitiveHeader(k) {
			redacted[k] = []string{RedactedPlaceholder}
		} else {
			copiedVals := make([]string, len(vals))
			copy(copiedVals, vals)
			redacted[k] = copiedVals
		}
	}
	return redacted
}

// RedactURL redacts userinfo credentials and sensitive query parameters from a raw URL string.
func RedactURL(rawURL string) string {
	if rawURL == "" {
		return ""
	}

	parsed, err := url.Parse(rawURL)
	if err != nil {
		return rawURL
	}

	// Redact UserInfo
	if parsed.User != nil {
		parsed.User = url.UserPassword(RedactedPlaceholder, RedactedPlaceholder)
	}

	// Redact Query Parameters
	query := parsed.Query()
	if len(query) > 0 {
		modified := false
		for paramKey := range query {
			if IsSensitiveQueryParam(paramKey) {
				query.Set(paramKey, RedactedPlaceholder)
				modified = true
			}
		}
		if modified {
			parsed.RawQuery = query.Encode()
		}
	}

	return parsed.String()
}
